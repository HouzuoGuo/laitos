package mailcmd

import (
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/inet"
	"net"
	"strings"
	"testing"
)

func TestMailProcessor_Process(t *testing.T) {
	runner := CommandRunner{
		Processor: &common.CommandProcessor{},
		ReplyMailClient: inet.MailClient{
			MTAHost:  "127.0.0.1",
			MTAPort:  25,
			MailFrom: "howard@localhost",
		},
		Undocumented1: TestUndocumented1,
	}
	if err := runner.Initialise(); err == nil || strings.Index(err.Error(), "filters must be configured") == -1 {
		t.Fatal(err)
	}
	// CommandRunner has insane command processor
	runner.Processor = common.GetInsaneCommandProcessor()
	if err := runner.Initialise(); err == nil || strings.Index(err.Error(), "PIN is too short") == -1 {
		t.Fatal(err)
	}
	// Prepare a good processor
	runner.Processor = common.GetTestCommandProcessor()
	TestCommandRunner(&runner, t)
}

func TestMailProcessor_Process_Undocumented1Reply(t *testing.T) {
	if TestUndocumented1Message == "" {
		t.Skip()
	}
	runner := CommandRunner{
		ReplyMailClient: inet.MailClient{
			MTAHost:  "127.0.0.1",
			MTAPort:  25,
			MailFrom: "howard@localhost",
		},
		Undocumented1: TestUndocumented1,
	}
	// Prepare a good processor
	runner.Processor = common.GetTestCommandProcessor()
	runner.Processor.Features.WolframAlpha = TestWolframAlpha
	runner.Processor.Features.LookupByTrigger[TestWolframAlpha.Trigger()] = &TestWolframAlpha
	if err := runner.Process([]byte(TestUndocumented1Message)); err != nil {
		t.Fatal(err)
	}
	// Real MTA is required for the self test
	if _, err := net.Dial("tcp", "127.0.0.1:25"); err != nil {
		t.Skip("there is no mta running on 127.0.0.1")
	}
	if err := runner.SelfTest(); err != nil {
		t.Fatal(err)
	}
}

func TestMailProcessor_Process_Undocumented2Reply(t *testing.T) {
	if TestUndocumented2Message == "" {
		t.Skip()
	}
	runner := CommandRunner{
		ReplyMailClient: inet.MailClient{
			MTAHost:  "127.0.0.1",
			MTAPort:  25,
			MailFrom: "howard@localhost",
		},
		Undocumented2: TestUndocumented2,
	}
	// Prepare a good processor
	runner.Processor = common.GetTestCommandProcessor()
	runner.Processor.Features.WolframAlpha = TestWolframAlpha
	runner.Processor.Features.LookupByTrigger[TestWolframAlpha.Trigger()] = &TestWolframAlpha
	if err := runner.Process([]byte(TestUndocumented2Message)); err != nil {
		t.Fatal(err)
	}
	// Real MTA is required for the self test
	if _, err := net.Dial("tcp", "127.0.0.1:25"); err != nil {
		t.Skip("there is no mta running on 127.0.0.1")
	}
	if err := runner.SelfTest(); err != nil {
		t.Fatal(err)
	}
}

func TestMailProcessor_Process_Undocumented3Reply(t *testing.T) {
	if TestUndocumented3Message == "" {
		t.Skip()
	}
	runner := CommandRunner{
		ReplyMailClient: inet.MailClient{
			MTAHost:  "127.0.0.1",
			MTAPort:  25,
			MailFrom: "howard@localhost",
		},
		Undocumented3: TestUndocumented3,
	}
	// Prepare a good processor
	runner.Processor = common.GetTestCommandProcessor()
	runner.Processor.Features.WolframAlpha = TestWolframAlpha
	runner.Processor.Features.LookupByTrigger[TestWolframAlpha.Trigger()] = &TestWolframAlpha
	if err := runner.Process([]byte(TestUndocumented3Message)); err != nil {
		t.Fatal(err)
	}
	// Real MTA is required for the self test
	if _, err := net.Dial("tcp", "127.0.0.1:25"); err != nil {
		t.Skip("there is no mta running on 127.0.0.1")
	}
	if err := runner.SelfTest(); err != nil {
		t.Fatal(err)
	}
}
