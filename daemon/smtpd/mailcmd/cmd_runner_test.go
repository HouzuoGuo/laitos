package mailcmd

import (
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/inet"
	"net"
	"strings"
	"testing"
)

func TestMailProcessor_Process(t *testing.T) {
	mailproc := CommandRunner{
		Processor:         &common.CommandProcessor{},
		CommandTimeoutSec: 5,
		ReplyMailer: inet.MailClient{
			MTAHost:  "127.0.0.1",
			MTAPort:  25,
			MailFrom: "howard@localhost",
		},
		Undocumented1: TestUndocumented1,
	}
	// CommandRunner has insane configuration
	if err := mailproc.Process([]byte("test body")); err == nil || !strings.Contains(err.Error(), common.ErrBadProcessorConfig) {
		t.Fatal("did not error due to insane CommandProcessor")
	}
	// Prepare a good processor
	mailproc.Processor = common.GetTestCommandProcessor()
	TestCommandRunner(&mailproc, t)
}

func TestMailProcessor_Process_Undocumented1Reply(t *testing.T) {
	if TestUndocumented1Message == "" {
		t.Skip()
	}
	mailproc := CommandRunner{
		CommandTimeoutSec: 5,
		ReplyMailer: inet.MailClient{
			MTAHost:  "127.0.0.1",
			MTAPort:  25,
			MailFrom: "howard@localhost",
		},
		Undocumented1: TestUndocumented1,
	}
	// Prepare a good processor
	mailproc.Processor = common.GetTestCommandProcessor()
	mailproc.Processor.Features.WolframAlpha = TestWolframAlpha
	mailproc.Processor.Features.LookupByTrigger[TestWolframAlpha.Trigger()] = &TestWolframAlpha
	if err := mailproc.Process([]byte(TestUndocumented1Message)); err != nil {
		t.Fatal(err)
	}
	// Real MTA is required for the self test
	if _, err := net.Dial("tcp", "127.0.0.1:25"); err != nil {
		t.Skip("there is no mta running on 127.0.0.1")
	}
	if err := mailproc.SelfTest(); err != nil {
		t.Fatal(err)
	}
}

func TestMailProcessor_Process_Undocumented2Reply(t *testing.T) {
	if TestUndocumented2Message == "" {
		t.Skip()
	}
	mailproc := CommandRunner{
		CommandTimeoutSec: 5,
		ReplyMailer: inet.MailClient{
			MTAHost:  "127.0.0.1",
			MTAPort:  25,
			MailFrom: "howard@localhost",
		},
		Undocumented2: TestUndocumented2,
	}
	// Prepare a good processor
	mailproc.Processor = common.GetTestCommandProcessor()
	mailproc.Processor.Features.WolframAlpha = TestWolframAlpha
	mailproc.Processor.Features.LookupByTrigger[TestWolframAlpha.Trigger()] = &TestWolframAlpha
	if err := mailproc.Process([]byte(TestUndocumented2Message)); err != nil {
		t.Fatal(err)
	}
	// Real MTA is required for the self test
	if _, err := net.Dial("tcp", "127.0.0.1:25"); err != nil {
		t.Skip("there is no mta running on 127.0.0.1")
	}
	if err := mailproc.SelfTest(); err != nil {
		t.Fatal(err)
	}
}
