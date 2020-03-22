package toolbox

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
)

const (
	/*
		ReportIntervalSec is the recommended interval at which subjects are to send their reports to a daemon.
		This is only a recommendation - subjects may freely exceed or relax the interval as they see fit.
	*/
	ReportIntervalSec = 60
	/*
		AppCommandResponseRetentionSec is the maximum duration to retain an app command execution result per host. During the retention period the result
		will be available for retrieval After the retention period the result will be available for retrieval one last time, and then removed from memory.
		The command execution timeout shares the same number.
	*/
	CommandResponseRetentionSec = ReportIntervalSec * 10
	/*
		MaxReportsPerHostName is the maximum number of reports to be kept in memory per each subject, identified by subject's self-reported
		host name, regardless of which server daemon received the report.
		The value tries to keep couple of days of reports in memory.
	*/
	MaxReportsPerHostName = 100 * 3600 / ReportIntervalSec
	// SubjectExpirySecond is the number of seconds after which if a subject is not heard from again it will be removed.
	SubjectExpirySecond = 72 * 3600
	/*
		StoreAndForwardMessageProcessorTrigger is the toolbox app command invocation prefix for the store&forward message processor.
		"mp" would have been more suitable, however "m" letter is already taken by send-mail app.
	*/
	StoreAndForwardMessageProcessorTrigger = ".0m"
)

