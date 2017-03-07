package mailp

import (
	"errors"
	"fmt"
	"github.com/HouzuoGuo/websh/bridge"
	"github.com/HouzuoGuo/websh/email"
	"github.com/HouzuoGuo/websh/feature"
	"github.com/HouzuoGuo/websh/frontend/common"
	"log"
	"strings"
)

// Process incoming mail and reply to it via the specified mailer.
type MailProcessor struct {
	Processor         *common.CommandProcessor `json:"-"`                 // Feature configuration
	CommandTimeoutSec int                      `json:"CommandTimeoutSec"` // Commands get time out error after this number of seconds
	ReplyMailer       email.Mailer             `json:"-"`                 // To deliver Email replies
}

/*
Make sure mail processor is sane before processing the incoming mail.
Process only one command (if found) in the incoming mail, figure out its reply address and send command result to
the reply address.
*/
func (mailproc *MailProcessor) Process(mailContent []byte) error {
	if errs := mailproc.Processor.IsSaneForInternet(); len(errs) > 0 {
		return fmt.Errorf("%+v", errs)
	}
	var commandIsProcessed bool
	walkErr := email.WalkMessage(mailContent, func(prop email.BasicProperties, body []byte) (bool, error) {
		// Avoid recursive processing
		if strings.Contains(prop.Subject, email.OutgoingMailSubjectKeyword) {
			return false, errors.New("Ignore email sent by this program itself")
		}
		log.Printf("MAILP: Handle %s - %s - %s", prop.FromAddress, prop.ContentType, prop.Subject)
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
		// Normally the result should be sent as Email reply, but there is an undocumented scenario.
		tmpUndoc1 := feature.Undocumented1{}
		undoc1, isConfigured := mailproc.Processor.Features.LookupByTrigger[tmpUndoc1.Trigger()]
		if isConfigured {
			undoc1T := undoc1.(*feature.Undocumented1)
			// The undocumented scenario is triggered by an Email address suffix
			if undoc1T.Addr1 != "" && strings.HasSuffix(prop.ReplyAddress, undoc1T.Addr1) {
				// Let the undocumented scenario take care of delivering the result
				undoc1Result := undoc1T.Execute(feature.Command{
					Content:    result.CombinedOutput,
					TimeoutSec: mailproc.CommandTimeoutSec,
				})
				if undoc1Result.Error == nil {
					return false, nil
				} else {
					return false, undoc1Result.Error
				}
			}
		}
		// The Email address suffix did not satisfy undocumented scenario, so send the result as a normal Email reply.
		if !mailproc.ReplyMailer.IsConfigured() {
			return false, errors.New("The reply has to be sent via Email but configuration is missing")
		}
		return false, mailproc.ReplyMailer.Send(email.OutgoingMailSubjectKeyword+"-reply-"+result.Command.Content, result.CombinedOutput, prop.ReplyAddress)
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

var TestUndocumented1Message = ""               // Content is set by init_mail_test.go
var TestUndocumented1 = feature.Undocumented1{} // Details are set by init_mail_test.go
