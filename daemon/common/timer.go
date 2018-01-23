package common

import (
	"fmt"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/toolbox"
	"strconv"
	"sync"
	"time"
)

const (
	TimerCommandTimeoutSec = 10 // TimerCommandTimeoutSec is a hard coded timeout number constraining all commands run by timer.
)

/*
CommandTimer executes series of commands, one at a time, at regular interval. Execution results of recent commands are
memorised and can be retrieved at a later time. Beyond command execution results, arbitrary text messages may also be
memorised and retrieved together with command results. CommandTimer is a useful structure for implementing notification
kind of mechanism.
*/
type CommandTimer struct {
	// PreConfiguredCommands are toolbox commands pre-configured to run by user, they never deleted upon clearing.
	PreConfiguredCommands []string `json:"PreConfiguredCommands"`
	// IntervalSec is the number of seconds to sleep between execution of all commands.
	IntervalSec int `json:"IntervalSec"`
	// MaxResults is the maximum number of results to memorise from command execution and text messages.
	MaxResults int `json:"MaxResults"`
	// CommandProcessor is the one going to run all commands.
	CommandProcessor *CommandProcessor `json:"-"`

	/*
		transientCommands are new commands that are added on the fly and can be cleared by calling a function.
		During trigger, these commands are executed after the pre-configured commands.
	*/
	transientCommands []string
	results           *misc.RingBuffer // results are the most recent command results and test messages to retrieve.
	mutex             sync.Mutex       // mutex prevents concurrent access to internal structures.
	running           bool             // running becomes true when command processing loop is running
	stop              chan struct{}    // stop channel signals Run function to return soon.
}

// Initialise prepares internal states of a new CommandTimer.
func (timer *CommandTimer) Initialise() error {
	if timer.IntervalSec < 1 {
		return fmt.Errorf("CommandTimer.Initialise: IntervalSec must be greater than 0")
	}
	if timer.MaxResults < 1 {
		return fmt.Errorf("CommandTimer.Initialise: MaxResults must be greater than 0")
	}
	if timer.PreConfiguredCommands == nil {
		timer.PreConfiguredCommands = []string{}
	}
	timer.results = misc.NewRingBuffer(int64(timer.MaxResults))
	timer.transientCommands = make([]string, 0, 10)
	timer.stop = make(chan struct{})
	return nil
}

/*
GetTransientCommands returns a copy of all transient commands memorises for execution. If there is none, it returns
an empty string array.
*/
func (timer *CommandTimer) GetTransientCommands() []string {
	timer.mutex.Lock()
	defer timer.mutex.Unlock()
	ret := make([]string, len(timer.transientCommands))
	copy(ret, timer.transientCommands)
	return ret
}

// AddTransientCommand places a new toolbox command toward the end of transient command list.
func (timer *CommandTimer) AddTransientCommand(cmd string) {
	timer.mutex.Lock()
	defer timer.mutex.Unlock()
	timer.transientCommands = append(timer.transientCommands, cmd)
}

// ClearTransientCommands removes all transient commands.
func (timer *CommandTimer) ClearTransientCommands() {
	timer.mutex.Lock()
	defer timer.mutex.Unlock()
	timer.transientCommands = make([]string, 0, 10)
}

// runAllCommands executes all pre-configured and transient commands one after another and store their results.
func (timer *CommandTimer) runAllCommands() {
	//	Access to the commands array is not protected by mutex since no other function modifies it
	if timer.PreConfiguredCommands != nil {
		for _, cmd := range timer.PreConfiguredCommands {
			timer.results.Push(timer.CommandProcessor.Process(toolbox.Command{
				TimeoutSec: TimerCommandTimeoutSec,
				Content:    cmd,
			}).CombinedOutput)
		}
	}
	// Make a copy of the latest transient commands to run
	timer.mutex.Lock()
	transientCommands := make([]string, len(timer.transientCommands))
	copy(transientCommands, timer.transientCommands)
	timer.mutex.Unlock()
	// Run transient commands one after another
	for _, cmd := range transientCommands {
		timer.results.Push(timer.CommandProcessor.Process(toolbox.Command{
			TimeoutSec: TimerCommandTimeoutSec,
			Content:    cmd,
		}).CombinedOutput)
	}
}

/*
Start runs an infinite loop to execute all commands one after another, then sleep for an interval.
The function blocks caller until Stop function is called.
If Start function is already running, calling it a second time will do nothing and return immediately.
*/
func (timer *CommandTimer) Start() {
	timer.mutex.Lock()
	if timer.running {
		misc.DefaultLogger.Warning("CommandTimer.Start", strconv.Itoa(timer.IntervalSec), nil, "starting an already started CommandTimer becomes a nop")
		timer.mutex.Unlock()
		return
	}
	timer.mutex.Unlock()
	misc.DefaultLogger.Info("CommandTimer.Start", strconv.Itoa(timer.IntervalSec), nil, "timer has started")
	for {
		timer.running = true
		select {
		case <-time.After(time.Duration(timer.IntervalSec) * time.Second):
			timer.runAllCommands()
		case <-timer.stop:
			return
		}
	}
}

/*
Stop informs the running command processing loop to terminate as early as possible. Blocks until the loop has
terminated. Calling the function while command processing loop is not running yields no effect.
*/
func (timer *CommandTimer) Stop() {
	timer.mutex.Lock()
	if timer.running {
		timer.stop <- struct{}{}
		timer.running = false
	}
	timer.mutex.Unlock()
}

// AddArbitraryTextToResult simply places an arbitrary text string into result.
func (timer *CommandTimer) AddArbitraryTextToResult(text string) {
	// RingBuffer supports concurrent push access, there is no need to protect it with timer's own mutex.
	timer.results.Push(text)
}

// GetResults returns the latest command execution results and text messages, then clears the result buffer.
func (timer *CommandTimer) GetResults() []string {
	timer.mutex.Lock()
	defer timer.mutex.Unlock()
	ret := timer.results.GetAll()
	timer.results.Clear()
	return ret
}
