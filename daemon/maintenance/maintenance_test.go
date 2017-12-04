package maintenance

import (
	"fmt"
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/inet"
	"strings"
	"testing"
)

func TestMaintenance_Execute(t *testing.T) {
	features := common.GetTestCommandProcessor().Features
	maint := Daemon{
		MailClient: inet.MailClient{
			MailFrom: "howard@localhost",
			MTAHost:  "localhost",
			MTAPort:  25,
		},
		Recipients:          []string{"howard@localhost"},
		FeaturesToTest:      features,
		MailCmdRunnerToTest: nil, // deliberately nil because it is not involved in this test
		HTTPHandlersToCheck: nil, // deliberately nil because it is not involved in this test
	}

	// Test default settings
	if err := maint.Initialise(); err != nil || maint.IntervalSec != 24*3600 {
		t.Fatal(err)
	}
	// Test invalid settings
	maint.IntervalSec = 1
	if err := maint.Initialise(); !strings.Contains(err.Error(), "IntervalSec") {
		t.Fatal(err)
	}
	// Prepare settings for test
	maint.IntervalSec = 3600
	if err := maint.Initialise(); err != nil {
		t.Fatal(err)
	}
	TestMaintenance(&maint, t)
}

func TestSystemMaintenance(t *testing.T) {
	// Just make sure the function does not crash
	maint := Daemon{
		IntervalSec: 3600,
		MailClient: inet.MailClient{
			MailFrom: "howard@localhost",
			MTAHost:  "localhost",
			MTAPort:  25,
		},
		Recipients:          []string{"howard@localhost"},
		FeaturesToTest:      common.GetTestCommandProcessor().Features,
		MailCmdRunnerToTest: nil, // deliberately nil because it is not involved in this test
		HTTPHandlersToCheck: nil, // deliberately nil because it is not involved in this test
	}
	ret := maint.SystemMaintenance()
	// Developer please manually inspect the output
	fmt.Println(ret)
}
