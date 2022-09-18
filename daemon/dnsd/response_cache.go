package dnsd

import (
	"sync"
	"time"
)

// CacheEntry is a cached TCP-over-DNS query response.
type CacheEntry struct {
	content   []byte
	createdAt time.Time
}

// ResponseCache caches TCP-over-DNS query responses for a brief period of time.
type ResponseCache struct {
	expiry       time.Duration
	mutex        *sync.Mutex
	lastCleanUp  time.Time
	cleanUpEvery int
	counter      int
	cache        map[string]CacheEntry
}

// NewResponseCache constructs a new instance of ResponseCache and initialises
// its internal state.
func NewResponseCache(expiry time.Duration, cleanUpEvery int) *ResponseCache {
	return &ResponseCache{
		mutex:        new(sync.Mutex),
		lastCleanUp:  time.Now(),
		cache:        make(map[string]CacheEntry),
		expiry:       expiry,
		cleanUpEvery: cleanUpEvery,
	}
}

// cleanUP removes all expired entries. The caller must lock mutex.
func (cache *ResponseCache) cleanUp() {
	var expired []string
	for name, entry := range cache.cache {
		if time.Since(entry.createdAt) >= cache.expiry {
			expired = append(expired, name)
		}
	}
	for _, name := range expired {
		delete(cache.cache, name)
	}
}

// Get returns the cached name response, or it invokes setFun and to return and
// cache its response.
func (cache *ResponseCache) GetOrSet(name string, setFun func() []byte) []byte {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	if entry, exists := cache.cache[name]; exists && time.Since(entry.createdAt) < cache.expiry {
		return entry.content
	}
	newEntry := CacheEntry{
		content:   setFun(),
		createdAt: time.Now(),
	}
	cache.cache[name] = newEntry
	cache.counter++
	if cache.counter%cache.cleanUpEvery == 0 {
		cache.cleanUp()
	}
	return newEntry.content
}
