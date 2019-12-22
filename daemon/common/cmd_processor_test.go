package common

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/toolbox"
	"github.com/HouzuoGuo/laitos/toolbox/filter"
)

func TestCommandProcessor_NonWindows(t *testing.T) {
	// Prepare feature set - the shell execution feature should be available even without configuration
	features := &toolbox.FeatureSet{}
	if err := features.Initialise(); err != nil {
		t.Fatal(features)
	}
	// Prepare all kinds of command bridges
	commandBridges := []filter.CommandFilter{
		&filter.PINAndShortcuts{PIN: "mypin"},
		&filter.TranslateSequences{Sequences: [][]string{{"alpha", "beta"}}},
	}
	// Prepare all kinds of result bridges
	resultBridges := []filter.ResultFilter{
		&filter.LintText{TrimSpaces: true, MaxLength: 2},
		&filter.NotifyViaEmail{},
	}

	proc := CommandProcessor{
		Features:       features,
		CommandFilters: commandBridges,
		ResultFilters:  resultBridges,
	}

	// Try mismatching PIN so that command bridge return early
	cmd := toolbox.Command{TimeoutSec: 5, Content: "badpin.secho alpha"}
	result := proc.Process(cmd, true)
	if !reflect.DeepEqual(result.Command, toolbox.Command{TimeoutSec: 5, Content: ""}) ||
		result.Error != filter.ErrPINAndShortcutNotFound || result.Output != "" ||
		result.CombinedOutput != filter.ErrPINAndShortcutNotFound.Error()[0:2] {
		t.Fatalf("%+v", result)
	}

	// Run a failing command - be aware of the word substitution conducted by command filter
	cmd = toolbox.Command{TimeoutSec: 5, Content: "mypin.secho alpha; does-not-exist"}
	result = proc.Process(cmd, true)
	if !reflect.DeepEqual(result.Command, toolbox.Command{TimeoutSec: 5, Content: ".secho beta; does-not-exist"}) ||
		result.Error == nil || !strings.Contains(result.Output, "beta") || result.CombinedOutput != result.Error.Error()[0:2] {
		t.Fatalf("%+v", result)
	}

	// Run a command that does not trigger a configured feature
	cmd = toolbox.Command{TimeoutSec: 5, Content: "mypin.tz"}
	result = proc.Process(cmd, true)
	if !reflect.DeepEqual(result.Command, toolbox.Command{TimeoutSec: 5, Content: ".tz"}) ||
		result.Error != ErrBadPrefix || result.Output != "" || result.CombinedOutput != ErrBadPrefix.Error()[0:2] {
		t.Fatalf("%+v", result)
	}

	// Run a successful command - be aware of the word substitution conducted by command filter
	cmd = toolbox.Command{TimeoutSec: 5, Content: "mypin.secho alpha"}
	result = proc.Process(cmd, true)
	if !reflect.DeepEqual(result.Command, toolbox.Command{TimeoutSec: 5, Content: ".secho beta"}) ||
		result.Error != nil || !strings.Contains(result.Output, "beta") || result.CombinedOutput != "be" {
		t.Fatalf("%+v", result)
	}
	// Test the tolerance to extra spaces in feature prefix matcher
	cmd = toolbox.Command{TimeoutSec: 5, Content: " mypin .s echo alpha "}
	result = proc.Process(cmd, true)
	if !reflect.DeepEqual(result.Command, toolbox.Command{TimeoutSec: 5, Content: ".s echo beta"}) ||
		result.Error != nil || !strings.Contains(result.Output, "beta") || result.CombinedOutput != "be" {
		t.Fatalf("%+v", result)
	}

	// Override PLT but PLT parameter values are not given
	cmd = toolbox.Command{TimeoutSec: 5, Content: "mypin  .plt   sadf asdf "}
	result = proc.Process(cmd, true)
	if !reflect.DeepEqual(result.Command, toolbox.Command{TimeoutSec: 5, Content: ""}) ||
		result.Error != ErrBadPLT || result.Output != "" || result.CombinedOutput != ErrBadPLT.Error()[0:2] {
		t.Fatalf("%v | %v | %v |%+v", result.Error, result.Output, result.CombinedOutput, result.Command)
	}
	// Override PLT using good PLT parameter values
	cmd = toolbox.Command{TimeoutSec: 1, Content: "mypin  .plt  2, 5. 4  .s  sleep 2 ; echo 0123456789 "}
	result = proc.Process(cmd, true)
	if !reflect.DeepEqual(result.Command, toolbox.Command{TimeoutSec: 4, Content: "  .s  sleep 2 ; echo 0123456789"}) ||
		result.Error != nil || !strings.Contains(result.Output, "0123456789") || result.CombinedOutput != "23456" {
		t.Fatalf("%v | %v | %v | %+v", result.Error, result.Output, result.CombinedOutput, result.Command)
	}

	// Trigger emergency lock down and try
	misc.TriggerEmergencyLockDown()
	cmd = toolbox.Command{TimeoutSec: 1, Content: "mypin  .plt  2, 5. 3  .s  sleep 2 ;  echo 0123456789 "}
	if result := proc.Process(cmd, true); result.Error != misc.ErrEmergencyLockDown {
		t.Fatal(result)
	}
	misc.EmergencyLockDown = false
}

