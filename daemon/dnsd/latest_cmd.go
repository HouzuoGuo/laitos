package dnsd

import (
	"github.com/HouzuoGuo/laitos/toolbox"
	"sync"
	"time"
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
	if time.Now().Unix()-rec.lastPurge > TextCommandReplyTTL {
		rec.lastPurge = time.Now().Unix()
		rec.latestResult = make(map[string]*toolbox.Result)
	}
}

// StoreResult stores a new command execution record for potential retrieval before the next TTL refresh.
func (rec *LatestCommands) StoreResult(cmdInput string, result *toolbox.Result) {
	rec.mutex.Lock()
	defer rec.mutex.Unlock()
	rec.purgeAfterTTL()
	rec.latestResult[cmdInput] = result
}

// Get returns a previous command execution result corresponding to the command input, or nil if there is none.
func (rec *LatestCommands) Get(cmdInput string) *toolbox.Result {
	rec.mutex.Lock()
	defer rec.mutex.Unlock()
	rec.purgeAfterTTL()
	return rec.latestResult[cmdInput]
}