// RegexNoRecursion matches an app command that invokes store&forward processor app itself. It helps to stop a recursion.
var RegexNoRecursion = regexp.MustCompile(`\` + StoreAndForwardMessageProcessorTrigger + `\s*{\s*"`)

/*
SubjectReport is a system status report arrived at the store&forward message processor, it contains the original report along with
additional subject information collected by the message processor.
*/
type SubjectReport struct {
	OriginalRequest SubjectReportRequest // OriginalRequest is the agent's report received by the message processor.
	SubjectClientIP string               // SubjectClientID is the subject's IP address as observed by server daemon.
	ServerTime      time.Time            // ServerClock is the system time of this computer upon receiving the report.
	DaemonName      string               // DaemonName is the name of server daemon that received this report.
}

/*
OutstandingCommand is a complete application command (and later on execution result) that a subject requested store&forward message processor to run.
A message processor keeps track of maximum of one outstanding command per host name, where the host name is self-reported by a subject.
*/
type OutstandingCommand struct {
	Request        SubjectReportRequest
	RunDurationSec int
	Result         Result
}

/*
MessageProcessor collects subject reports and relays outstanding app command requests and responses using the store&forward technique.
It also implements the usual toolbox app interface so that monitored subjects can reach it via app-compatible daemons to send their reports.
*/
type MessageProcessor struct {
	// SubjectReports is a map of subject's self-reported host name and its most recent reports, sorted from earliest to latest.
	SubjectReports map[string]*[]SubjectReport `json:"-"`
	// OutstandingCommands is a map of subject's self reported host name and an app command the subject would like the message processor to run.
	OutstandingCommands map[string]*OutstandingCommand `json:"-"`
	/*
		PendingSubjectCommand is a map of subject's self reported host name and an app command that the message processor would like the subject to run.
		This command is delivered as often as the reports in.
	*/
	UpcomingSubjectCommand map[string]string `json:"-"`
	// CmdProcessor processes app commands as requested by a remote server.
	CmdProcessor *CommandProcessor `json:"-"`

	// totalReports is the total number of reports received thus far.
	totalReports int
	// mutex prevents concurrent modifications made to internal structures.
	mutex  *sync.Mutex
	logger lalog.Logger
}

// SetUpcomingSubjectCommand stores an app command that the message processor carries in a reply to a subject report.
func (proc *MessageProcessor) SetUpcomingSubjectCommand(host, cmdContent string) {
	proc.mutex.Lock()
	defer proc.mutex.Unlock()
	proc.UpcomingSubjectCommand[host] = cmdContent
}

/*
StoreReports stores the most recent report from a subject and evicts older report automatically.
If the report carries an app command, then the command will run in the background.
*/
func (proc *MessageProcessor) StoreReport(request SubjectReportRequest, clientIP, daemonName string) SubjectReportResponse {
	proc.mutex.Lock()
	hostName := request.SubjectHostName
	reports := proc.SubjectReports[hostName]
	if reports == nil {
		// Reserve the maximum capacity as computer subjects often stay online for quite a while
		newReports := make([]SubjectReport, 0, MaxReportsPerHostName)
		reports = &newReports
	}
	// Note down server's time in the original request
	request.ServerTime = time.Now()
	newReport := SubjectReport{
		OriginalRequest: request,
		SubjectClientIP: clientIP,
		ServerTime:      request.ServerTime,
		DaemonName:      daemonName,
	}
	// Discard the oldest report
	if len(*reports) == MaxReportsPerHostName {
		*reports = (*reports)[1:]
	}
	// Append the latest report
	*reports = append(*reports, newReport)
	proc.SubjectReports[hostName] = reports
	// Remove expired subjects after couple of thousands of reports
	proc.totalReports++
	if proc.totalReports%MaxReportsPerHostName == 0 {
		proc.removeExpiredSubjects()
	}
	upcomingSubjectCommand := proc.UpcomingSubjectCommand[request.SubjectHostName]
	pendingCommandRequest := AppCommandRequest{
		Command: upcomingSubjectCommand,
	}
	// Release the lock for report handling is now completed. The app command (if requested) will run without holding the lock.
	proc.mutex.Unlock()
	cmdResponse := proc.processCommandRequest(request, clientIP, daemonName)
	if upcomingSubjectCommand == "" {
		proc.logger.Info("StoreReport", fmt.Sprintf("%s-%s", request.SubjectHostName, clientIP), nil, "received report via daemon %s", daemonName)
	} else {
		proc.logger.Info("StoreReport", fmt.Sprintf("%s-%s", request.SubjectHostName, clientIP), nil, "received report via daemon %s, replying with its pending app command.", daemonName)
	}
	return SubjectReportResponse{
		CommandRequest:  pendingCommandRequest,
		CommandResponse: cmdResponse,
	}
}

/*
processCommandRequest runs the app command presented in the request, waits for it to complete and returns the result.
If the same app command or an empty command request comes in, the previous result (if ready and available) will be returned.
*/
func (proc *MessageProcessor) processCommandRequest(request SubjectReportRequest, clientIP, daemonName string) (resp AppCommandResponse) {
	if proc.CmdProcessor == nil {
		return
	}
	// Keep in mind that the application command contains the access password, do not write it into a log entry.
	appCmd := request.CommandRequest.Command

	proc.mutex.Lock()
	prevCmd, exists := proc.OutstandingCommands[request.SubjectHostName]
	proc.mutex.Unlock()

	if appCmd == "" || exists && prevCmd.Request.CommandRequest.Command == appCmd {
		// The subject does not make a command request or has made the identical request. Retrieve previously requested command result if there is any.
		if exists {
			proc.logger.Info("processCommandRequest", fmt.Sprintf("%s-%s", request.SubjectHostName, clientIP), nil,
				"retrieve result from app command submitted at %s and completed in %d seconds", prevCmd.Request.ServerTime, prevCmd.RunDurationSec)
			if prevCmd.Request.ServerTime.Before(time.Now().Add(-CommandResponseRetentionSec * time.Second)) {
				// Erase the result from memory beyond the retention period
				proc.mutex.Lock()
				delete(proc.OutstandingCommands, request.SubjectHostName)
				proc.mutex.Unlock()
			}
			// Return the memorised result
			resp = AppCommandResponse{
				Command:        prevCmd.Request.CommandRequest.Command,
				ReceivedAt:     prevCmd.Request.ServerTime,
				Result:         prevCmd.Result.CombinedOutput,
				RunDurationSec: prevCmd.RunDurationSec,
			}
		}
		// No memorised result to retrieve, the function's return value remains empty.
	} else {
		// The subject made a request to run a brand new command
		if RegexNoRecursion.MatchString(appCmd) {
			// Prevent recursive store&forward
			resp = AppCommandResponse{
				Command:    appCmd,
				ReceivedAt: request.ServerTime,
				Result:     "error: will not run a recursive store&forward command",
			}
			proc.logger.Warning("processCommandRequest", fmt.Sprintf("%s-%s", request.SubjectHostName, clientIP), nil,
				"will not run a recursive store&forward command - %s", appCmd)
			return
		}
		cmd := Command{
			DaemonName: daemonName,
			ClientID:   clientIP,
			Content:    appCmd,
			TimeoutSec: CommandResponseRetentionSec,
		}
		proc.mutex.Lock()
		// Store the constructed command as an outstanding commands
		proc.OutstandingCommands[request.SubjectHostName] = &OutstandingCommand{
			Request:        request,
			RunDurationSec: -1, // duration of -1 indicates that result is not yet available
			Result:         Result{Command: cmd},
		}
		proc.mutex.Unlock()
		// Run the app command and then memorise the result
		startTimeSec := time.Now().Unix()
		result := proc.CmdProcessor.Process(cmd, true)
		durationSec := time.Now().Unix() - startTimeSec
		proc.mutex.Lock()
		proc.OutstandingCommands[request.SubjectHostName] = &OutstandingCommand{
			Request:        request,
			RunDurationSec: int(durationSec),
			Result:         *result,
		}
		proc.mutex.Unlock()
		// Return the result to caller
		resp = AppCommandResponse{
			Command:        appCmd,
			ReceivedAt:     request.ServerTime,
			Result:         result.CombinedOutput,
			RunDurationSec: int(durationSec),
		}
		proc.logger.Info("processCommandRequest", fmt.Sprintf("%s-%s", request.SubjectHostName, clientIP), result.Error, "command completed in %d seconds", durationSec)
	}
	return
}

/*
GetLatestReportsFromSubject returns the latest subject reports sent by the specified host name.
The maximum number of reports to retrieve must be a positive integer.
The returned values are sorted from latest to oldest, in contrast to the order they were stored internally (oldest to latest).
When there are insufficient number of reports arrived from that subject, the number of returned values will be less than the maximum limit.
*/
func (proc *MessageProcessor) GetLatestReportsFromSubject(hostName string, maxLimit int) (ret []SubjectReport) {
	ret = make([]SubjectReport, 0)
	if maxLimit < 1 {
		return
	}
	proc.mutex.Lock()
	defer proc.mutex.Unlock()
	if reports, exist := proc.SubjectReports[hostName]; exist {
		// Retrieve the latest reports, keep in mind that the order in storage goes from oldest to latest
		if len(*reports) > maxLimit {
			ret = append(ret, (*reports)[len(*reports)-maxLimit:]...)
		} else {
			ret = append(ret, (*reports)...)
		}
		// Reverse the elements in order to return the reports from latest to oldest.
		for i := len(ret)/2 - 1; i >= 0; i-- {
			opp := len(ret) - 1 - i
			ret[i], ret[opp] = ret[opp], ret[i]
		}
	}
	return
}

/*
GetLatestReportsFromSubject returns the latest subject reports received by this message processor from all subjects.
The maximum number of reports to retrieve must be a positive integer.
The returned values are sorted from latest to oldest, in contrast to the order they were stored internally (oldest to latest).
When there are insufficient number of subject reports to retrieve, the number of returned values will be less than the maximum limit.
*/
func (proc *MessageProcessor) GetLatestReports(maxLimit int) (ret []SubjectReport) {
	ret = make([]SubjectReport, 0)
	if maxLimit < 1 {
		return
	}
	proc.mutex.Lock()
	defer proc.mutex.Unlock()
	// Go through all subject reports, starting from the latest (last element) to the oldest (first element).
	subjectReportIndex := make(map[string]int)
	for subject, reports := range proc.SubjectReports {
		if reportsLen := len(*reports); reportsLen > 0 {
			subjectReportIndex[subject] = reportsLen - 1
		}
	}
	for {
		// Already collected enough reports or all subjects have no more reports to offer
		if len(ret) >= maxLimit || len(subjectReportIndex) == 0 {
			break
		}
		noMoreReportsFromSubjects := make([]string, 0)
		// Collect one report from each subject
		for subject := range subjectReportIndex {
			if len(ret) >= maxLimit {
				break
			}
			currentSubjectIndex := subjectReportIndex[subject]
			ret = append(ret, (*proc.SubjectReports[subject])[currentSubjectIndex])
			if currentSubjectIndex == 0 {
				// The subject has no more reports to offer, remove it from the next round.
				noMoreReportsFromSubjects = append(noMoreReportsFromSubjects, subject)
			} else {
				// Retrieve the next latest report in the next round
				subjectReportIndex[subject] = currentSubjectIndex - 1
			}
		}
		// Remove subjects that have no more reports to offer
		for _, subject := range noMoreReportsFromSubjects {
			delete(subjectReportIndex, subject)
		}
	}
	// Sort the result (from all subjects) in chronologically descending order
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].OriginalRequest.ServerTime.After(ret[j].OriginalRequest.ServerTime)
	})
	return
}