func TestCommandProcessor_RateLimit(t *testing.T) {
	proc := GetTestCommandProcessor()
	proc.MaxCmdPerSec = 2

	// Exceed the rate limit by repeatedly executing a command
	succeeded := 0
	failed := 0
	for i := 0; i < 30; i++ {
		if result := proc.Process(toolbox.Command{Content: "verysecret .elog", TimeoutSec: 10}, true); result.Error == nil {
			succeeded++
		} else if result.Error == ErrRateLimitExceeded {
			failed++
		}
	}
	if succeeded < 2 || succeeded > 4 || failed < 30-succeeded {
		t.Fatal(succeeded, failed)
	}

	// Wait for rate limit to expire and retry
	time.Sleep(2 * time.Second)
	if result := proc.Process(toolbox.Command{Content: "verysecret .elog", TimeoutSec: 10}, true); result.Error != nil {
		t.Fatal(result.Error)
	}

	// Use the default hard upper limit with a new command processor
	proc = GetTestCommandProcessor()
	if result := proc.Process(toolbox.Command{Content: "verysecret .elog", TimeoutSec: 10}, true); result.Error != nil {
		t.Fatal(result.Error)
	}
	if proc.MaxCmdPerSec != MaxCmdPerSecHardLimit {
		t.Fatal(proc.MaxCmdPerSec)
	}
}

