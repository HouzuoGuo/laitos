package healthcheck

import (
	"github.com/HouzuoGuo/laitos/email"
	"github.com/HouzuoGuo/laitos/frontend/common"
	"strings"
	"testing"
)

func TestHealthCheck_Execute(t *testing.T) {
	features := common.GetTestCommandProcessor().Features
	check := HealthCheck{
		IntervalSec: 10,
		Mailer: email.Mailer{
			MailFrom: "howard@localhost",
			MTAHost:  "localhost",
			MTAPort:  25,
		},
		Recipients:      []string{"howard@localhost"},
		FeaturesToCheck: features,
		MailpToCheck:    nil, // deliberately nil
	}

	if err := check.Initialise(); !strings.Contains(err.Error(), "IntervalSec") {
		t.Fatal(err)
	}

	check.IntervalSec = 3600
	TestHealthCheck(&check, t)
}
