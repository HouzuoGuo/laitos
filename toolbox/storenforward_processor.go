package toolbox

import (
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
)

const (
	/*
		MaxReportsPerHostName is the maximum number of reports to be kept in memory per each subject, identified by subject's self-reported
		host name, regardless of which server daemon received the report.
		2000 reports are plentiful for at least two days of reports at 2 minutes/report.
		The value also determines how often expired subjects are cleaned up.
	*/
	MaxReportsPerHostName = 2000
	// SubjectExpirySecond is the number of seconds after which if a subject is not heard from again it will be removed.
	SubjectExpirySecond = 72 * 3600
)

/*
SubjectReport is a system status report arrived at the store&forward message processor, it contains the original report along with
additional subject information collected by the message processor.
*/
type SubjectReport struct {
	OriginalRequest SubjectReportRequest // OriginalRequest is the agent's report received by the message processor.
	SubjectClientIP string               // SubjectClientID is the subject's IP address as observed by server daemon.
	DaemonName      string               // DaemonName is the name of server daemon that received this report.
	StoredAt        time.Time            // StoredAt is the server's system time when the report is stored.
}

/*
MessageProcessor collects subject reports and relays outstanding app command requests and responses using the store&forward technique.
It implements app interfaces and works just like an ordinary app, while also offering routines that help running a store&forward
messaging daemon.
*/
type MessageProcessor struct {
	// SubjectReports is a map of subject's self-reported host name and its most recent reports, sorted from earliest to latest.
	SubjectReports map[string]*[]SubjectReport
	// OutstandingCommands is a map of subject's self reported host name and an app command the processor would like it to run.
	OutstandingCommands map[string]string
	// CmdProcessor processes app commands as requested by a remote server.
	CmdProcessor *CommandProcessor

	// totalReports is the total number of reports received thus far.
	totalReports int
	// mutex prevents concurrent modifications made to internal structures.
	mutex  *sync.Mutex
	logger lalog.Logger
}

// NewMessageProcessor returns an initialised message processor.
func NewMessageProcessor() *MessageProcessor {
	return &MessageProcessor{
		SubjectReports:      make(map[string]*[]SubjectReport),
		OutstandingCommands: make(map[string]string),
		totalReports:        0,
		mutex:               new(sync.Mutex),
	}
}

// StoreReports stores the most recent report from a subject and evicts older report automatically.
func (proc *MessageProcessor) StoreReport(request SubjectReportRequest, clientIP, daemonName string) {
	proc.mutex.Lock()
	defer proc.mutex.Unlock()
	hostName := request.SubjectHostName
	reports := proc.SubjectReports[hostName]
	if reports == nil {
		// Reserve the maximum capacity as computer subjects often stay online for quite a while
		newReports := make([]SubjectReport, 0, MaxReportsPerHostName)
		reports = &newReports
	}
	newReport := SubjectReport{
		OriginalRequest: request,
		SubjectClientIP: clientIP,
		DaemonName:      daemonName,
		StoredAt:        time.Now(),
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
		proc.logger.Info("StoreReport", clientIP, nil, "going to clean up expired subjects")
		proc.removeExpiredSubjects()
	}
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
		return ret[i].StoredAt.After(ret[j].StoredAt)
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
		if latestReport.StoredAt.Before(time.Now().Add(-SubjectExpirySecond * time.Second)) {
			subjectsToRemove[subject] = latestReport
		}
	}
	for subject, lastReport := range subjectsToRemove {
		proc.logger.Warning("removeExpiredSubjects", subject, nil, "removing the inactive subject, its last report was: %+v", lastReport)
		delete(proc.SubjectReports, subject)
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
	return nil
}

func (proc *MessageProcessor) Trigger() Trigger {
	// "mp" would have been more suitable, however "m" letter is already taken by send-mail app.
	return ".0m"
}

func (proc *MessageProcessor) Execute(cmd Command) *Result {
	if errResult := cmd.Trim(); errResult != nil {
		return errResult
	}
	// Subject report arrives as an app command in a JSON document
	var incomingReport SubjectReportRequest
	if err := json.Unmarshal([]byte(cmd.Content), &incomingReport); err != nil {
		return &Result{Error: err}
	}
	return nil
}