func TestCommandProcessorIsSaneForInternet(t *testing.T) {
	proc := CommandProcessor{
		Features:       nil,
		CommandFilters: nil,
		ResultFilters:  nil,
	}
	if !proc.IsEmpty() {
		t.Fatal("not empty")
	}
	// Give it some filters but leave PIN empty, it should still be considered an empty configuration.
	proc.CommandFilters = []filter.CommandFilter{
		&filter.PINAndShortcuts{}, // leave PIN empty
	}
	proc.ResultFilters = []filter.ResultFilter{
		&filter.SayEmptyOutput{},
	}
	if !proc.IsEmpty() {
		t.Fatal("not empty")
	}
	// Empty configuration is not sane for facing visitors from the public Internet
	if errs := proc.IsSaneForInternet(); len(errs) != 3 {
		t.Fatal(errs)
	}
	// FeatureSet is assigned but not initialised
	proc.Features = &toolbox.FeatureSet{}
	if errs := proc.IsSaneForInternet(); len(errs) != 3 {
		t.Fatal(errs)
	}
	// Good feature set
	if err := proc.Features.Initialise(); err != nil {
		t.Fatal(err)
	}
	if errs := proc.IsSaneForInternet(); len(errs) != 2 {
		t.Fatal(errs)
	}
	// No PIN bridge
	proc.CommandFilters = []filter.CommandFilter{}
	if errs := proc.IsSaneForInternet(); len(errs) != 2 {
		t.Fatal(errs)
	}
	// PIN bridge has short PIN
	proc.CommandFilters = []filter.CommandFilter{&filter.PINAndShortcuts{PIN: "aaaaaa"}}
	if errs := proc.IsSaneForInternet(); len(errs) != 2 {
		t.Fatal(errs)
	}
	// Despite PIN being very short, the command processor is not without configuration.
	if proc.IsEmpty() {
		t.Fatal("should not be empty")
	}
	// PIN bridge has nothing
	proc.CommandFilters = []filter.CommandFilter{&filter.PINAndShortcuts{}}
	if errs := proc.IsSaneForInternet(); len(errs) != 2 {
		t.Fatal(errs)
	}
	// Good PIN bridge
	proc.CommandFilters = []filter.CommandFilter{&filter.PINAndShortcuts{PIN: "very-long-pin"}}
	if errs := proc.IsSaneForInternet(); len(errs) != 1 {
		t.Fatal(errs)
	}
	// No linter bridge
	proc.ResultFilters = []filter.ResultFilter{}
	if errs := proc.IsSaneForInternet(); len(errs) != 1 {
		t.Fatal(errs)
	}
	// Linter bridge has out-of-range max length
	proc.ResultFilters = []filter.ResultFilter{&filter.LintText{MaxLength: 1}}
	if errs := proc.IsSaneForInternet(); len(errs) != 1 {
		t.Fatal(errs)
	}
	// Good linter bridge
	proc.ResultFilters = []filter.ResultFilter{&filter.LintText{MaxLength: 35}}
	if errs := proc.IsSaneForInternet(); len(errs) != 0 {
		t.Fatal(errs)
	}
}

func TestGetTestCommandProcessor(t *testing.T) {
	proc := GetTestCommandProcessor()
	if testErr := proc.Features.SelfTest(); testErr != nil {
		t.Fatal(testErr)
	} else if saneErrs := proc.IsSaneForInternet(); len(saneErrs) > 0 {
		t.Fatal(saneErrs)
	} else if result := proc.Process(toolbox.Command{Content: "verysecret .elog", TimeoutSec: 10}, true); result.Error != nil {
		t.Fatal(result.Error)
	}
}

func TestConcealedLogMessages(t *testing.T) {
	proc := GetTestCommandProcessor()
	// These two features are the ones to be concealed from log
	proc.Features.AESDecrypt = toolbox.GetTestAESDecrypt()
	proc.Features.TwoFACodeGenerator = toolbox.TwoFACodeGenerator{SecretFile: toolbox.GetTestAESDecrypt().EncryptedFiles[toolbox.TestAESDecryptFileAlphaName]}
	// Reinitialise features so that it understands the two new prefixes
	if err := proc.Features.Initialise(); err != nil {
		t.Fatal(err)
	}
	proc.Process(toolbox.Command{Content: "verysecret .a does not matter", TimeoutSec: 10}, true)
	proc.Process(toolbox.Command{Content: "verysecret .2 does not matter", TimeoutSec: 10}, true)
	t.Log("Please observe <hidden due to AESDecryptTrigger or TwoFATrigger> from log output, otherwise consider this test is failed")
}

func TestGetEmptyCommandProcessor(t *testing.T) {
	proc := GetEmptyCommandProcessor()
	if testErr := proc.Features.SelfTest(); testErr != nil {
		t.Fatal(testErr)
	} else if saneErrs := proc.IsSaneForInternet(); len(saneErrs) > 0 {
		t.Fatal(saneErrs)
	} else if result := proc.Process(toolbox.Command{Content: "verysecret .elog", TimeoutSec: 10}, true); result.Error == nil {
		t.Fatal("did not error")
	}
}
