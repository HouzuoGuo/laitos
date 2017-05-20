package mailp

import (
	"github.com/HouzuoGuo/laitos/bridge"
	"github.com/HouzuoGuo/laitos/email"
	"github.com/HouzuoGuo/laitos/frontend/common"
	"net"
	"strings"
	"testing"
)

func TestMailProcessor_Process_MailReply(t *testing.T) {
	mailproc := MailProcessor{
		Processor:         &common.CommandProcessor{},
		CommandTimeoutSec: 5,
		ReplyMailer: email.Mailer{
			MTAHost:  "127.0.0.1",
			MTAPort:  25,
			MailFrom: "howard@localhost",
		},
		Undocumented1: TestUndocumented1,
	}
	// Processor has insane configuration
	if err := mailproc.Process([]byte("test body")); err == nil || !strings.Contains(err.Error(), common.ErrBadProcessorConfig) {
		t.Fatal("did not error due to insane CommandProcessor")
	}
	// Prepare a good processor
	mailproc.Processor = common.GetTestCommandProcessor()
	// Real MTA is required for the test from now on
	if _, err := net.Dial("tcp", "127.0.0.1:25"); err != nil {
		t.Skip()
	}
	if err := mailproc.SelfTest(); err != nil {
		t.Fatal(err)
	}
	// PIN mismatch
	pinMismatch := `From howard@localhost Sun Feb 26 18:17:34 2017
Return-Path: <howard@localhost>
X-Original-To: howard@localhost
Delivered-To: howard@localhost
Received: by localhost (Postfix, from userid 1000)
        id 542EA2421BD; Sun, 26 Feb 2017 18:17:34 +0100 (CET)
Date: Sun, 26 Feb 2017 18:17:34 +0100
To: howard@localhost
Subject: hi howard
User-Agent: Heirloom mailx 12.5 7/5/10
MIME-Version: 1.0
Content-Type: text/plain; charset=us-ascii
Content-Transfer-Encoding: 7bit
Message-Id: <20170226171734.542EA2421BD@localhost.>
From: howard@localhost (Howard Guo)
Status: R

PIN mismatch`
	if err := mailproc.Process([]byte(pinMismatch)); err != bridge.ErrPINAndShortcutNotFound {
		t.Fatal(err)
	}
	// PIN matches
	pinMatch := `From howard@localhost Sun Feb 26 18:17:34 2017
Return-Path: <howard@localhost>
X-Original-To: howard@localhost
Delivered-To: howard@localhost
Received: by localhost (Postfix, from userid 1000)
        id 542EA2421BD; Sun, 26 Feb 2017 18:17:34 +0100 (CET)
Date: Sun, 26 Feb 2017 18:17:34 +0100
To: howard@localhost
Subject: hi howard
User-Agent: Heirloom mailx 12.5 7/5/10
MIME-Version: 1.0
Content-Type: text/plain; charset=us-ascii
Content-Transfer-Encoding: 7bit
Message-Id: <20170226171734.542EA2421BD@localhost.>
From: howard@localhost (Howard Guo)
Status: R

PIN mismatch
verysecret.s echo hi
`
	if err := mailproc.Process([]byte(pinMatch)); err != nil {
		t.Fatal(err)
	}
	// PIN matches and override reply addr
	if err := mailproc.Process([]byte(pinMatch), "root@localhost"); err != nil {
		t.Fatal(err)
	}
	t.Log("Check mail box of both root@localhost and howard@localhost")
}

func TestMailProcessor_Process_Undocument1Reply(t *testing.T) {
	if TestUndocumented1Message == "" {
		t.Skip()
	}
	mailproc := MailProcessor{
		CommandTimeoutSec: 5,
		ReplyMailer: email.Mailer{
			MTAHost:  "127.0.0.1",
			MTAPort:  25,
			MailFrom: "howard@localhost",
		},
		Undocumented1: TestUndocumented1,
	}
	// Prepare a good processor
	mailproc.Processor = common.GetTestCommandProcessor()
	mailproc.Processor.Features.WolframAlpha = TestUndocumented1Wolfram
	mailproc.Processor.Features.LookupByTrigger[TestUndocumented1Wolfram.Trigger()] = &TestUndocumented1Wolfram
	// Real MTA is required for the test from now on
	if _, err := net.Dial("tcp", "127.0.0.1:25"); err != nil {
		t.Skip()
	}
	if err := mailproc.SelfTest(); err != nil {
		t.Fatal(err)
	}
	if err := mailproc.Process([]byte(TestUndocumented1Message)); err != nil {
		t.Fatal(err)
	}
}
