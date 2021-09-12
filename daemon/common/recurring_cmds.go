package common

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/datastruct"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/toolbox"
)

const (
	TimerCommandTimeoutSec = 10 // TimerCommandTimeoutSec is a hard coded timeout number constraining all commands run by timer.
)

/*
RecurringCommands executes series of commands, one at a time, at regular interval. Execution results of recent commands are
memorised and can be retrieved at a later time. Beyond command execution results, arbitrary text messages may also be
memorised and retrieved together with command results. RecurringCommands is a useful structure for implementing notification
kind of mechanism.
*/
type RecurringCommands struct {
	// PreConfiguredCommands are toolbox commands pre-configured to run by user, they never deleted upon clearing.
	PreConfiguredCommands []string `json:"PreConfiguredCommands"`
	// IntervalSec is the number of seconds to sleep between execution of all commands.
	IntervalSec int `json:"IntervalSec"`
	// MaxResults is the maximum number of results to memorise from command execution and text messages.
	MaxResults int `json:"MaxResults"`
	// CommandProcessor is the one going to run all commands.
	CommandProcessor *toolbox.CommandProcessor `json:"-"`

	/*
		transientCommands are new commands that are added on the fly and can be cleared by calling a function.
		During trigger, these commands are executed after the pre-configured commands.
	*/
	transientCommands []string
	results           *datastruct.RingBuffer // results are the most recent command results and test messages to retrieve.
	mutex             sync.Mutex             // mutex prevents concurrent access to internal structures.
	logger            lalog.Logger
	cancelFunc        func()
}

// Initialise prepares internal states of a new RecurringCommands.
func (cmds *RecurringCommands) Initialise() error {
	if cmds.IntervalSec < 1 {
		return fmt.Errorf("RecurringCommands.Initialise: IntervalSec must be greater than 0")
	}
	if cmds.MaxResults < 1 {
		return fmt.Errorf("RecurringCommands.Initialise: MaxResults must be greater than 0")
	}
	if cmds.PreConfiguredCommands == nil {
		cmds.PreConfiguredCommands = []string{}
	}
	cmds.results = datastruct.NewRingBuffer(int64(cmds.MaxResults))
	cmds.transientCommands = make([]string, 0, 10)
	cmds.logger = lalog.Logger{
		ComponentName: "RecurringCommands",
		ComponentID:   []lalog.LoggerIDField{{Key: "Intv", Value: cmds.IntervalSec}},
	}
	return nil
}

/*
GetTransientCommands returns a copy of all transient commands memorises for execution. If there is none, it returns
an empty string array.
*/
func (cmds *RecurringCommands) GetTransientCommands() []string {
	cmds.mutex.Lock()
	defer cmds.mutex.Unlock()
	ret := make([]string, len(cmds.transientCommands))
	copy(ret, cmds.transientCommands)
	return ret
}

// AddTransientCommand places a new toolbox command toward the end of transient command list.
func (cmds *RecurringCommands) AddTransientCommand(cmd string) {
	cmds.mutex.Lock()
	defer cmds.mutex.Unlock()
	cmds.transientCommands = append(cmds.transientCommands, cmd)
}

// ClearTransientCommands removes all transient commands.
func (cmds *RecurringCommands) ClearTransientCommands() {
	cmds.mutex.Lock()
	defer cmds.mutex.Unlock()
	cmds.transientCommands = make([]string, 0, 10)
}

// runAllCommands executes all pre-configured and transient commands one after another and store their results.
func (cmds *RecurringCommands) runAllCommands(ctx context.Context) {
	//	Access to the commands array is not protected by mutex since no other function modifies it
	if cmds.PreConfiguredCommands != nil {
		for _, cmd := range cmds.PreConfiguredCommands {
			// Skip result filters that may send notifications or manipulate result in other means
			result := cmds.CommandProcessor.Process(ctx, toolbox.Command{
				DaemonName: "RecurringCommands",
				TimeoutSec: TimerCommandTimeoutSec,
				Content:    cmd,
			}, false)
			cmds.mutex.Lock()
			cmds.results.Push(result.CombinedOutput)
			cmds.mutex.Unlock()
		}
	}
	// Make a copy of the latest transient commands that are about to run
	cmds.mutex.Lock()
	transientCommands := make([]string, len(cmds.transientCommands))
	copy(transientCommands, cmds.transientCommands)
	cmds.mutex.Unlock()
	// Run transient commands one after another
	for _, cmd := range transientCommands {
		// Skip result filters that may send notifications or manipulate result in other means
		result := cmds.CommandProcessor.Process(ctx, toolbox.Command{
			DaemonName: "RecurringCommands",
			TimeoutSec: TimerCommandTimeoutSec,
			Content:    cmd,
		}, false)
		cmds.mutex.Lock()
		cmds.results.Push(result.CombinedOutput)
		cmds.mutex.Unlock()
	}

}

/*
Start runs an infinite loop to execute all commands one after another, then sleep for an interval.
The function blocks caller until Stop function is called.
If Start function is already running, calling it a second time will do nothing and return immediately.
*/
func (cmds *RecurringCommands) Start() {
	cmds.mutex.Lock()
	defer cmds.mutex.Unlock()
	if cmds.cancelFunc != nil {
		cmds.logger.Warning("RecurringCommands.Start", fmt.Sprintf("Intv=%d", cmds.IntervalSec), nil, "starting an already started RecurringCommands becomes a nop")
		return
	}
	cmds.logger.Info("RecurringCommands.Start", fmt.Sprintf("Intv=%d", cmds.IntervalSec), nil, "command execution now starts")
	ctx, cancelFunc := context.WithCancel(context.Background())
	cmds.cancelFunc = cancelFunc
	periodicFunc := func(ctx context.Context, _, _ int) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			cmds.runAllCommands(ctx)
		}
		return nil
	}
	periodic := &misc.Periodic{
		LogActorName: cmds.logger.ComponentName,
		Interval:     time.Duration(cmds.IntervalSec) * time.Second,
		MaxInt:       1,
		Func:         periodicFunc,
	}
	_ = periodic.Start(ctx)
}

/*
Stop informs the running command processing loop to terminate as early as possible. Blocks until the loop has
terminated. Calling the function while command processing loop is not running yields no effect.
*/
func (cmds *RecurringCommands) Stop() {
	cmds.mutex.Lock()
	defer cmds.mutex.Unlock()
	if cmds.cancelFunc != nil {
		cmds.cancelFunc()
		cmds.cancelFunc = nil
	}
	cmds.logger.Info("Stop", "", nil, "stopped on request")
}

// AddArbitraryTextToResult simply places an arbitrary text string into result.
func (cmds *RecurringCommands) AddArbitraryTextToResult(text string) {
	// RingBuffer supports concurrent push access, there is no need to protect it with timer's own mutex.
	cmds.results.Push(text)
}

// GetResults returns the latest command execution results and text messages, then clears the result buffer.
func (cmds *RecurringCommands) GetResults() []string {
	cmds.mutex.Lock()
	defer cmds.mutex.Unlock()
	ret := cmds.results.GetAll()
	cmds.results.Clear()
	return ret
}
