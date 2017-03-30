package global

import "testing"

func TestTriggerEmergencyLockDown(t *testing.T) {
	if StartupTime.Year() < 2016 {
		t.Fatal("start time is wrong")
	}
	TriggerEmergencyLockDown()
	if !EmergencyLockDown {
		t.Fatal("did not trigger")
	}
}
