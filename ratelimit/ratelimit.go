package ratelimit

import (
	"log"
	"sync"
	"time"
)

// Allow an actor to perform no more than certain specified number of actions per unit of time.
type RateLimit struct {
	UnitSecs      int64
	MaxCount      int
	lastTimestamp int64
	counter       map[string]int
	logged        map[string]struct{}
	counterMutex  *sync.Mutex
}

// Initialise rate limiter internal states.
func (limit *RateLimit) Initialise() {
	limit.counter = make(map[string]int)
	limit.counterMutex = new(sync.Mutex)
	if limit.UnitSecs < 1 || limit.MaxCount < 1 {
		panic("RateLimit.Initialise: unit or max count must be greater than 0")
	}
}

// Increase counter of the actor by one. If the counter exceeds max limit, return false, otherwise return true.
func (limit *RateLimit) Add(actor string, logIfLimitHit bool) bool {
	limit.counterMutex.Lock()
	// Reset all counters if unit of time has past
	if now := time.Now().Unix(); now-limit.lastTimestamp >= limit.UnitSecs {
		limit.counter = make(map[string]int)
		limit.logged = make(map[string]struct{})
		limit.lastTimestamp = now
	}
	if count, exists := limit.counter[actor]; exists {
		if count >= limit.MaxCount {
			if _, hasLogged := limit.logged[actor]; !hasLogged && logIfLimitHit {
				log.Printf("RATELIMIT: %s exceeded limit of %d hits per %d seconds", actor, limit.MaxCount, limit.UnitSecs)
				limit.logged[actor] = struct{}{}
			}
			limit.counterMutex.Unlock()
			return false
		} else {
			limit.counter[actor] = count + 1
		}
	} else {
		limit.counter[actor] = 1
	}
	limit.counterMutex.Unlock()
	return true
}
