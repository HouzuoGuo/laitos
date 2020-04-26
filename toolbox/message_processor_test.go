package toolbox

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/misc"
)

func TestMessageProcessor_StoreReport(t *testing.T) {
	proc := &MessageProcessor{MaxReportsPerHostName: 100}
	if err := proc.Initialise(); err != nil {
		t.Fatal(err)
	}

	// Retrieve non-existent reports
	if reports := proc.GetLatestReports(1000); len(reports) != 0 {
		t.Fatalf("%+v", reports)
	} else if reports := proc.GetLatestReportsFromSubject("non-existent", 1000); len(reports) != 0 {
		t.Fatalf("%+v", reports)
	}

	// Store one report and retrieve
	proc.StoreReport(SubjectReportRequest{
		SubjectIP:       "subject-ip1",
		SubjectHostName: "subject-host-NAME1", // all incoming subject host names are converted to lower case
		SubjectPlatform: "subject-platform",
	}, "ip", "daemon")

	if reports := proc.GetLatestReports(1000); len(reports) != 1 {
		t.Fatalf("%+v", reports)
	} else if reports[0].OriginalRequest.SubjectIP != "subject-ip1" {
		t.Fatalf("%+v", reports)
	}
	// Verify the time keeping aspect of the report as well
	if reports := proc.GetLatestReportsFromSubject("subject-host-name1", 1000); len(reports) != 1 {
		t.Fatalf("%+v", reports)
	} else if reports[0].OriginalRequest.SubjectIP != "subject-ip1" || time.Now().Unix()-reports[0].ServerTime.Unix() > 3 ||
		time.Now().Unix()-reports[0].OriginalRequest.ServerTime.Unix() > 3 {
		t.Fatalf("%+v", reports)
	}

	// Store a report for another subject and retrieve
	time.Sleep(1100 * time.Millisecond)
	proc.StoreReport(SubjectReportRequest{
		SubjectIP:       "subject-ip2",
		SubjectHostName: "subject-host-NAME2",
		SubjectPlatform: "subject-platform",
	}, "ip", "daemon")

	// Keep in mind that reports are retrieved from latest to oldest
	if reports := proc.GetLatestReports(1000); len(reports) != 2 {
		t.Fatalf("%+v", reports)
	} else if reports[0].OriginalRequest.SubjectIP != "subject-ip2" || reports[1].OriginalRequest.SubjectIP != "subject-ip1" {
		t.Fatalf("%+v", reports)
	}
	if reports := proc.GetLatestReportsFromSubject("subject-host-name2", 1000); len(reports) != 1 {
		t.Fatalf("%+v", reports)
	} else if reports[0].OriginalRequest.SubjectIP != "subject-ip2" {
		t.Fatalf("%+v", reports)
	}

	// Store a second report for the first subject and retrieve
	time.Sleep(1100 * time.Millisecond)
	proc.StoreReport(SubjectReportRequest{
		SubjectIP:       "subject-ip1",
		SubjectHostName: "subject-host-name1",
		SubjectPlatform: "new-subject-platform",
	}, "ip", "daemon")

	if reports := proc.GetLatestReports(1000); len(reports) != 3 {
		t.Fatalf("%+v", reports)
	} else if reports[0].OriginalRequest.SubjectIP != "subject-ip1" || reports[1].OriginalRequest.SubjectIP != "subject-ip2" || reports[2].OriginalRequest.SubjectIP != "subject-ip1" ||
		reports[0].OriginalRequest.SubjectPlatform != "new-subject-platform" || reports[2].OriginalRequest.SubjectPlatform != "subject-platform" {
		t.Fatalf("%+v", reports)
	}
	if reports := proc.GetLatestReportsFromSubject("subject-host-name1", 1000); len(reports) != 2 {
		t.Fatalf("%+v", reports)
	} else if reports[0].OriginalRequest.SubjectPlatform != "new-subject-platform" || reports[1].OriginalRequest.SubjectPlatform != "subject-platform" {
		t.Fatalf("%+v", reports)
	}
}

