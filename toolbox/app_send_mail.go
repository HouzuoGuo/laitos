package toolbox

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
)

var (
	// RegexMailCommand captures mail command (address@domain.tld "this is email subject" this is email body) into three string groups.
	RegexMailCommand = regexp.MustCompile(`([a-zA-Z0-9!#$%&'*+-/=?_{|}~.^]+@[a-zA-Z0-9!#$%&'*+-/=?_{|}~.^]+.[a-zA-Z0-9!#$%&'*+-/=?_{|}~.^]+)\s*"(.*)"\s*(.*)`)
	/*
		SOSEmailRecipientMagic is the magic email recipient that corresponds to a built-in list of rescue coordinate
		centre Emails.
	*/
	SOSEmailRecipientMagic = "sos@sos"
	// ErrBadSendMailParam is the command execution result from an incomplete or malformed input command.
	ErrBadSendMailParam = errors.New(`example: addr@dom.tld "subj" body (send SOS to sos@sos)`)
)

// SendMail is a toolbox feature for sending ordinary and SOS emails.
type SendMail struct {
	MailClient      inet.MailClient `json:"MailClient"`      // MailClient offers mail transport configuration for this toolbox feature.
	SOSPersonalInfo string          `json:"SOSPersonalInfo"` // SOSPersonalInfo is a free-style human-readable text to be sent along with rest of SOS email body.

	logger             lalog.Logger
	sosTestCaseSendFun func(string, string, ...string) error // sosTestCaseSendFun is a substitue of email sending routine for SOS, to be used by test case.
}

var TestSendMail = SendMail{} // Details are set by init_feature_test.go

func (email *SendMail) IsConfigured() bool {
	return email.MailClient.IsConfigured()
}

func (email *SendMail) SelfTest() error {
	if !email.IsConfigured() {
		return ErrIncompleteConfig
	}
	if err := email.MailClient.SelfTest(); err != nil {
		return fmt.Errorf("SendMail.SelfTest: mail client error - %v", err)
	}
	return nil
}

func (email *SendMail) Initialise() error {
	email.logger = lalog.Logger{ComponentName: "sendmail", ComponentID: []lalog.LoggerIDField{{Key: "From", Value: email.MailClient.MailFrom}}}
	return nil
}

func (email *SendMail) Trigger() Trigger {
	return ".m"
}

func (email *SendMail) Execute(ctx context.Context, cmd Command) *Result {
	if errResult := cmd.Trim(); errResult != nil {
		return errResult
	}

	params := RegexMailCommand.FindStringSubmatch(cmd.Content)
	if len(params) != 4 {
		return &Result{Error: ErrBadSendMailParam}
	}
	mailTo := params[1]
	mailSubject := params[2]
	mailBody := params[3]
	if strings.TrimSpace(strings.ToLower(mailTo)) == SOSEmailRecipientMagic {
		// SOS emails are sent in background
		email.SendSOS(mailSubject, mailBody)
		return &Result{Output: "Sending SOS"}
	} else {
		// Wait for Email to be sent in foreground, but inform user if it takes too long.
		sendErrChan := make(chan error, 1)
		go func() {
			sendErrChan <- email.MailClient.Send(mailSubject, mailBody, mailTo)
		}()
		select {
		case <-time.After(time.Duration(cmd.TimeoutSec) * time.Second):
			return &Result{Output: "Sending in background"}
		case sendErr := <-sendErrChan:
			if sendErr == nil {
				// Normal result is the length of email body
				return &Result{Output: strconv.Itoa(len(mailBody))}
			} else {
				return &Result{Error: sendErr}
			}
		}
	}
}

// SendSOS delivers an SOS email to public search-and-rescue institutions.
func (email *SendMail) SendSOS(subject, body string) {
	// Prefix body text with some environment information
	body = fmt.Sprintf(`SOS - send help immediately!
Composed at %s by the operator of computer %s
Message: %s
Additional info: %s`,
		time.Now().UTC().Format(time.RFC3339), inet.GetPublicIP(),
		body,
		email.SOSPersonalInfo)

	email.logger.Warning(subject, nil, "sending SOS email, body is: %s", body)

	for _, recipient := range GetAllSAREmails() {
		if recipient == "" {
			continue
		}
		go func(recipient string) {
			var err error
			if email.sosTestCaseSendFun == nil {
				err = email.MailClient.Send("SOS HELP "+subject, body, recipient)
			} else {
				err = email.sosTestCaseSendFun("SOS HELP "+subject, body, recipient)
			}
			email.logger.Warning("", err, "attempted to deliver to %s", recipient)
		}(recipient)
	}
}
