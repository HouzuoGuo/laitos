package mailp

import (
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/bridge"
	"github.com/HouzuoGuo/laitos/email"
	"github.com/HouzuoGuo/laitos/env"
	"github.com/HouzuoGuo/laitos/feature"
	"github.com/HouzuoGuo/laitos/frontend/common"
	"github.com/HouzuoGuo/laitos/global"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

var DurationStats = env.NewStats(0.01) // DurationStats stores statistics of duration of all processed mails.

/*
Look for feature commands from an incoming mail, run them and reply the sender with command results.
Usually used in combination of laitos' own SMTP daemon, but it can also work independently in something like
postfix's "forward-mail-to-program" mechanism.
*/
type MailProcessor struct {
	CommandTimeoutSec int                      `json:"CommandTimeoutSec"` // Commands get time out error after this number of seconds
	Undocumented1     Undocumented1            `json:"Undocumented1"`     // Intentionally undocumented he he he he
	Undocumented2     Undocumented2            `json:"Undocumented2"`     // Intentionally undocumented he he he he
	Processor         *common.CommandProcessor `json:"-"`                 // Feature configuration
	ReplyMailer       email.Mailer             `json:"-"`                 // To deliver Email replies
	Logger            global.Logger            `json:"-"`                 // Logger
}

// Run a health check on mailer and "undocumented" things.
func (mailproc *MailProcessor) SelfTest() error {
	ret := make([]error, 0, 0)
	retMutex := &sync.Mutex{}
	wait := &sync.WaitGroup{}
	// One mailer and one undocumented
	wait.Add(3)
	go func() {
		err := mailproc.ReplyMailer.SelfTest()
		if err != nil {
			retMutex.Lock()
			ret = append(ret, err)
			retMutex.Unlock()
		}
		wait.Done()
	}()
	go func() {
		if mailproc.Undocumented1.IsConfigured() {
			err := mailproc.Undocumented1.SelfTest()
			if err != nil {
				retMutex.Lock()
				ret = append(ret, err)
				retMutex.Unlock()
			}
		}
		wait.Done()
	}()
	go func() {
		if mailproc.Undocumented2.IsConfigured() {
			err := mailproc.Undocumented2.SelfTest()
			if err != nil {
				retMutex.Lock()
				ret = append(ret, err)
				retMutex.Unlock()
			}
		}
		wait.Done()
	}()
	wait.Wait()
	if len(ret) == 0 {
		return nil
	}
	return fmt.Errorf("%v", ret)
}

/*
Make sure mail processor is sane before processing the incoming mail.
Process only one command (if found) in the incoming mail. If reply addresses are specified, send command result
to the specified addresses. If they are not specified, use the incoming mail sender's address as reply address.
*/
func (mailproc *MailProcessor) Process(mailContent []byte, replyAddresses ...string) error {
	// Put query duration (including IO time) into statistics
	beginTimeNano := time.Now().UnixNano()
	defer DurationStats.Trigger(float64((time.Now().UnixNano() - beginTimeNano) / 1000000))
	if global.EmergencyLockDown {
		return global.ErrEmergencyLockDown
	}
	mailproc.Logger = global.Logger{ComponentName: "MailProcessor", ComponentID: strconv.Itoa(mailproc.CommandTimeoutSec)}
	if mailproc.Processor == nil {
		mailproc.Processor = common.GetEmptyCommandProcessor()
	}
	mailproc.Processor.SetLogger(mailproc.Logger)
	if errs := mailproc.Processor.IsSaneForInternet(); len(errs) > 0 {
		return fmt.Errorf("MailProcessor.Process: %+v", errs)
	}
	var commandIsProcessed bool
	walkErr := email.WalkMessage(mailContent, func(prop email.BasicProperties, body []byte) (bool, error) {
		// Avoid recursive processing
		if strings.Contains(prop.Subject, email.OutgoingMailSubjectKeyword) {
			return false, errors.New("Ignore email sent by this program itself")
		}
		mailproc.Logger.Printf("Process", prop.FromAddress, nil, "process message of type %s, subject \"%s\"", prop.ContentType, prop.Subject)
		// By contract, PIN processor finds command among input lines.
		result := mailproc.Processor.Process(feature.Command{
			Content:    string(body),
			TimeoutSec: mailproc.CommandTimeoutSec,
		})
		// If this part does not have a PIN/shortcut match, simply move on to the next part.
		if result.Error == bridge.ErrPINAndShortcutNotFound {
			// Move on, do not return error.
			return true, nil
		} else if result.Error != nil {
			// In case of command processing error, do not move on, return the error.
			return false, result.Error
		}
		// A command has been processed, now work on the reply.
		commandIsProcessed = true
		// Normally the result should be sent as Email reply, but there are undocumented scenarios.
		if mailproc.Undocumented1.MayReplyTo(prop) {
			if undoc1Err := mailproc.Undocumented1.SendMessage(result.CombinedOutput); undoc1Err == nil {
				return false, nil
			} else {
				return false, undoc1Err
			}
		}
		if mailproc.Undocumented2.MayReplyTo(prop) {
			if undoc1Err := mailproc.Undocumented2.SendMessage(result.CombinedOutput); undoc1Err == nil {
				return false, nil
			} else {
				return false, undoc1Err
			}
		}
		// The Email address suffix did not satisfy undocumented scenario, so send the result as a normal Email reply.
		if !mailproc.ReplyMailer.IsConfigured() {
			return false, errors.New("The reply has to be sent via Email but configuration is missing")
		}
		recipients := replyAddresses
		if recipients == nil || len(recipients) == 0 {
			recipients = []string{prop.ReplyAddress}
		}
		return false, mailproc.ReplyMailer.Send(email.OutgoingMailSubjectKeyword+"-reply-"+result.Command.Content, result.CombinedOutput, recipients...)
	})
	if walkErr != nil {
		return walkErr
	}
	// If all parts have been visited but no command is found, return the PIN mismatch error.
	if !commandIsProcessed {
		return bridge.ErrPINAndShortcutNotFound
	}
	return nil
}

var TestUndocumented1Message = ""             // Content is set by init_mail_test.go
var TestWolframAlpha = feature.WolframAlpha{} // Details are set by init_mail_test.go
var TestUndocumented2Message = ""             // Content is set by init_mail_test.go

// Run unit tests on mail processor. See TestMailProcessor_Process for processor setup.
func TestMailp(mailp *MailProcessor, t *testing.T) {
	// Real MTA is required to run the tests
	if _, err := net.Dial("tcp", "127.0.0.1:25"); err != nil {
		fmt.Println("there is no mta running on 127.0.0.1")
		return
	}
	if err := mailp.SelfTest(); err != nil {
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
	if err := mailp.Process([]byte(pinMismatch)); err != bridge.ErrPINAndShortcutNotFound {
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
	if err := mailp.Process([]byte(pinMatch)); err != nil {
		t.Fatal(err)
	}
	// PIN matches and override reply addr
	if err := mailp.Process([]byte(pinMatch), "root@localhost"); err != nil {
		t.Fatal(err)
	}
	t.Log("Check mail box of both root@localhost and howard@localhost")
}
