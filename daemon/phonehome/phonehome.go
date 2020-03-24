package phonehome

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/testingstub"
	"github.com/HouzuoGuo/laitos/toolbox"
)

/*
MessageProcessorServer contains password configuration and memorises execution result of the most recent
app command that the server requested this subject to run.
*/
type MessageProcessorServer struct {
	// Password is the password PIN that the server accepts for command execution.
	Password string `json:"Password"`
	// HostName is the host name portion of server app command execution URL, it is calculated by Initialise function.
	HostName string `json:"-"`
}

/*
Daemon phones home periodically by contacting one or more store&forward message processor servers over
app command execution URLs.
*/
type Daemon struct {
	// MessageProcessorServers is a map between message processor server URL and their configuration.
	MessageProcessorServers map[string]*MessageProcessorServer `json:"MessageProcessorServers"`

	// ReportIntervalSec is the interval in seconds at which this daemon reports to the servers.
	ReportIntervalSec int `json:"ReportIntervalSec"`

	// LocalMessageProcessor answers to servers' app command requests
	LocalMessageProcessor *toolbox.MessageProcessor `json:"-"`
	// cmdProcessor runs app commands coming in from a store&forward message processor server.
	Processor *toolbox.CommandProcessor `json:"-"`

	runLoop bool
	logger  lalog.Logger
}

// Initialise validates the daemon configuration and initalises internal states.
func (daemon *Daemon) Initialise() error {
	if daemon.ReportIntervalSec < 1 {
		daemon.ReportIntervalSec = toolbox.ReportIntervalSec
	}
	if len(daemon.MessageProcessorServers) == 0 {
		return errors.New("phonehome.Initialise: MessageProcessorServers must have at least one entry")
	}
	/*
		Daemon's app command processor is not used directly, instead it is given to the local message processor
		to process app commands requested by remote server.
	*/
	if daemon.Processor != nil {
		if errs := daemon.Processor.IsSaneForInternet(); len(errs) > 0 {
			return fmt.Errorf("phonehome.Initialise: %+v", errs)
		}
	}
	// There is no point in keeping many app command exchange reports in memory
	daemon.LocalMessageProcessor = &toolbox.MessageProcessor{MaxReportsPerHostName: 10, CmdProcessor: daemon.Processor}
	if err := daemon.LocalMessageProcessor.Initialise(); err != nil {
		return fmt.Errorf("phonehome.Initialise: failed to initialise local message processor - %v", err)
	}
	// Calculate the host name portion of each URL, the host name is used by the local message processor.
	for cmdURL, srv := range daemon.MessageProcessorServers {
		if srv == nil || srv.Password == "" {
			return fmt.Errorf("phonehome.Initialise: missing password configuration for server URL %s", cmdURL)
		}
		u, err := url.Parse(cmdURL)
		if err != nil {
			return fmt.Errorf("phonehome.Initialise: malformed app command URL \"%s\"", cmdURL)
		}
		srv.HostName = u.Hostname()
	}
	daemon.logger = lalog.Logger{ComponentName: "phonehome"}
	return nil
}

func (daemon *Daemon) getTwoFACode(server *MessageProcessorServer) string {
	// The first 2FA is calculated from the command password
	_, cmdPassword1, _, err := toolbox.GetTwoFACodes(server.Password)
	if err != nil {
		daemon.logger.Warning("getTwoFACode", "", err, "failed to generate the first 2FA")
		return ""
	}
	// The second 2FA is calculated from the reversed command password
	reversedPass := []rune(server.Password)
	for i, j := 0, len(reversedPass)-1; i < j; i, j = i+1, j-1 {
		reversedPass[i], reversedPass[j] = reversedPass[j], reversedPass[i]
	}
	_, cmdPassword2, _, err := toolbox.GetTwoFACodes(string(reversedPass))
	if err != nil {
		daemon.logger.Warning("getTwoFACode", "", err, "failed to generate the second 2FA")
		return ""
	}
	return cmdPassword1 + cmdPassword2
}

func (daemon *Daemon) getReportForServer(cmdURL string) string {
	srv := daemon.MessageProcessorServers[cmdURL]
	// Ask local message processor for a pending app command request and/or app command response
	cmdExchange := daemon.LocalMessageProcessor.StoreReport(toolbox.SubjectReportRequest{SubjectHostName: srv.HostName}, cmdURL, "phonehome")
	// Craft the report for this server
	hostname, _ := os.Hostname()
	report := toolbox.SubjectReportRequest{
		SubjectIP:       inet.GetPublicIP(),
		SubjectHostName: hostname,
		SubjectPlatform: fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH),
		SubjectComment:  toolbox.GetRuntimeInfo(),
		CommandRequest:  cmdExchange.CommandRequest,
		CommandResponse: cmdExchange.CommandResponse,
	}
	reportJSON, err := json.Marshal(report)
	if err != nil {
		daemon.logger.Warning("getReportForServer", cmdURL, err, "failed to serialise report")
		return ""
	}
	return string(reportJSON)
}

