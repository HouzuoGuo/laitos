package healthcheck

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/email"
	"github.com/HouzuoGuo/laitos/feature"
	"github.com/HouzuoGuo/laitos/frontend/mailp"
	"github.com/HouzuoGuo/laitos/global"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"
)

const (
	TCPConnectionTimeoutSec = 10
)

// Periodically check TCP ports and feature set, send notification mail along with latest log entries.
type HealthCheck struct {
	TCPPorts        []int                `json:"TCPPorts"`    // Check that these TCP ports are listening on this host
	IntervalSec     int                  `json:"IntervalSec"` // Check TCP ports and features at this interval
	Mailer          email.Mailer         `json:"Mailer"`      // Send notification mails via this mailer
	Recipients      []string             `json:"Recipients"`  // Address of recipients of notification mails
	FeaturesToCheck *feature.FeatureSet  `json:"-"`           // Health check subject - features and their API keys
	MailpToCheck    *mailp.MailProcessor `json:"-"`           // Health check subject - mail processor and its mailer
	Logger          global.Logger        `json:"-"`
}

// Check TCP ports and features, return all-OK or not.
func (check *HealthCheck) Execute() (string, bool) {
	check.Logger.Printf("Execute", "", nil, "running now")
	allOK := true
	// Check TCP ports in parallel
	portCheckResult := make(map[int]bool)
	portCheckMutex := new(sync.Mutex)
	waitPorts := new(sync.WaitGroup)
	waitPorts.Add(len(check.TCPPorts))
	for _, portNumber := range check.TCPPorts {
		go func(portNumber int) {
			conn, err := net.DialTimeout("tcp", "localhost:"+strconv.Itoa(portNumber), TCPConnectionTimeoutSec*time.Second)
			portCheckMutex.Lock()
			portCheckResult[portNumber] = err == nil
			allOK = allOK && portCheckResult[portNumber]
			portCheckMutex.Unlock()
			if err == nil {
				conn.Close()
			}
			waitPorts.Done()
		}(portNumber)
	}
	waitPorts.Wait()
	// Check features and mail processor
	featureErrs := make(map[feature.Trigger]error)
	if check.FeaturesToCheck != nil {
		featureErrs = check.FeaturesToCheck.SelfTest()
	}
	var mailpErr error
	if check.MailpToCheck != nil {
		mailpErr = check.MailpToCheck.SelfTest()
	}
	allOK = allOK && len(featureErrs) == 0 && mailpErr == nil
	// Compose mail body
	var result bytes.Buffer
	if allOK {
		result.WriteString("All OK\n")
	} else {
		result.WriteString("There are errors!!!\n")
	}
	// 0 - runtime info
	result.WriteString(feature.GetRuntimeInfo())
	// 1 - port checks
	result.WriteString("\nPorts:\n")
	for _, portNumber := range check.TCPPorts {
		if portCheckResult[portNumber] {
			result.WriteString(fmt.Sprintf("TCP %d: OK\n", portNumber))
		} else {
			result.WriteString(fmt.Sprintf("TCP %d: Error\n", portNumber))
		}
	}
	// 2 - feature checks
	if len(featureErrs) == 0 {
		result.WriteString("\nFeatures: OK\n")
	} else {
		for trigger, err := range featureErrs {
			result.WriteString(fmt.Sprintf("\nFeatures %s: %+v\n", trigger, err))
		}
	}
	// 3 - mail processor checks
	if mailpErr == nil {
		result.WriteString("\nMail processor: OK\n")
	} else {
		result.WriteString(fmt.Sprintf("\nMail processor: %v\n", mailpErr))
	}
	// 4 - warnings
	result.WriteString("\nWarnings:\n")
	result.WriteString(feature.GetLatestWarnings())
	// 5 - logs
	result.WriteString("\nLogs:\n")
	result.WriteString(feature.GetLatestLog())
	// 6 - stack traces
	result.WriteString("\nStack traces:\n")
	result.WriteString(feature.GetGoroutineStacktraces())
	// Send away!
	if allOK {
		check.Logger.Printf("Execute", "", nil, "completed with everything being OK")
	} else {
		fmt.Println(result.String())
		check.Logger.Warningf("Execute", "", nil, "completed with some errors")
	}
	if err := check.Mailer.Send(email.OutgoingMailSubjectKeyword+"-healthcheck", result.String(), check.Recipients...); err == nil {
		check.Logger.Warningf("Execute", "", err, "failed to send notification mail")
	}
	return result.String(), allOK
}

func (check *HealthCheck) Initialise() error {
	check.Logger = global.Logger{ComponentName: "HealthCheck", ComponentID: strconv.Itoa(check.IntervalSec)}
	if check.IntervalSec < 120 {
		return errors.New("HealthCheck.StartAndBlock: IntervalSec must be above 119")
	}
	return nil
}

/*
You may call this function only after having called Initialise()!
Start health check loop and block until this program exits.
*/
func (check *HealthCheck) StartAndBlock() error {
	sort.Ints(check.TCPPorts)
	for {
		if global.EmergencyLockDown {
			return global.ErrEmergencyLockDown
		}
		time.Sleep(time.Duration(check.IntervalSec) * time.Second)
		check.Execute()
	}
}
