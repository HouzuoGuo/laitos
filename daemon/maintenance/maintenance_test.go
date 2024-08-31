package maintenance

import (
	"strings"
	"testing"

	"github.com/HouzuoGuo/laitos/toolbox"
	"github.com/stretchr/testify/require"
)

func TestMaintenance_Execute(t *testing.T) {
	features := toolbox.GetTestCommandProcessor().Features
	maint := Daemon{
		BlockSystemLoginExcept:     []string{"root", "howard"},
		EnableStartServices:        []string{"does-not-exist"},
		DisableStopServices:        []string{"does-not-exist"},
		InstallPackages:            []string{"htop"},
		SetTimeZone:                "UTC",
		TuneLinux:                  true,
		DoEnhanceFileSecurity:      true,
		SwapFileSizeMB:             100,
		ToolboxSelfTest:            features,
		ScriptForUnix:              "touch /tmp/laitos-maintenance-pre-script-test",
		ShrinkSystemdJournalSizeMB: 1000,
		MailCommandRunnerSelfTest:  nil, // deliberately nil because it is not involved in this test
		HttpHandlersSelfTest:       nil, // deliberately nil because it is not involved in this test
	}

	// Test default settings
	require.NoError(t, maint.Initialise())
	require.Equal(t, MinimumIntervalSec, maint.IntervalSec)
	require.Equal(t, 60, maint.PrometheusScrapeIntervalSec)
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
