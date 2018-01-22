package common

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCommandTimer(t *testing.T) {
	timer := CommandTimer{}
	if err := timer.Initialise(); !strings.Contains(err.Error(), "IntervalSec") {
		t.Fatal(err)
	}
	timer.IntervalSec = 1
	if err := timer.Initialise(); !strings.Contains(err.Error(), "MaxResults") {
		t.Fatal(err)
	}
	timer.MaxResults = 4
	timer.CommandProcessor = GetTestCommandProcessor()
	timer.PreConfiguredCommands = []string{
		TestCommandProcessorPIN + ".s echo first",
		TestCommandProcessorPIN + ".s echo second",
	}
	timer.Initialise()

	// There shall be no transient commands or results to begin with
	if a := timer.GetTransientCommands(); !reflect.DeepEqual(a, []string{}) {
		t.Fatal(a)
	}
	if a := timer.GetResults(); !reflect.DeepEqual(a, []string{}) {
		t.Fatal(a)
	}

	// Add two dummy transient commands and clear.
	timer.AddTransientCommand("transient 1")
	timer.AddTransientCommand("transient 2")
	if a := timer.GetTransientCommands(); !reflect.DeepEqual(a, []string{"transient 1", "transient 2"}) {
		t.Fatal(a)
	}
	timer.ClearTransientCommands()
	if a := timer.GetTransientCommands(); !reflect.DeepEqual(a, []string{}) {
		t.Fatal(a)
	}

	// Add two proper transient commands
	timer.AddTransientCommand(TestCommandProcessorPIN + ".s echo third")
	timer.AddTransientCommand(TestCommandProcessorPIN + ".s echo fourth")

	// Collect result from all four commands
	timer.runAllCommands()
	if a := timer.GetResults(); !reflect.DeepEqual(a, []string{"first", "second", "third", "fourth"}) {
		t.Fatal(a)
	}
	if a := timer.GetResults(); !reflect.DeepEqual(a, []string{}) {
		t.Fatal(a)
	}

	// Chuck in some arbitrary strings
	timer.AddArbitraryTextToResult("arbitrary 1")
	timer.AddArbitraryTextToResult("arbitrary 2")
	if a := timer.GetResults(); !reflect.DeepEqual(a, []string{"arbitrary 1", "arbitrary 2"}) {
		t.Fatal(a)
	}

	// Run in a loop and check for result
	var stopped bool
	go func() {
		timer.Start()
		stopped = true
	}()
	time.Sleep(time.Duration(timer.IntervalSec*2) * time.Second)
	if a := timer.GetResults(); len(a) != len(timer.transientCommands)+len(timer.PreConfiguredCommands) {
		t.Fatal(a)
	}

	// Expect it to stop within 2 seconds
	timer.Stop()
	time.Sleep(2 * time.Second)
	if !stopped {
		t.Fatal("did not stop in time")
	}

	// Repeatedly stopping the loop should not matter
	timer.Stop()
	timer.Stop()
	timer.Stop()
}
