package global

import (
	"errors"
	"time"
)

var (
	StartupTime          = time.Now() // Timestamp when this program started
	EmergencyLockDown    bool         // As many features and daemons as possible should refuse to serve requests once this switches on
	ErrEmergencyLockDown = errors.New("Emergency system lock-down")
)

/*
Turn on EmergencyLockDown, so that as many features and daemons as possible will refuse to serve requests, yet the laitos
program keeps running.
There is no way to turn off Emergency Stop other than restarting laitos process.
*/
func TriggerEmergencyLockDown() {
	emerLog := Logger{
		ComponentID:   "Global",
		ComponentName: "EmergencyLockDown",
	}
	emerLog.Warningf("TriggerEmergencyLockDown", "", nil, "successfully triggered, most features will be disabled ASAP.")
	EmergencyLockDown = true
}

// Log a message and then crash the entire program after 30 seconds.
func TriggerEmergencyStop() {
	emerLog := Logger{
		ComponentID:   "Global",
		ComponentName: "EmergencyStop",
	}
	emerLog.Warningf("TriggerEmergencyStop", "", nil, "successfully triggered, program will crash in 30 seconds.")
	go func() {
		time.Sleep(30 * time.Second)
		emerLog.Fatalf("TriggerEmergencyStop", "", nil, "program crashes now")
	}()
}
