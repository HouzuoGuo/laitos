package phonehome

import (
	"strings"
	"testing"

	"github.com/HouzuoGuo/laitos/toolbox"
)

func TestPhoneHomeDaemon(t *testing.T) {
	daemon := Daemon{Processor: toolbox.GetInsaneCommandProcessor(), ReportIntervalSec: 1}
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), "at least one entry") {
		t.Fatal(err)
	}
	daemon.MessageProcessorServers = []*MessageProcessorServer{{Passwords: []string{"a"}, HTTPEndpointURL: "a"}}
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), toolbox.ErrBadProcessorConfig) {
		t.Fatal(err)
	}
	daemon.Processor = toolbox.GetTestCommandProcessor()
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	TestServer(&daemon, t)
}