/*
removeExpiredSubjects is an internal function that looks at the most recent report made by each subject and removes subjects that have not
made any report for a long time. The internal function assumes that its caller is holding the mutex.
*/
func (proc *MessageProcessor) removeExpiredSubjects() {
	subjectsToRemove := make(map[string]SubjectReport)
	for subject, reports := range proc.SubjectReports {
		latestReport := (*reports)[len(*reports)-1]
		if latestReport.OriginalRequest.ServerTime.Before(time.Now().Add(-SubjectExpirySecond * time.Second)) {
			subjectsToRemove[subject] = latestReport
		}
	}
	for subject, lastReport := range subjectsToRemove {
		proc.logger.Warning("removeExpiredSubjects", subject, nil, "removing the inactive subject, its last report was: %+v", lastReport)
		delete(proc.SubjectReports, subject)
		delete(proc.OutstandingCommands, subject)
		delete(proc.UpcomingSubjectCommand, subject)
	}
}

// App interface

func (proc *MessageProcessor) IsConfigured() bool {
	// Even if command processor may not be ready/configured, the app itself may still receive and store subject reports.
	return true
}

func (proc *MessageProcessor) SelfTest() error {
	return nil
}

func (proc *MessageProcessor) Initialise() error {
	proc.SubjectReports = make(map[string]*[]SubjectReport)
	proc.OutstandingCommands = make(map[string]*OutstandingCommand)
	proc.UpcomingSubjectCommand = make(map[string]string)
	proc.mutex = new(sync.Mutex)
	if proc.CmdProcessor != nil {
		if errs := proc.CmdProcessor.IsSaneForInternet(); len(errs) > 0 {
			return fmt.Errorf("MessageProcessor.Initialise: %+v", errs)
		}
	}
	return nil
}

func (proc *MessageProcessor) Trigger() Trigger {
	return StoreAndForwardMessageProcessorTrigger
}

func (proc *MessageProcessor) Execute(cmd Command) *Result {
	if errResult := cmd.Trim(); errResult != nil {
		return errResult
	}
	// Subject report arrives as a JSON payload
	var incomingReport SubjectReportRequest
	if err := json.Unmarshal([]byte(cmd.Content), &incomingReport); err != nil {
		return &Result{Error: fmt.Errorf("failed to decode JSON input: %w", err)}
	}
	/*
		Store the subject report, the client ID is an IP address by convention.
		If the report carries an app command, it will be processed by this app's own command processor.
		There is no point in honoring the incoming command's timeout configuration, as the result
		is memorised for unlimited retrieval according to rentention timeout.
	*/
	resp := proc.StoreReport(incomingReport, cmd.ClientID, cmd.DaemonName)
	// The response is base64 encoded gob data
	respBytes, err := json.Marshal(resp)
	if err != nil {
		return &Result{Error: fmt.Errorf("failed to encode JSON response: %w", err)}
	}
	return &Result{Output: string(respBytes)}
}