func TestMessageProcessor_EvictOldReports(t *testing.T) {
	proc := &MessageProcessor{MaxReportsPerHostName: 100}
	if err := proc.Initialise(); err != nil {
		t.Fatal(err)
	}
	// Evict older reports (10 of them) from memory
	for i := 0; i < proc.MaxReportsPerHostName+10; i++ {
		proc.StoreReport(SubjectReportRequest{
			SubjectIP:       strconv.Itoa(i),
			SubjectHostName: "subject-host-name1",
			SubjectPlatform: "new-subject-platform",
		}, "ip", "daemon")
	}

	if reports := proc.GetLatestReports(2 * proc.MaxReportsPerHostName); len(reports) != proc.MaxReportsPerHostName {
		t.Fatal(len(reports))
	} else if latestReport := reports[0]; latestReport.OriginalRequest.SubjectIP != strconv.Itoa(proc.MaxReportsPerHostName+10-1) {
		t.Fatalf("%+v", latestReport)
	}

	if reports := proc.GetLatestReports(2 * proc.MaxReportsPerHostName); len(reports) != proc.MaxReportsPerHostName {
		t.Fatal(len(reports))
	} else if latestReport := reports[0]; latestReport.OriginalRequest.SubjectIP != strconv.Itoa(proc.MaxReportsPerHostName+10-1) {
		t.Fatalf("%+v", latestReport)
	}
}

func TestMessageProcessor_EvictExpiredReports(t *testing.T) {
	proc := &MessageProcessor{MaxReportsPerHostName: 100}
	if err := proc.Initialise(); err != nil {
		t.Fatal(err)
	}
	// Store a report
	proc.StoreReport(SubjectReportRequest{
		SubjectIP:       "1",
		SubjectHostName: "subject-host-name1",
		SubjectPlatform: "new-subject-platform",
	}, "ip", "daemon")
	// Change the timestamp of the report to make it expire
	(*proc.SubjectReports["subject-host-name1"])[0].OriginalRequest.ServerTime = time.Now().Add(-(SubjectExpirySecond + 1) * time.Second)

	// Store thousands of reports for an active subject, which triggers clean up in the meanwhile.
	for i := 0; i < proc.MaxReportsPerHostName+10; i++ {
		proc.StoreReport(SubjectReportRequest{
			SubjectIP:       strconv.Itoa(i),
			SubjectHostName: "subject-host-name2",
			SubjectPlatform: "new-subject-platform",
		}, "ip", "daemon")
	}

	if reports := proc.GetLatestReportsFromSubject("subject-host-name1", 1000); len(reports) != 0 {
		t.Fatal(len(reports))
	}
	if len(proc.IncomingAppCommands) != 0 {
		t.Fatalf("%+v", proc.IncomingAppCommands)
	}
}

func TestMessageProcessor_PendingCommandRequest(t *testing.T) {
	proc := &MessageProcessor{CmdProcessor: GetTestCommandProcessor(), MaxReportsPerHostName: 100}
	if err := proc.Initialise(); err != nil {
		t.Fatal(err)
	}

	cmd := TestCommandProcessorPIN + ".s echo 123"
	proc.SetOutgoingCommand("subject-host-NAME1", "test cmd")
	resp := proc.StoreReport(SubjectReportRequest{
		SubjectHostName: "subject-host-name1",
		CommandRequest:  AppCommandRequest{Command: cmd},
	}, "ip", "daemon")
	if resp.CommandRequest.Command != "test cmd" ||
		resp.CommandResponse.Command != cmd || resp.CommandResponse.RunDurationSec != 0 || resp.CommandResponse.Result != "123" {
		t.Fatalf("%+v", resp)
	}

	if cmds := proc.GetAllOutgoingCommands(); len(cmds) != 1 || cmds["subject-host-name1"] != "test cmd" {
		t.Fatalf("%+v", cmds)
	}

	proc.SetOutgoingCommand("subject-host-NAME1", "test cmd2")
	resp = proc.StoreReport(SubjectReportRequest{
		SubjectHostName: "subject-host-name1",
		CommandRequest:  AppCommandRequest{Command: cmd},
	}, "ip", "daemon")
	if resp.CommandRequest.Command != "test cmd2" ||
		resp.CommandResponse.Command != cmd || resp.CommandResponse.RunDurationSec != 0 || resp.CommandResponse.Result != "123" {
		t.Fatalf("%+v", resp)
	}

	proc.SetOutgoingCommand("subject-host-NAME1", "")
	resp = proc.StoreReport(SubjectReportRequest{
		SubjectHostName: "subject-host-name1",
		CommandRequest:  AppCommandRequest{Command: cmd},
	}, "ip", "daemon")
	if resp.CommandRequest.Command != "" ||
		resp.CommandResponse.Command != cmd || resp.CommandResponse.RunDurationSec != 0 || resp.CommandResponse.Result != "123" {
		t.Fatalf("%+v", resp)
	}
}

