package healthcheck

import (
	"github.com/HouzuoGuo/laitos/email"
	"github.com/HouzuoGuo/laitos/feature"
	"github.com/HouzuoGuo/laitos/frontend/common"
	"net"
	"strings"
	"testing"
	"time"
)

func TestHealthCheck_Execute(t *testing.T) {
	// Port is now listening
	go func() {
		listener, err := net.Listen("tcp", "127.0.0.1:9818")
		if err != nil {
			t.Fatal(err)
		}
		for {
			if _, err := listener.Accept(); err != nil {
				t.Fatal(err)
			}
		}
	}()
	features := common.GetTestCommandProcessor().Features
	check := HealthCheck{
		TCPPorts:    []int{9818},
		IntervalSec: 10,
		Mailer: email.Mailer{
			MailFrom: "howard@localhost",
			MTAHost:  "localhost",
			MTAPort:  25,
		},
		Recipients:      []string{"howard@localhost"},
		FeaturesToCheck: features,
		MailpToCheck:    nil, // deliberately nil
	}
	if !check.Execute() {
		t.Fatal("some check failed")
	}
	// Break a feature
	check.FeaturesToCheck.LookupByTrigger[".s"] = &feature.Shell{}
	if check.Execute() {
		t.Fatal("did not fail")
	}
	check.FeaturesToCheck.LookupByTrigger[".s"] = &feature.Shell{InterpreterPath: "/bin/bash"}
	if err := check.Initialise(); err == nil || strings.Index(err.Error(), "IntervalSec") == -1 {
		t.Fatal("did not error")
	}
	// Expect checks to begin within a second
	check.IntervalSec = 300
	if err := check.Initialise(); err != nil {
		t.Fatal(err)
	}
	go func() {
		if err := check.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()
	time.Sleep(1 * time.Second)
}
