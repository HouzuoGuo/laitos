package misc

import (
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
)

/*
RateLimit tracks number of hits performed by each source ("actor") to determine whether a source has exceeded
specified rate limit. Instead of being a rolling counter, the tracking data is reset to empty at regular interval.
Remember to call Initialise() before use!
*/
type RateLimit struct {
	UnitSecs int64
	MaxCount int
	Logger   lalog.Logger

	lastTimestamp int64
	counter       map[string]int
	logged        map[string]struct{}
	counterMutex  *sync.Mutex
}

// Initialise rate limiter internal states.
func (limit *RateLimit) Initialise() {
	limit.lastTimestamp = 0
	limit.counter = make(map[string]int)
	limit.logged = make(map[string]struct{})
	limit.counterMutex = new(sync.Mutex)
	if limit.UnitSecs < 1 || limit.MaxCount < 1 {
		limit.Logger.Panic("Initialise", "RateLimit", nil, "UnitSecs and MaxCount must be greater than 0")
		return
	}
	// Turn per-second limit into greater limit over multiple seconds to reduce log spamming
	if limit.UnitSecs == 1 {
		for _, factor := range []int{11, 7, 5, 3, 2} {
			if limit.MaxCount%factor == 0 {
				limit.UnitSecs = int64(factor)
				limit.MaxCount *= factor
				break
			}
		}
	}
}

/*
Add increases the current counter by one for the actor name/ID if the max count per time interval has not been exceeded, and returns true.
Otherwise, the actor's current counter stays until the interval passes, and the function will return false.
*/
func (limit *RateLimit) Add(actor string, logIfLimitHit bool) bool {
	limit.counterMutex.Lock()
	defer limit.counterMutex.Unlock()
	// Reset all counters after the interval.
	if now := time.Now().Unix(); now-limit.lastTimestamp >= limit.UnitSecs {
		limit.counter = make(map[string]int)
		limit.logged = make(map[string]struct{})
		limit.lastTimestamp = now
	}
	if count, exists := limit.counter[actor]; exists {
		if count >= limit.MaxCount {
			if _, hasLogged := limit.logged[actor]; !hasLogged && logIfLimitHit {
				limit.Logger.Info("Add", "RateLimit", nil, "%s exceeded limit of %d hits per %d seconds", actor, limit.MaxCount, limit.UnitSecs)
				limit.logged[actor] = struct{}{}
			}
			return false
		} else {
			limit.counter[actor] = count + 1
		}
	} else {
		limit.counter[actor] = 1
	}
	return true
}
