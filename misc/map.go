package misc

import "sync"

// LenSyncMap returns the number of elements stored in sync.Map.
func LenSyncMap(m *sync.Map) int {
	count := 0
	m.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}
