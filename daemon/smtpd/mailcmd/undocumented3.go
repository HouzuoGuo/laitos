package mailcmd

import (
	"errors"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/toolbox"
)

// (IM) Intentionally undocumented he he he.
type Undocumented3 struct {
	MailAddrSuffix string          `json:"MailAddrSuffix"`
	MailClient     inet.MailClient `json:"-"`
	Logger         lalog.Logger    `json:"-"`
}

var TestUndocumented3 = Undocumented3{}  // Details are set by init_mail_test.go
var TestUndocumented3Mail inet.BasicMail // Details are set by init_mail_test.go

func (und *Undocumented3) IsConfigured() bool {
	return und.MailAddrSuffix != ""
}

func (und *Undocumented3) SelfTest() error {
	if !und.IsConfigured() {
		return toolbox.ErrIncompleteConfig
	}
	return nil
}

func (und *Undocumented3) MayReplyTo(prop inet.BasicMail) bool {
	return und.IsConfigured() && und.MailAddrSuffix != "" && strings.HasSuffix(prop.ReplyAddress, und.MailAddrSuffix)
}

func (und *Undocumented3) SendMessage(prop inet.BasicMail, message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return errors.New("Undocumented3.SendMessage: message is empty")
	}
	if len(message) > 134 {
		message = message[:134]
	}
	und.Logger.Info("Undocumented3.SendMessage", prop.FromAddress, nil, "will send reply to: %s", prop.ReplyAddress)
	return und.MailClient.Send(time.Now().String(), message, prop.ReplyAddress)
}
