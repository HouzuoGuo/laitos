package autounlock

import (
	"testing"
)

func TestDaemon_StartAndBlock(t *testing.T) {
	d := &Daemon{URLAndPassword: map[string]string{}, IntervalSec: 1}
	TestAutoUnlock(d, t)
}
