package dnsd

import (
	"context"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/toolbox"
)

/*
LatestCommands records the commands executed during the past TTL-period. The DNS server tracks these command execution
results to avoid repeatedly executing the same command for a recursive DNS server that uses a timeout too short.
*/
type LatestCommands struct {
	mutex        *sync.Mutex
	lastPurge    int64
	latestResult map[string]*toolbox.Result
}

// NewLatestCommands constructs a new instance of LatestCommands and initialises its internal state.
func NewLatestCommands() (rec *LatestCommands) {
	return &LatestCommands{
		mutex:        new(sync.Mutex),
		lastPurge:    0,
		latestResult: make(map[string]*toolbox.Result),
	}
}

// purgeAfterTTL removes all stored command records if a period of TTL has elapsed. Caller must lock the mutex.
func (rec *LatestCommands) purgeAfterTTL() {
	if time.Now().Unix()-rec.lastPurge > CommonResponseTTL {
		rec.lastPurge = time.Now().Unix()
		rec.latestResult = make(map[string]*toolbox.Result)
	}
}

/*
Execute looks for an ongoing or past execution of the command input. If an ongoing execution of the command is found,
then the function waits until result is ready from the ongoing execution and returns it; if the same command has been
executed recently, the function will return the past execution result; otherwise, the command execution begins right
away.
*/
func (rec *LatestCommands) Execute(ctx context.Context, cmdProcessor *toolbox.CommandProcessor, clientIP, cmdInput string) (result *toolbox.Result) {
	// Purge old result
	rec.purgeAfterTTL()
	// If execution of the command is ongoing, or has recently completed.
	if result, found := rec.get(cmdInput); found {
		// If execution of the command has recently started but not yet completed
		if result == nil {
			// Wait for its completion at 200ms interval
			for {
				result, found = rec.get(cmdInput)
				if !found {
					// Due to unfortunate timing, the result is evicted after a period of TTL, therefore re-run the command.
					goto execute
				}
				if result != nil {
					return result
				}
				time.Sleep(200 * time.Millisecond)
			}
		} else {
			// Return completed command result
			return result
		}
	}
execute:
	// Offer an indication that the command execution is ongoing but not yet completed
	rec.mutex.Lock()
	rec.latestResult[cmdInput] = nil
	rec.mutex.Unlock()
	// Execute the command and leave the lock available for another command that runs in parallel
	result = cmdProcessor.Process(ctx, toolbox.Command{
		ClientTag:  clientIP,
		DaemonName: "dnsd",
		TimeoutSec: CommonResponseTTL - 1,
		Content:    cmdInput,
	}, true)
	// After the command execution has completed, store the result into map for potential retrieval.
	rec.mutex.Lock()
	rec.latestResult[cmdInput] = result
	rec.mutex.Unlock()
	return
}

// get uses a mutex to guard against concurrent retrieval of past command execution result.
func (rec *LatestCommands) get(cmdInput string) (result *toolbox.Result, found bool) {
	rec.mutex.Lock()
	defer rec.mutex.Unlock()
	result, found = rec.latestResult[cmdInput]
	return
}
