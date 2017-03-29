package global

import "testing"

func TestTriggerEmergencyStop(t *testing.T) {
	if StartupTime.Year() < 2016 {
		t.Fatal("start time is wrong")
	}
	TriggerEmergencyStop()
	if !EmergencyStop {
		t.Fatal("did not trigger")
	}

}
