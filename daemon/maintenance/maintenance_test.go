package maintenance

import (
	"fmt"
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/inet"
	"strings"
	"testing"
)

func TestMaintenance_Execute(t *testing.T) {
	features := common.GetTestCommandProcessor().Features
	maint := Daemon{
		IntervalSec: 10,
		Mailer: inet.MailClient{
			MailFrom: "howard@localhost",
			MTAHost:  "localhost",
			MTAPort:  25,
		},
		Recipients:      []string{"howard@localhost"},
		FeaturesToCheck: features,
		MailpToCheck:    nil, // deliberately nil
	}

	if err := maint.Initialise(); !strings.Contains(err.Error(), "IntervalSec") {
		t.Fatal(err)
	}

	maint.IntervalSec = 3600
	TestMaintenance(&maint, t)
}

func TestSystemMaintenance(t *testing.T) {
	// Just make sure the function does not crash
	maint := Daemon{
		IntervalSec: 3600,
		Mailer: inet.MailClient{
			MailFrom: "howard@localhost",
			MTAHost:  "localhost",
			MTAPort:  25,
		},
		Recipients:      []string{"howard@localhost"},
		FeaturesToCheck: common.GetTestCommandProcessor().Features,
		MailpToCheck:    nil, // deliberately nil
	}
	ret := maint.SystemMaintenance()
	// Developer please manually inspect the output
	fmt.Println(ret)
}