// StartAndBlock starts the periodic reports and blocks caller until the daemon is stopped.
func (daemon *Daemon) StartAndBlock() error {
	defer func() {
		daemon.runLoop = false
	}()
	/*
		Instead of sending numerous reports in a row and then wait for a longer duration, send one report at a time and
		wait a shorter duration. This helps to reduce server load and overall offers more reliability.
		If there is a large number of servers to contact, the minimum interval will be one second.
	*/
	intervalSecBetweenReports := daemon.ReportIntervalSec / len(daemon.MessageProcessorServers)
	if intervalSecBetweenReports < 1 {
		intervalSecBetweenReports = 1
	}
	daemon.logger.Info("StartAndBlock", "", nil, "reporting to %d URLs and pausing %d seconds between each",
		len(daemon.MessageProcessorServers), intervalSecBetweenReports)
	daemon.runLoop = true
	for {
		if misc.EmergencyLockDown {
			daemon.logger.Warning("StartAndBlock", "", misc.ErrEmergencyLockDown, "")
			return misc.ErrEmergencyLockDown
		}
		/*
			Shuffle the destination URLs that reports are sent to.
			Reports are sent using 2FA authentication rather than the regular password authentication, if destinations
			are not contacted in a random order, there is a chance that the daemon may reach its own server first (this
			is a valid configuration) and it will always reject further reports as 2FA codes cannot be used a second time.
		*/
		allURLs := make([]string, 0, len(daemon.MessageProcessorServers))
		for cmdURL := range daemon.MessageProcessorServers {
			allURLs = append(allURLs, cmdURL)
		}
		rand.Shuffle(len(allURLs), func(i, j int) { allURLs[i], allURLs[j] = allURLs[j], allURLs[i] })

		for _, cmdURL := range allURLs {
			if !daemon.runLoop {
				return nil
			}
			time.Sleep(time.Duration(intervalSecBetweenReports) * time.Second)
			srv := daemon.MessageProcessorServers[cmdURL]
			// Send the latest report via HTTP client
			resp, err := inet.DoHTTP(inet.HTTPRequest{
				TimeoutSec: 15,
				MaxBytes:   16 * 1024,
				Method:     http.MethodPost,
				Body: strings.NewReader(url.Values{
					"cmd": {daemon.getTwoFACode(srv) + toolbox.StoreAndForwardMessageProcessorTrigger + daemon.getReportForServer(cmdURL)},
				}.Encode()),
			}, cmdURL)
			if err != nil {
				daemon.logger.Warning("StartAndBlock", cmdURL, err, "failed to send HTTP request")
				continue
			}
			var reportResponse toolbox.SubjectReportResponse
			if err := json.Unmarshal(resp.Body, &reportResponse); err != nil {
				daemon.logger.Info("StartAndBlock", cmdURL, nil, "failed to deserialise JSON report response - %s", resp.GetBodyUpTo(200))
				continue
			}
			// Deserialise the server response and pass it to local message processor to process the command request
			daemon.LocalMessageProcessor.StoreReport(toolbox.SubjectReportRequest{
				SubjectHostName: srv.HostName,
				ServerTime:      time.Time{},
				CommandRequest:  reportResponse.CommandRequest,
				CommandResponse: reportResponse.CommandResponse,
			}, cmdURL, "phonehome")

			if newCmd := reportResponse.CommandRequest.Command; newCmd != "" && daemon.Processor != nil {
			} else {
				daemon.logger.Info("StartAndBlock", cmdURL, nil, "report sent")
			}
		}
	}
}

// Stop the daemon.
func (daemon *Daemon) Stop() {
	daemon.runLoop = false
}