func TestMessageProcessor_processCommandRequest_QuickCommand(t *testing.T) {
	proc := &MessageProcessor{CmdProcessor: GetTestCommandProcessor(), MaxReportsPerHostName: 100}
	if err := proc.Initialise(); err != nil {
		t.Fatal(err)
	}
	// Store a report that carries a command with incorrect password
	cmd := "BadPass .s echo 123"
	resp := proc.StoreReport(SubjectReportRequest{
		SubjectHostName: "subject-host-name1",
		CommandRequest:  AppCommandRequest{Command: cmd},
	}, "ip", "daemon")
	if resp.CommandResponse.Command != cmd || resp.CommandResponse.RunDurationSec != 0 ||
		resp.CommandResponse.Result != ErrPINAndShortcutNotFound.Error() {
		t.Fatalf("%+v", resp)
	}
	// Store a report that carries a quick command
	cmd = TestCommandProcessorPIN + ".s echo 123"
	resp = proc.StoreReport(SubjectReportRequest{
		SubjectHostName: "subject-host-name1",
		CommandRequest:  AppCommandRequest{Command: cmd},
	}, "ip", "daemon")
	if resp.CommandResponse.Command != cmd || resp.CommandResponse.RunDurationSec != 0 ||
		resp.CommandResponse.Result != "123" {
		t.Fatalf("%+v", resp)
	}
}

func TestMessageProcessor_processCommandRequest_RecursiveCommand(t *testing.T) {
	proc := &MessageProcessor{CmdProcessor: GetTestCommandProcessor(), MaxReportsPerHostName: 100}
	if err := proc.Initialise(); err != nil {
		t.Fatal(err)
	}
	// Store a report that carries a recursive command, which shall be met with an error response.
	cmd := TestCommandProcessorPIN + `.0m  {  "something JSON`
	resp := proc.StoreReport(SubjectReportRequest{
		SubjectHostName: "subject-host-name1",
		CommandRequest:  AppCommandRequest{Command: cmd},
	}, "ip", "daemon")
	if resp.CommandResponse.Command != cmd || resp.CommandResponse.RunDurationSec != 0 ||
		!strings.Contains(resp.CommandResponse.Result, "will not run a recursive") {
		t.Fatalf("%+v", resp)
	}
	if len(proc.IncomingAppCommands) != 0 {
		t.Fatalf("%+v", proc.IncomingAppCommands)
	}
}

func TestMessageProcessor_processCommandRequest_SlowCommand(t *testing.T) {
	proc := &MessageProcessor{CmdProcessor: GetTestCommandProcessor(), MaxReportsPerHostName: 100}
	if err := proc.Initialise(); err != nil {
		t.Fatal(err)
	}
	// The slow command test uses "touch" shell command that is not compatible with Windows
	misc.SkipIfWindows(t)
	// Store a report that carries a slow command
	fileName := "/tmp/laitos-storenforward-slow-command"
	_ = os.Remove(fileName)
	defer os.Remove(fileName)
	cmd := TestCommandProcessorPIN + ".s sleep 3; touch " + fileName + "; echo done"
	go func() {
		resp := proc.StoreReport(SubjectReportRequest{
			SubjectHostName: "subject-host-name1",
			CommandRequest:  AppCommandRequest{Command: cmd},
		}, "ip", "daemon")
		if resp.CommandResponse.Command != cmd || (resp.CommandResponse.RunDurationSec < 3 && resp.CommandResponse.RunDurationSec > 4) ||
			resp.CommandResponse.Result != "done" {
			log.Panicf("%+v", resp)
		}
	}()
	// Retrieve result of that outstanding command - duration -1 indicates the command execution is still ongoing
	time.Sleep(1 * time.Second)
	resp := proc.StoreReport(SubjectReportRequest{
		SubjectHostName: "subject-host-name1",
		CommandRequest:  AppCommandRequest{Command: cmd},
	}, "ip", "daemon")
	if resp.CommandResponse.Command != cmd || resp.CommandResponse.RunDurationSec != -1 ||
		resp.CommandResponse.Result != "" {
		t.Fatalf("%+v", resp)
	}
	if _, err := os.Stat(fileName); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	// Retrieve result of that outstanding command - without repeating the same command in the request
	time.Sleep(1 * time.Second)
	resp = proc.StoreReport(SubjectReportRequest{
		SubjectHostName: "subject-host-name1",
	}, "ip", "daemon")
	if resp.CommandResponse.Command != cmd || resp.CommandResponse.RunDurationSec != -1 ||
		resp.CommandResponse.Result != "" {
		t.Fatalf("%+v", resp)
	}
	if _, err := os.Stat(fileName); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	// Retrieve result of the completed command
	time.Sleep(2 * time.Second)
	resp = proc.StoreReport(SubjectReportRequest{
		SubjectHostName: "subject-host-name1",
	}, "ip", "daemon")
	if resp.CommandResponse.Command != cmd || (resp.CommandResponse.RunDurationSec < 3 && resp.CommandResponse.RunDurationSec > 4) ||
		resp.CommandResponse.Result != "done" {
		t.Fatalf("%+v", resp)
	}
	if _, err := os.Stat(fileName); err != nil {
		t.Fatal(err)
	}
	// Delete the created file and for the next 5 seconds ensure that the file is not created, i.e. command not repeated
	if err := os.Remove(fileName); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		time.Sleep(1 * time.Second)
		resp = proc.StoreReport(SubjectReportRequest{
			SubjectHostName: "subject-host-name1",
		}, "ip", "daemon")
		if resp.CommandResponse.Command != cmd || (resp.CommandResponse.RunDurationSec < 3 && resp.CommandResponse.RunDurationSec > 4) ||
			resp.CommandResponse.Result != "done" {
			t.Fatalf("%+v", resp)
		}
		if _, err := os.Stat(fileName); !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
}

