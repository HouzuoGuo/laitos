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
	emerLog.Printf("TriggerEmergencyLockDown", "", nil, "successfully triggered, most features will be disabled ASAP.")
	EmergencyLockDown = true
}

// Log a message and then immediately crash the entire program.
func TriggerEmergencyStop() {
	emerLog := Logger{
		ComponentID:   "Global",
		ComponentName: "EmergencyStop",
	}
	emerLog.Fatalf("TriggerEmergencyStop", "", nil, "successfully triggered, program now crashes.")
}
