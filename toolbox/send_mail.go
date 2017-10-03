package toolbox

import (
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/inet"
	"net"
	"regexp"
	"strconv"
	"time"
)

var (
	// Captured into three groups, mail command looks like: address@domain.tld "this is email subject" this is email body
	RegexMailCommand    = regexp.MustCompile(`([a-zA-Z0-9!#$%&'*+-/=?_{|}~.^]+@[a-zA-Z0-9!#$%&'*+-/=?_{|}~.^]+.[a-zA-Z0-9!#$%&'*+-/=?_{|}~.^]+)\s*"(.*)"\s*(.*)`)
	ErrBadSendMailParam = errors.New(`Example: addr@dom.tld "subj" body`)
)

// Send outgoing emails.
type SendMail struct {
	Mailer inet.Mailer `json:"Mailer"`
}

var TestSendMail = SendMail{} // Details are set by init_feature_test.go

func (email *SendMail) IsConfigured() bool {
	return email.Mailer.IsConfigured()
}

func (email *SendMail) SelfTest() error {
	if !email.IsConfigured() {
		return ErrIncompleteConfig
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", email.Mailer.MTAHost, email.Mailer.MTAPort), TestTimeoutSec*time.Second)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

func (email *SendMail) Initialise() error {
	return nil
}

func (email *SendMail) Trigger() Trigger {
	return ".m"
}

func (email *SendMail) Execute(cmd Command) *Result {
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
	// Send email in background if it takes too long
	sendErrChan := make(chan error, 1)
	go func() {
		sendErrChan <- email.Mailer.Send(mailSubject, mailBody, mailTo)
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
