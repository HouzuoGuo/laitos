package common

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRecurringCommands(t *testing.T) {
	cmds := RecurringCommands{}
	if err := cmds.Initialise(); !strings.Contains(err.Error(), "IntervalSec") {
		t.Fatal(err)
	}
	cmds.IntervalSec = 1
	if err := cmds.Initialise(); !strings.Contains(err.Error(), "MaxResults") {
		t.Fatal(err)
	}
	cmds.MaxResults = 4
	cmds.CommandProcessor = GetTestCommandProcessor()
	cmds.PreConfiguredCommands = []string{
		TestCommandProcessorPIN + ".s echo first",
		TestCommandProcessorPIN + ".s echo second",
	}
	cmds.Initialise()

	// There shall be no transient commands or results to begin with
	if a := cmds.GetTransientCommands(); !reflect.DeepEqual(a, []string{}) {
		t.Fatal(a)
	}
	if a := cmds.GetResults(); !reflect.DeepEqual(a, []string{}) {
		t.Fatal(a)
	}

	// Add two dummy transient commands and clear.
	cmds.AddTransientCommand("transient 1")
	cmds.AddTransientCommand("transient 2")
	if a := cmds.GetTransientCommands(); !reflect.DeepEqual(a, []string{"transient 1", "transient 2"}) {
		t.Fatal(a)
	}
	cmds.ClearTransientCommands()
	if a := cmds.GetTransientCommands(); !reflect.DeepEqual(a, []string{}) {
		t.Fatal(a)
	}

	// Add two proper transient commands
	cmds.AddTransientCommand(TestCommandProcessorPIN + ".s echo third")
	cmds.AddTransientCommand(TestCommandProcessorPIN + ".s echo fourth")

	// Collect result from all four commands
	cmds.runAllCommands()
	if a := cmds.GetResults(); !reflect.DeepEqual(
		[]string{strings.TrimSpace(a[0]), strings.TrimSpace(a[1]), strings.TrimSpace(a[2]), strings.TrimSpace(a[3])},
		[]string{"first", "second", "third", "fourth"}) {
		t.Fatal(a)
	}
	if a := cmds.GetResults(); !reflect.DeepEqual(a, []string{}) {
		t.Fatal(a)
	}

	// Chuck in some arbitrary strings
	cmds.AddArbitraryTextToResult("arbitrary 1")
	cmds.AddArbitraryTextToResult("arbitrary 2")
	if a := cmds.GetResults(); !reflect.DeepEqual(a, []string{"arbitrary 1", "arbitrary 2"}) {
		t.Fatal(a)
	}

	// Run in a loop and check for result
	var stopped bool
	go func() {
		cmds.Start()
		stopped = true
	}()
	time.Sleep(time.Duration(cmds.IntervalSec*5) * time.Second)
	if a := cmds.GetResults(); len(a) != len(cmds.transientCommands)+len(cmds.PreConfiguredCommands) {
		t.Fatal(a, len(a), len(cmds.transientCommands)+len(cmds.PreConfiguredCommands))
	}

	// Expect it to stop within 2 seconds
	cmds.Stop()
	time.Sleep(2 * time.Second)
	if !stopped {
		t.Fatal("did not stop in time")
	}

	// Repeatedly stopping the loop should not matter
	cmds.Stop()
	cmds.Stop()
	cmds.Stop()
}
