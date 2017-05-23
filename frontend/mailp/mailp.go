package mailp

import (
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/bridge"
	"github.com/HouzuoGuo/laitos/email"
	"github.com/HouzuoGuo/laitos/feature"
	"github.com/HouzuoGuo/laitos/frontend/common"
	"github.com/HouzuoGuo/laitos/global"
	"strconv"
	"strings"
	"sync"
)

/*
Look for feature commands from an incoming mail, run them and reply the sender with command results.
Usually used in combination of laitos' own SMTP daemon, but it can also work independently in something like
postfix's "forward-mail-to-program" mechanism.
*/
type MailProcessor struct {
	CommandTimeoutSec int                      `json:"CommandTimeoutSec"` // Commands get time out error after this number of seconds
	Undocumented1     Undocumented1            `json:"Undocumented1"`     // Intentionally undocumented he he he he
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
	wait.Add(2)
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
		err := mailproc.Undocumented1.SelfTest()
		if err != nil {
			retMutex.Lock()
			ret = append(ret, err)
			retMutex.Unlock()
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
	mailproc.Logger = global.Logger{ComponentName: "MailProcessor", ComponentID: strconv.Itoa(mailproc.CommandTimeoutSec)}
	mailproc.Processor.SetLogger(mailproc.Logger)
	if global.EmergencyLockDown {
		return global.ErrEmergencyLockDown
	}
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
		if mailproc.Undocumented1.IsConfigured() {
			// The undocumented scenario is triggered by an Email address suffix
			if mailproc.Undocumented1.Addr1 != "" && strings.HasSuffix(prop.ReplyAddress, mailproc.Undocumented1.Addr1) {
				// Let the undocumented scenario take care of delivering the result
				if undoc1Err := mailproc.Undocumented1.SendMessage(result.CombinedOutput); undoc1Err == nil {
					return false, nil
				} else {
					return false, undoc1Err
				}
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

var TestUndocumented1Message = ""                     // Content is set by init_mail_test.go
var TestUndocumented1Wolfram = feature.WolframAlpha{} // Details are set by init_mail_test.go
