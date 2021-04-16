package mailcmd

import (
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/toolbox"
)

func TestMailProcessor_Process(t *testing.T) {
	runner := CommandRunner{
		Processor: &toolbox.CommandProcessor{},
		ReplyMailClient: inet.MailClient{
			MTAHost:  "127.0.0.1",
			MTAPort:  25,
			MailFrom: "howard@localhost",
		},
		Undocumented1: TestUndocumented1,
	}
	if err := runner.Initialise(); err == nil || !strings.Contains(err.Error(), "filters must be configured") {
		t.Fatal(err)
	}
	// CommandRunner has insane command processor
	runner.Processor = toolbox.GetInsaneCommandProcessor()
	if err := runner.Initialise(); err == nil || !strings.Contains(err.Error(), "password must be at least 7 characters long") {
		t.Fatal(err)
	}
	// Prepare a good processor
	runner.Processor = toolbox.GetTestCommandProcessor()
	TestCommandRunner(&runner, t)
}

func TestMailProcessor_SelfTest(t *testing.T) {
	runner := CommandRunner{
		Processor: &toolbox.CommandProcessor{},
		ReplyMailClient: inet.MailClient{
			MTAHost:  "127.0.0.1",
			MTAPort:  25,
			MailFrom: "howard@localhost",
		},
	}
	// Real MTA is required to run the self tests
	if _, err := net.Dial("tcp", "127.0.0.1:25"); err != nil {
		fmt.Println("skip the test due to no MTA running on 127.0.0.1")
		return
	}
	if err := runner.SelfTest(); err != nil {
		t.Fatal(err)
	}
}

func TestMailProcessor_Process_Undocumented1Reply(t *testing.T) {
	if TestUndocumented1Message == "" {
		t.Log("skip because TestUndocumented1Message is empty")
		return
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
	runner.Processor = toolbox.GetTestCommandProcessor()
	runner.Processor.Features.WolframAlpha = TestWolframAlpha
	runner.Processor.Features.LookupByTrigger[TestWolframAlpha.Trigger()] = &TestWolframAlpha
	if err := runner.Process("", []byte(TestUndocumented1Message)); err != nil {
		t.Fatal(err)
	}
	// Real MTA is required for the self test
	if _, err := net.Dial("tcp", "127.0.0.1:25"); err != nil {
		t.Skip("skip self test because there is no mta running on 127.0.0.1")
	}
	if err := runner.SelfTest(); err != nil {
		t.Fatal(err)
	}
}

func TestMailProcessor_Process_Undocumented2Reply(t *testing.T) {
	if TestUndocumented2Message == "" {
		t.Log("skip because TestUndocumented2Message is empty")
		return
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
	runner.Processor = toolbox.GetTestCommandProcessor()
	runner.Processor.Features.WolframAlpha = TestWolframAlpha
	runner.Processor.Features.LookupByTrigger[TestWolframAlpha.Trigger()] = &TestWolframAlpha
	if err := runner.Process("", []byte(TestUndocumented2Message)); err != nil {
		t.Fatal(err)
	}
	// Real MTA is required for the self test
	if _, err := net.Dial("tcp", "127.0.0.1:25"); err != nil {
		t.Log("skip self test because there is no mta running on 127.0.0.1")
		return
	}
	if err := runner.SelfTest(); err != nil {
		t.Fatal(err)
	}
}

func TestMailProcessor_Process_Undocumented3Reply(t *testing.T) {
	if TestUndocumented3Message == "" {
		t.Log("skip because TestUndocumented3Message is empty")
		return
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
	runner.Processor = toolbox.GetTestCommandProcessor()
	runner.Processor.Features.WolframAlpha = TestWolframAlpha
	runner.Processor.Features.LookupByTrigger[TestWolframAlpha.Trigger()] = &TestWolframAlpha
	if err := runner.Process("", []byte(TestUndocumented3Message)); err != nil {
		t.Fatal(err)
	}
	// Real MTA is required for the self test
	if _, err := net.Dial("tcp", "127.0.0.1:25"); err != nil {
		t.Skip("skip self test because there is no mta running on 127.0.0.1")
	}
	if err := runner.SelfTest(); err != nil {
		t.Fatal(err)
	}
}
