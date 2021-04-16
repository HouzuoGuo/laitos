package mailcmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/testingstub"
	"github.com/HouzuoGuo/laitos/toolbox"
)

const (
	// CommandTimeoutSec is the timeout used for executing app commands found in mail.
	CommandTimeoutSec = 120

	// MaxMailSizeForAppCommand is the maximum size of an incoming mail that may carry an app command.
	// The command runner will not look for app command in a mail larger than this size
	MaxMailSizeForAppCommand = 16 * toolbox.MaxCmdLength
)

/*
CommandRunner looks for exactly one feature command from an incoming mail, runs it and reply the sender with command
results. Usually used in combination of laitos' own SMTP daemon, but it can also work independently with another MTA
such as the forwarding-mail-to-program mechanism from postfix.
*/
type CommandRunner struct {
	Undocumented1   Undocumented1             `json:"Undocumented1"` // Intentionally undocumented he he he he
	Undocumented2   Undocumented2             `json:"Undocumented2"` // Intentionally undocumented he he he he
	Undocumented3   Undocumented3             `json:"Undocumented3"` // Intentionally undocumented he he he he
	Processor       *toolbox.CommandProcessor `json:"-"`             // Feature configuration
	ReplyMailClient inet.MailClient           `json:"-"`             // To deliver Email replies
	logger          lalog.Logger

	// processTestCaseFunc works along side of command processing routine, it offers execution result to test case for inspection.
	processTestCaseFunc func(*toolbox.Result)
}

// Initialise initialises internal states of command runner. This function must be called before using the command runner.
func (runner *CommandRunner) Initialise() error {
	if runner.Processor == nil || runner.Processor.IsEmpty() {
		return fmt.Errorf("mailcmd.Initialise: command processor and its filters must be configured")
	}
	runner.logger = lalog.Logger{
		ComponentName: "mailcmd",
		ComponentID:   []lalog.LoggerIDField{{Key: "From", Value: runner.ReplyMailClient.MailFrom}},
	}
	runner.Processor.SetLogger(runner.logger)
	runner.Undocumented3.Logger = runner.logger
	runner.Undocumented3.MailClient = runner.ReplyMailClient
	if errs := runner.Processor.IsSaneForInternet(); len(errs) > 0 {
		return fmt.Errorf("mailcmd.Process: %+v", errs)
	}
	return nil
}

// Run a health check on mailer and "undocumented" things.
func (runner *CommandRunner) SelfTest() error {
	ret := make([]string, 0)
	retMutex := &sync.Mutex{}
	wait := &sync.WaitGroup{}
	// One mailer and 3 undocumented
	wait.Add(4)
	go func() {
		err := runner.ReplyMailClient.SelfTest()
		if err != nil {
			retMutex.Lock()
			ret = append(ret, err.Error())
			retMutex.Unlock()
		}
		wait.Done()
	}()
	go func() {
		if runner.Undocumented1.IsConfigured() {
			err := runner.Undocumented1.SelfTest()
			if err != nil {
				retMutex.Lock()
				ret = append(ret, err.Error())
				retMutex.Unlock()
			}
		}
		wait.Done()
	}()
	go func() {
		if runner.Undocumented2.IsConfigured() {
			err := runner.Undocumented2.SelfTest()
			if err != nil {
				retMutex.Lock()
				ret = append(ret, err.Error())
				retMutex.Unlock()
			}
		}
		wait.Done()
	}()
	go func() {
		if runner.Undocumented3.IsConfigured() {
			err := runner.Undocumented3.SelfTest()
			if err != nil {
				retMutex.Lock()
				ret = append(ret, err.Error())
				retMutex.Unlock()
			}
		}
		wait.Done()
	}()
	wait.Wait()
	if len(ret) == 0 {
		return nil
	}
	return errors.New(strings.Join(ret, " | "))
}

