package global

import (
	"errors"
	"time"
)

var (
	StartupTime      = time.Now() // Timestamp when this program started
	EmergencyStop    bool         // As many features and daemons as possible should refuse to serve requests once this switches on
	ErrEmergencyStop = errors.New("Emergency system lock-down")
)

/*
Turn on Emergency Stop, so that as many features and daemons as possible will refuse to serve requests, yet the laitos
program keeps running.
There is no way to turn off Emergency Stop other than restarting laitos process.
*/
func TriggerEmergencyStop() {
	EmergencyStop = true
}