func TestMessageProcessor_processCommandRequest_SlowAlternatingCommand(t *testing.T) {
	proc := &MessageProcessor{CmdProcessor: GetTestCommandProcessor(), MaxReportsPerHostName: 100}
	if err := proc.Initialise(); err != nil {
		t.Fatal(err)
	}
	// The slow command test uses "touch" shell command that is not compatible with Windows
	misc.SkipIfWindows(t)

	// The first slow command ultimately creates a new file
	fileName1 := "/tmp/laitos-storenforward-slow-alternating-command1"
	cmd1 := TestCommandProcessorPIN + ".s sleep 3; touch " + fileName1 + "; echo done1"
	_ = os.Remove(fileName1)
	defer os.Remove(fileName1)
	go func() {
		resp := proc.StoreReport(SubjectReportRequest{
			SubjectHostName: "subject-host-name1",
			CommandRequest:  AppCommandRequest{Command: cmd1},
		}, "ip", "daemon")
		if resp.CommandResponse.Command != cmd1 || (resp.CommandResponse.RunDurationSec < 3 && resp.CommandResponse.RunDurationSec > 4) ||
			resp.CommandResponse.Result != "done1" {
			log.Panicf("%+v", resp)
		}
	}()
	// Retrieve result of that outstanding command - duration -1 indicates the command execution is still ongoing
	time.Sleep(1 * time.Second)
	resp := proc.StoreReport(SubjectReportRequest{
		SubjectHostName: "subject-host-name1",
	}, "ip", "daemon")
	if resp.CommandResponse.Command != cmd1 || resp.CommandResponse.RunDurationSec != -1 ||
		resp.CommandResponse.Result != "" {
		t.Fatalf("%+v", resp)
	}
	if _, err := os.Stat(fileName1); !os.IsNotExist(err) {
		t.Fatal(err)
	}

	// The second slow command also creates a new file
	fileName2 := "/tmp/laitos-storenforward-slow-alternating-command1"
	cmd2 := TestCommandProcessorPIN + ".s sleep 3; touch " + fileName2 + "; echo done2"
	_ = os.Remove(fileName2)
	defer os.Remove(fileName2)
	go func() {
		resp := proc.StoreReport(SubjectReportRequest{
			SubjectHostName: "subject-host-name1",
			CommandRequest:  AppCommandRequest{Command: cmd2},
		}, "ip", "daemon")
		if resp.CommandResponse.Command != cmd2 || (resp.CommandResponse.RunDurationSec < 3 && resp.CommandResponse.RunDurationSec > 4) ||
			resp.CommandResponse.Result != "done2" {
			log.Panicf("%+v", resp)
		}
	}()

	// Retrieve result of the second outstanding command
	time.Sleep(1 * time.Second)
	resp = proc.StoreReport(SubjectReportRequest{
		SubjectHostName: "subject-host-name1",
		CommandRequest:  AppCommandRequest{Command: cmd2},
	}, "ip", "daemon")
	if resp.CommandResponse.Command != cmd2 || resp.CommandResponse.RunDurationSec != -1 ||
		resp.CommandResponse.Result != "" {
		t.Fatalf("%+v", resp)
	}
	if _, err := os.Stat(fileName2); !os.IsNotExist(err) {
		t.Fatal(err)
	}

	// Wait till both commands are completed and check that both files now exist
	time.Sleep(4 * time.Second)
	if _, err := os.Stat(fileName1); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fileName2); err != nil {
		t.Fatal(err)
	}

	// Retrieve the output from second command
	resp = proc.StoreReport(SubjectReportRequest{
		SubjectHostName: "subject-host-name1",
		CommandRequest:  AppCommandRequest{Command: cmd2},
	}, "ip", "daemon")
	if resp.CommandResponse.Command != cmd2 || (resp.CommandResponse.RunDurationSec < 3 && resp.CommandResponse.RunDurationSec > 4) ||
		resp.CommandResponse.Result != "done2" {
		t.Fatalf("%+v", resp)
	}
}