/*
Make sure mail processor is sane before processing the incoming mail.
Process only one command (if found) in the incoming mail. If reply addresses are specified, send command result
to the specified addresses. If they are not specified, use the incoming mail sender's address as reply address.
*/
func (runner *CommandRunner) Process(clientIP string, mailContent []byte, replyAddresses ...string) error {
	if misc.EmergencyLockDown {
		return misc.ErrEmergencyLockDown
	}
	var commandIsProcessed bool

	walkErr := inet.WalkMailMessage(mailContent, func(prop inet.BasicMail, body []byte) (bool, error) {
		// Avoid recursive processing
		if strings.Contains(prop.Subject, inet.OutgoingMailSubjectKeyword) {
			return false, errors.New("ignore email sent by this program itself")
		}
		runner.logger.Info("Process", prop.FromAddress, nil, "process message subject \"%s\", content type \"%s\"", prop.Subject, prop.ContentType)
		if partSize := len(body); partSize > MaxMailSizeForAppCommand {
			runner.logger.Info("Process", prop.FromAddress, nil, "the mail part size (%d) is too large to be carrying an app command", partSize)
			return false, nil
		}
		// By contract, PIN processor finds command among input lines.
		result := runner.Processor.Process(context.TODO(), toolbox.Command{
			DaemonName: "smtpd",
			ClientTag:  clientIP,
			Content:    string(body),
			TimeoutSec: CommandTimeoutSec,
		}, true)
		// Offer execution result to test case for inspection
		if runner.processTestCaseFunc != nil {
			runner.processTestCaseFunc(result)
		}
		// If this part does not have a PIN/shortcut match, simply move on to the next part.
		if result.Error == toolbox.ErrPINAndShortcutNotFound {
			// Move on, do not return error.
			return true, nil
		} else if result.Error != nil {
			// In case of command processing error, do not move on, return the error.
			return false, result.Error
		}
		// A command has been processed, now work on the reply.
		commandIsProcessed = true
		// Normally the result should be sent as Email reply, but there are undocumented scenarios.
		if runner.Undocumented1.MayReplyTo(prop) {
			runner.logger.Info("Process", prop.FromAddress, nil, "invoke Undocumented1")
			if undocErr := runner.Undocumented1.SendMessage(result.CombinedOutput); undocErr == nil {
				return false, nil
			} else {
				return false, undocErr
			}
		}
		if runner.Undocumented2.MayReplyTo(prop) {
			runner.logger.Info("Process", prop.FromAddress, nil, "invoke Undocumented2")
			if undocErr := runner.Undocumented2.SendMessage(result.CombinedOutput); undocErr == nil {
				return false, nil
			} else {
				return false, undocErr
			}
		}
		if runner.Undocumented3.MayReplyTo(prop) {
			runner.logger.Info("Process", prop.FromAddress, nil, "invoke Undocumented3")
			if undocErr := runner.Undocumented3.SendMessage(prop, result.CombinedOutput); undocErr == nil {
				return false, nil
			} else {
				return false, undocErr
			}
		}
		// The Email address suffix did not satisfy undocumented scenario, so send the result as a normal Email reply.
		if !runner.ReplyMailClient.IsConfigured() {
			return false, errors.New("the reply has to be sent via Email but configuration is missing")
		}
		recipients := replyAddresses
		if len(recipients) == 0 {
			recipients = []string{prop.ReplyAddress}
		}
		return false, runner.ReplyMailClient.Send(inet.OutgoingMailSubjectKeyword+"-reply-"+result.Command.Content, result.CombinedOutput, recipients...)
	})
	if walkErr != nil {
		return fmt.Errorf("failed to process mail body - %w", walkErr)
	}
	// If all parts have been visited but no command is found, return the PIN mismatch error.
	if !commandIsProcessed {
		return toolbox.ErrPINAndShortcutNotFound
	}
	return nil
}

var (
	// The content of the following variables are set by init_mailcmd_test.go

	TestUndocumented1Message = ""
	TestWolframAlpha         = toolbox.WolframAlpha{}
	TestUndocumented2Message = ""
	TestUndocumented3Message = ""
)

// Run unit tests on mail processor. See TestMailProcessor_Process for processor setup.
func TestCommandRunner(runner *CommandRunner, t testingstub.T) {
	var lastResult *toolbox.Result
	runner.processTestCaseFunc = func(result *toolbox.Result) {
		lastResult = result
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
	lastResult = nil
	if err := runner.Process("", []byte(pinMismatch)); err != toolbox.ErrPINAndShortcutNotFound {
		t.Fatal(err)
	} else if lastResult == nil || lastResult.Error != toolbox.ErrPINAndShortcutNotFound {
		t.Fatalf("should not have executed any command %+v", lastResult)
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
	lastResult = nil
	if err := runner.Process("", []byte(pinMatch)); err != nil {
		t.Fatal(err)
	} else if lastResult == nil || lastResult.Error != nil || strings.TrimSpace(lastResult.CombinedOutput) != "hi" {
		t.Fatalf("%+v", lastResult)
	}
	// PIN matches and override reply addr
	lastResult = nil
	if err := runner.Process("", []byte(pinMatch), "root@localhost"); err != nil {
		t.Fatal(err)
	} else if lastResult == nil || lastResult.Error != nil || strings.TrimSpace(lastResult.CombinedOutput) != "hi" {
		t.Fatalf("%+v", lastResult)
	}

	trimAppCommandBeforeUse := "From:a@example.com\nTo: b@example.com\nSubject: test\r\n\r\n verysecret.s echo success"
	lastResult = nil
	if err := runner.Process("", []byte(trimAppCommandBeforeUse)); err != nil {
		t.Fatal(err)
	} else if lastResult == nil || lastResult.Error != nil || strings.TrimSpace(lastResult.CombinedOutput) != "success" {
		t.Fatalf("%+v", lastResult)
	}

}
