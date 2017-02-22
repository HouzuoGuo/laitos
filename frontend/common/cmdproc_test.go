package common

import (
	"github.com/HouzuoGuo/websh/bridge"
	"github.com/HouzuoGuo/websh/feature"
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
		&bridge.CommandPINOrShortcut{PIN: "mypin"},
		&bridge.CommandTranslator{Sequences: [][]string{{"alpha", "beta"}}},
	}
	// Prepare all kinds of result bridges
	resultBridges := []bridge.ResultBridge{
		&bridge.ResetCombinedText{},
		&bridge.LintCombinedText{TrimSpaces: true, MaxLength: 2},
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
	if !reflect.DeepEqual(result.Command, feature.Command{TimeoutSec: 5, Content: "echo beta && false"}) ||
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
	if !reflect.DeepEqual(result.Command, feature.Command{TimeoutSec: 5, Content: "echo beta"}) ||
		result.Error != nil || result.Output != "beta\n" || result.CombinedOutput != "be" {
		t.Fatalf("%+v", result)
	}
	// Test the tolerance to extra spaces in feature prefix matcher
	cmd = feature.Command{TimeoutSec: 5, Content: " mypin .s echo alpha "}
	result = proc.Process(cmd)
	if !reflect.DeepEqual(result.Command, feature.Command{TimeoutSec: 5, Content: "echo beta"}) ||
		result.Error != nil || result.Output != "beta\n" || result.CombinedOutput != "be" {
		t.Fatalf("%+v", result)
	}
}
