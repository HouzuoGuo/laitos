package common

import (
	"github.com/HouzuoGuo/laitos/bridge"
	"github.com/HouzuoGuo/laitos/feature"
	"github.com/HouzuoGuo/laitos/global"
	"reflect"
	"testing"
)

func TestCommandProcessor_Process(t *testing.T) {
	// Prepare feature set - the shell execution feature should be available even without configuration
	features := &feature.FeatureSet{}
	if err := features.Initialise(); err != nil {
		t.Fatal(features)
	}
	// Prepare all kinds of command bridges
	commandBridges := []bridge.CommandBridge{
		&bridge.PINAndShortcuts{PIN: "mypin"},
		&bridge.TranslateSequences{Sequences: [][]string{{"alpha", "beta"}}},
	}
	// Prepare all kinds of result bridges
	resultBridges := []bridge.ResultBridge{
		&bridge.ResetCombinedText{},
		&bridge.LintText{TrimSpaces: true, MaxLength: 2},
		&bridge.NotifyViaEmail{},
	}

	proc := CommandProcessor{
		Features:       features,
		CommandBridges: commandBridges,
		ResultBridges:  resultBridges,
	}

	// Try mismatching PIN so that command bridge return early
	cmd := feature.Command{TimeoutSec: 5, Content: "badpin.secho alpha"}
	result := proc.Process(cmd)
	if !reflect.DeepEqual(result.Command, cmd) ||
		result.Error != bridge.ErrPINAndShortcutNotFound || result.Output != "" ||
		result.CombinedOutput != bridge.ErrPINAndShortcutNotFound.Error()[0:2] {
		t.Fatalf("%+v", result)
	}

	// Run a failing command
	cmd = feature.Command{TimeoutSec: 5, Content: "mypin.secho alpha && false"}
	result = proc.Process(cmd)
	if !reflect.DeepEqual(result.Command, feature.Command{TimeoutSec: 5, Content: ".secho beta && false"}) ||
		result.Error == nil || result.Output != "beta\n" || result.CombinedOutput != result.Error.Error()[0:2] {
		t.Fatalf("%+v", result)
	}

	// Run a command that does not trigger a configured feature
	cmd = feature.Command{TimeoutSec: 5, Content: "mypin.tg"}
	result = proc.Process(cmd)
	if !reflect.DeepEqual(result.Command, feature.Command{TimeoutSec: 5, Content: ".tg"}) ||
		result.Error != ErrBadPrefix || result.Output != "" || result.CombinedOutput != ErrBadPrefix.Error()[0:2] {
		t.Fatalf("%+v", result)
	}

	// Run a successful command
	cmd = feature.Command{TimeoutSec: 5, Content: "mypin.secho alpha"}
	result = proc.Process(cmd)
	if !reflect.DeepEqual(result.Command, feature.Command{TimeoutSec: 5, Content: ".secho beta"}) ||
		result.Error != nil || result.Output != "beta\n" || result.CombinedOutput != "be" {
		t.Fatalf("%+v", result)
	}
	// Test the tolerance to extra spaces in feature prefix matcher
	cmd = feature.Command{TimeoutSec: 5, Content: " mypin .s echo alpha "}
	result = proc.Process(cmd)
	if !reflect.DeepEqual(result.Command, feature.Command{TimeoutSec: 5, Content: ".s echo beta"}) ||
		result.Error != nil || result.Output != "beta\n" || result.CombinedOutput != "be" {
		t.Fatalf("%+v", result)
	}

	// Override LPT but LPT parameter values are not given
	cmd = feature.Command{TimeoutSec: 5, Content: "mypin  .lpt   sadf asdf "}
	result = proc.Process(cmd)
	if !reflect.DeepEqual(result.Command, feature.Command{TimeoutSec: 5, Content: ".lpt   sadf asdf"}) ||
		result.Error != ErrBadLPT || result.Output != "" || result.CombinedOutput != ErrBadLPT.Error()[0:2] {
		t.Fatalf("'%v' '%v' '%v' '%v'", result.Error, result.Output, result.CombinedOutput, result.Command)
	}
	// Override LPT using good LPT parameter values
	cmd = feature.Command{TimeoutSec: 1, Content: "mypin  .lpt  5, 2. 3  .s  sleep 2 && echo -n 0123456789 "}
	result = proc.Process(cmd)
	if !reflect.DeepEqual(result.Command, feature.Command{TimeoutSec: 3, Content: ".lpt  5, 2. 3  .s  sleep 2 && echo -n 0123456789"}) ||
		result.Error != nil || result.Output != "0123456789" || result.CombinedOutput != "23456" {
		t.Fatalf("'%v' '%v' '%v' '%+v'", result.Error, result.Output, result.CombinedOutput, result.Command)
	}

	// Trigger emergency stop and try
	global.TriggerEmergencyLockDown()
	cmd = feature.Command{TimeoutSec: 1, Content: "mypin  .lpt  5, 2. 3  .s  sleep 2 && echo -n 0123456789 "}
	if result := proc.Process(cmd); result.Error != global.ErrEmergencyLockDown {
		t.Fatal(result)
	}
	global.EmergencyLockDown = false
}

func TestCommandProcessor_IsSane(t *testing.T) {
	proc := CommandProcessor{
		Features:       nil,
		CommandBridges: nil,
		ResultBridges:  nil,
	}
	if errs := proc.IsSaneForInternet(); len(errs) != 3 {
		t.Fatal(errs)
	}
	// FeatureSet is assigned but not initialised
	proc.Features = &feature.FeatureSet{}
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
	proc.CommandBridges = []bridge.CommandBridge{}
	if errs := proc.IsSaneForInternet(); len(errs) != 2 {
		t.Fatal(errs)
	}
	// PIN bridge has short PIN
	proc.CommandBridges = []bridge.CommandBridge{&bridge.PINAndShortcuts{PIN: "aaaaaa"}}
	if errs := proc.IsSaneForInternet(); len(errs) != 2 {
		t.Fatal(errs)
	}
	// PIN bridge has nothing
	proc.CommandBridges = []bridge.CommandBridge{&bridge.PINAndShortcuts{}}
	if errs := proc.IsSaneForInternet(); len(errs) != 2 {
		t.Fatal(errs)
	}
	// Good PIN bridge
	proc.CommandBridges = []bridge.CommandBridge{&bridge.PINAndShortcuts{PIN: "very-long-pin"}}
	if errs := proc.IsSaneForInternet(); len(errs) != 1 {
		t.Fatal(errs)
	}
	// No linter bridge
	proc.ResultBridges = []bridge.ResultBridge{}
	if errs := proc.IsSaneForInternet(); len(errs) != 1 {
		t.Fatal(errs)
	}
	// Linter bridge has out-of-range max length
	proc.ResultBridges = []bridge.ResultBridge{&bridge.LintText{MaxLength: 1}}
	if errs := proc.IsSaneForInternet(); len(errs) != 1 {
		t.Fatal(errs)
	}
	// Good linter bridge
	proc.ResultBridges = []bridge.ResultBridge{&bridge.LintText{MaxLength: 35}}
	if errs := proc.IsSaneForInternet(); len(errs) != 0 {
		t.Fatal(errs)
	}
}

func TestGetTestCommandProcessor(t *testing.T) {
	if proc := GetTestCommandProcessor(); proc == nil {
		t.Fatal("did not return")
	} else if errs := proc.Features.SelfTest(); len(errs) != 0 {
		t.Fatal(errs)
	}
}