// TestServer implements test cases for the phone-home daemon.
func TestServer(server *Daemon, t testingstub.T) {
	// Start a web server that behaves like a message processor server
	mux := http.NewServeMux()
	muxNumRequests := 0
	muxMessageProcessor := toolbox.MessageProcessor{CmdProcessor: toolbox.GetTestCommandProcessor()}
	if err := muxMessageProcessor.Initialise(); err != nil {
		t.Fatal(err)
	}
	mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		// The handler is designed to handle exactly two requests
		if r.Method == http.MethodPost {
			if muxNumRequests == 0 {
				// The first request is expected to be a regular report
				reqCmd := r.FormValue("cmd")
				result := muxMessageProcessor.Execute(toolbox.Command{
					Content:    reqCmd[strings.Index(reqCmd, ".0m")+3:],
					TimeoutSec: 2,
					ClientID:   r.RemoteAddr,
					DaemonName: "httpd",
				})
				t.Log(reqCmd)
				if result.Error != nil {
					t.Fatalf("1st request error: %+v", result)
				}
				if len(muxMessageProcessor.OutstandingCommands) != 1 { // "local-to-server"
					t.Fatalf("1st request unexpected outstanding command: %+v", muxMessageProcessor.OutstandingCommands)
				}
				if len(muxMessageProcessor.SubjectReports) != 1 {
					t.Fatalf("1st request unexpected subject reports: %+v", muxMessageProcessor.SubjectReports)
				}
				for _, reports := range muxMessageProcessor.SubjectReports {
					report0 := (*reports)[0]
					if report0.SubjectClientID == "" || report0.DaemonName == "" || report0.OriginalRequest.SubjectHostName == "" ||
						report0.OriginalRequest.CommandRequest.Command != "local-to-server" || report0.OriginalRequest.CommandResponse.Command != "" {
						t.Fatalf("1st request, unexpected memorised report: %+v", report0)
					}
				}
				// The response will ask the daemon to run an app command
				resp := toolbox.SubjectReportResponse{
					CommandRequest: toolbox.AppCommandRequest{
						Command: toolbox.TestCommandProcessorPIN + ".s echo hi",
					},
				}
				respJSON, err := json.Marshal(resp)
				if err != nil {
					t.Fatal(err)
				}
				_, _ = w.Write(respJSON)
				muxNumRequests++
			} else if muxNumRequests == 1 {
				// The second request is a report that carries the app execution result from the app command.
				reqCmd := r.FormValue("cmd")
				result := muxMessageProcessor.Execute(toolbox.Command{
					Content:    reqCmd[strings.Index(reqCmd, ".0m")+3:],
					TimeoutSec: 2,
					ClientID:   r.RemoteAddr,
					DaemonName: "httpd",
				})
				t.Log(reqCmd)
				if result.Error != nil {
					t.Fatalf("2st request error: %+v", result)
				}
				if len(muxMessageProcessor.OutstandingCommands) != 1 { // "local-to-server"
					t.Fatalf("2st request unexpected outstanding command: %+v", muxMessageProcessor.OutstandingCommands)
				}
				if len(muxMessageProcessor.SubjectReports) != 1 {
					t.Fatalf("2st request unexpected subject reports: %+v", muxMessageProcessor.SubjectReports)
				}
				for _, reports := range muxMessageProcessor.SubjectReports {
					report1 := (*reports)[1]
					if report1.SubjectClientID == "" || report1.DaemonName == "" || report1.OriginalRequest.SubjectHostName == "" ||
						report1.OriginalRequest.CommandRequest.Command != "local-to-server" || report1.OriginalRequest.CommandResponse.Command == "" ||
						report1.OriginalRequest.CommandResponse.Result != "hi" || report1.OriginalRequest.CommandResponse.RunDurationSec > 2 {
						t.Fatalf("2nd request, unexpected memorised report: %+v", report1)
					}
				}

				// The response will ask the daemon to run an app command
				resp := toolbox.SubjectReportResponse{
					CommandRequest: toolbox.AppCommandRequest{
						Command: toolbox.TestCommandProcessorPIN + ".s echo hi",
					},
				}
				respJSON, err := json.Marshal(resp)
				if err != nil {
					t.Fatal(err)
				}
				_, _ = w.Write(respJSON)
				muxNumRequests++
			}
		}
	})
	l, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := http.Server{Addr: "0.0.0.0:0", Handler: mux}
	go func() {
		if err := srv.Serve(l); err != nil {
			t.Fatal(err)
		}
	}()
	// Start phone-home daemon
	cmdURL := fmt.Sprintf("http://localhost:%d/test", l.Addr().(*net.TCPAddr).Port)
	server.MessageProcessorServers = map[string]*MessageProcessorServer{
		cmdURL: &MessageProcessorServer{Password: toolbox.TestCommandProcessorPIN},
	}
	if err := server.Initialise(); err != nil {
		t.Fatal(err)
	}
	// Prepare an outstanding to be sent to the server by the local message processor
	server.LocalMessageProcessor.SetUpcomingSubjectCommand("localhost", "local-to-server")
	var stoppedNormally bool
	go func() {
		if err := server.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
		stoppedNormally = true
	}()
	// The daemon is expected to run at 1 second interval and the web server tests the request/response sequences
	time.Sleep(5 * time.Second)
	if muxNumRequests < 2 {
		t.Fatalf("did not hit test server - got %d requests", muxNumRequests)
	}
	// Check local message processor's number of reports
	if localReports := server.LocalMessageProcessor.GetLatestReports(1000); len(localReports) < 6 {
		t.Fatalf("%+v", localReports)
	}
	// Daemon should stop within a second
	server.Stop()
	time.Sleep(1 * time.Second)
	if !stoppedNormally {
		t.Fatal("did not stop")
	}
	// Repeatedly stopping the daemon should have no negative consequence
	server.Stop()
	server.Stop()
}
