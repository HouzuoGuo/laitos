package maintenance

import (
	"strings"
	"testing"

	"github.com/HouzuoGuo/laitos/toolbox"
)

func TestMaintenance_Execute(t *testing.T) {
	features := toolbox.GetTestCommandProcessor().Features
	maint := Daemon{
		BlockSystemLoginExcept: []string{"root", "howard"},
		EnableStartServices:    []string{"does-not-exist"},
		DisableStopServices:    []string{"does-not-exist"},
		InstallPackages:        []string{"htop"},
		SetTimeZone:            "UTC",
		TuneLinux:              true,
		DoEnhanceFileSecurity:  true,
		SwapFileSizeMB:         100,
		FeaturesToTest:         features,
		PreScriptUnix:          "touch /tmp/laitos-maintenance-pre-script-test",
		MailCmdRunnerToTest:    nil, // deliberately nil because it is not involved in this test
		HTTPHandlersToCheck:    nil, // deliberately nil because it is not involved in this test
	}

	// Test default settings
	if err := maint.Initialise(); err != nil || maint.IntervalSec != MinimumIntervalSec {
		t.Fatal(err)
	}
	// Test invalid settings
	maint.IntervalSec = 1
	if err := maint.Initialise(); !strings.Contains(err.Error(), "IntervalSec") {
		t.Fatal(err)
	}
	// Prepare settings for test
	maint.IntervalSec = MinimumIntervalSec
	if err := maint.Initialise(); err != nil {
		t.Fatal(err)
	}
	TestMaintenance(&maint, t)
}