func TestMessageProcessor_App(t *testing.T) {
	// Initialise with a bad command processor results in an error
	proc := &MessageProcessor{CmdProcessor: GetInsaneCommandProcessor(), MaxReportsPerHostName: 100}
	if err := proc.Initialise(); err == nil || !strings.Contains(err.Error(), "bad configuration") {
		t.Fatal(err)
	}
	if !proc.IsConfigured() {
		t.Fatal("not configured")
	}

	// Initialise with a good command processor
	proc = &MessageProcessor{CmdProcessor: GetTestCommandProcessor(), MaxReportsPerHostName: 100}
	if err := proc.Initialise(); err != nil {
		t.Fatal(err)
	}
	if !proc.IsConfigured() {
		t.Fatal("not configured")
	}
	if err := proc.Initialise(); err != nil {
		t.Fatal(err)
	}
	if err := proc.SelfTest(); err != nil {
		t.Fatal(err)
	}

	// Construct the report and encode into JSON
	report := SubjectReportRequest{
		SubjectIP:       "subject-ip",
		SubjectHostName: "subject-host-name",
		SubjectPlatform: "subject-platform",
		CommandRequest: AppCommandRequest{
			Command: TestCommandProcessorPIN + ".s echo hi",
		},
		CommandResponse: AppCommandResponse{
			Command:        "resp command",
			Result:         "resp result",
			RunDurationSec: 1,
		},
	}

	// Send it to app for execution
	result := proc.Execute(Command{
		ClientID:   "subject-ip",
		DaemonName: "command-daemon-name",
		Content:    report.SerialiseCompact(),
	})
	if result.Error != nil || result.Output == "" {
		t.Fatalf("%+v", result)
	}
	t.Logf("Report size is %d, response size is %d", len(report.SerialiseCompact()), len(result.Output))
	// Decode execution result
	var resp SubjectReportResponse
	if err := json.Unmarshal([]byte(result.Output), &resp); err != nil {
		t.Fatal(err)
	}

	// Verify execution result
	if resp.CommandResponse.Command != report.CommandRequest.Command || resp.CommandResponse.Result != "hi" || resp.CommandResponse.RunDurationSec != 0 {
		t.Fatalf("%+v", resp)
	}
	// Verify stored report
	reports := proc.GetLatestReports(100)
	if len(reports) != 1 {
		t.Fatalf("%+v", reports)
	}
	report0 := reports[0]
	if time.Now().Unix()-report0.OriginalRequest.ServerTime.Unix() > 3 {
		t.Fatalf("%+v", report0)
	}
	if report0.DaemonName != "command-daemon-name" || report0.SubjectClientID != "subject-ip" ||
		report0.OriginalRequest.SubjectIP != report.SubjectIP ||
		report0.OriginalRequest.SubjectHostName != report.SubjectHostName ||
		report0.OriginalRequest.SubjectPlatform != report.SubjectPlatform ||
		report0.OriginalRequest.CommandRequest.Command != report.CommandRequest.Command ||
		report0.OriginalRequest.CommandResponse.Command != report.CommandResponse.Command ||
		report0.OriginalRequest.CommandResponse.Result != report.CommandResponse.Result ||
		report0.OriginalRequest.CommandResponse.RunDurationSec != report.CommandResponse.RunDurationSec {
		t.Fatalf("\n%+v\n%+v\n%+v\n", report0, report0.OriginalRequest, report)
	}
}
