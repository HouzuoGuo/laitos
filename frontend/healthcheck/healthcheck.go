package healthcheck

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/email"
	"github.com/HouzuoGuo/laitos/feature"
	"github.com/HouzuoGuo/laitos/lalog"
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
	TCPPorts    []int              `json:"TCPPorts"`    // Check that these TCP ports are listening on this host
	IntervalSec int                `json:"IntervalSec"` // Check TCP ports and features at this interval
	Mailer      email.Mailer       `json:"Mailer"`      // Send notification mails via this mailer
	Recipients  []string           `json:"Recipients"`  // Address of recipients of notification mails
	Features    feature.FeatureSet `json:"-"`
	Logger      lalog.Logger       `json:"-"`
}

// Check TCP ports and features, return all-OK or not.
func (check *HealthCheck) Execute() bool {
	check.Logger.Printf("Execute", "", nil, "running now")
	allOK := true
	// Check TCP ports in parallel
	checkTCPPorts := make(map[int]bool)
	waitPorts := new(sync.WaitGroup)
	waitPorts.Add(len(check.TCPPorts))
	for _, portNumber := range check.TCPPorts {
		go func(portNumber int) {
			conn, err := net.DialTimeout("tcp", "localhost:"+strconv.Itoa(portNumber), TCPConnectionTimeoutSec*time.Second)
			fmt.Println("CONN ERR", conn, err)
			checkTCPPorts[portNumber] = err == nil
			allOK = allOK && checkTCPPorts[portNumber]
			if err == nil {
				conn.Close()
			}
			waitPorts.Done()
		}(portNumber)
	}
	waitPorts.Wait()
	// Check features
	featureErrs := check.Features.SelfTest()
	allOK = allOK && len(featureErrs) == 0
	// Compose mail body
	var mailMessage bytes.Buffer
	if allOK {
		mailMessage.WriteString("All OK\n\n")
	} else {
		mailMessage.WriteString("There are errors.\n\n")
	}
	for _, portNumber := range check.TCPPorts {
		if checkTCPPorts[portNumber] {
			mailMessage.WriteString(fmt.Sprintf("TCP %d: OK\n", portNumber))
		} else {
			mailMessage.WriteString(fmt.Sprintf("TCP %d: Error\n", portNumber))
		}
	}
	if len(featureErrs) == 0 {
		mailMessage.WriteString("\nFeatures: OK\n")
	} else {
		for trigger, err := range featureErrs {
			mailMessage.WriteString(fmt.Sprintf("\nFeatures %s: %+v\n", trigger, err))
		}
	}
	mailMessage.WriteString("\nLogs:\n")
	lalog.GlobalRingBuffer.Iterate(func(_ uint64, entry string) bool {
		mailMessage.WriteString(entry + "\n")
		return true
	})
	// Send away!
	if allOK {
		check.Logger.Printf("Execute", "", nil, "completed with everything being OK")
	} else {
		check.Logger.Printf("Execute", "", nil, "completed with some errors")
	}
	if err := check.Mailer.Send(email.OutgoingMailSubjectKeyword+"-healthcheck", mailMessage.String(), check.Recipients...); err == nil {
		check.Logger.Printf("Execute", "", err, "failed to send notification mail")
	}
	return allOK
}

func (check *HealthCheck) Initialise() error {
	if check.IntervalSec < 30 {
		return errors.New("HealthCheck.StartAndBlock: IntervalSec must be above 29")
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
		time.Sleep(time.Duration(check.IntervalSec) * time.Second)
		check.Execute()
	}
}
