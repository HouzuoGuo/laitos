package maintenance

import (
	"github.com/HouzuoGuo/laitos/daemon/common"
	"strings"
	"testing"
)

func TestMaintenance_Execute(t *testing.T) {
	features := common.GetTestCommandProcessor().Features
	maint := Daemon{
		BlockSystemLoginExcept: []string{"root", "howard"},
		EnableStartServices:    []string{"does-not-exist"},
		DisableStopServices:    []string{"does-not-exist"},
		InstallPackages:        []string{"htop"},
		SetTimeZone:            "UTC",
		TuneLinux:              true,
		SwapOff:                true,
		FeaturesToTest:         features,
		MailCmdRunnerToTest:    nil, // deliberately nil because it is not involved in this test
		HTTPHandlersToCheck:    nil, // deliberately nil because it is not involved in this test
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
	maint.IntervalSec = MinimumIntervalSec + 1
	if err := maint.Initialise(); err != nil {
		t.Fatal(err)
	}
	TestMaintenance(&maint, t)
}
