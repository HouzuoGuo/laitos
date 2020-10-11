package toolbox

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/inet"
)

func TestSendMail_Execute(t *testing.T) {
	if !TestSendMail.IsConfigured() {
		t.Skip("not configured")
	}
	if err := TestSendMail.Initialise(); err != nil {
		t.Fatal(err)
	}
	if err := TestSendMail.SelfTest(); err != nil {
		t.Fatal(err)
	}
	if ret := TestSendMail.Execute(context.Background(), Command{TimeoutSec: 10, Content: "wrong"}); ret.Error != ErrBadSendMailParam {
		t.Fatal(ret)
	}
	if ret := TestSendMail.Execute(context.Background(), Command{TimeoutSec: 10, Content: `guohouzuo@gmail.com "laitos send mail test" this is laitos send mail test`}); ret.Error != nil || ret.Output != "29" {
		t.Fatal(ret)
	}
}

type sentSOSMail struct {
	subject   string
	body      string
	recipient string
}

func TestSendMail_SendSOS(t *testing.T) {
	sentSOSMails := make(map[string]sentSOSMail)
	sentSOSMailsMutex := new(sync.Mutex)

	sendMail := SendMail{
		MailClient: inet.MailClient{
			MailFrom: "laitos software test case",
			MTAHost:  "example.com",
		},
		SOSPersonalInfo: "laitos software test from TestSendMail_SendSOS",
		sosTestCaseSendFun: func(subject string, body string, recipients ...string) error {
			sentSOSMailsMutex.Lock()
			for _, recipient := range recipients {
				sentSOSMails[recipient] = sentSOSMail{
					subject:   subject,
					body:      body,
					recipient: recipient,
				}
			}
			sentSOSMailsMutex.Unlock()
			return nil
		},
	}

	result := sendMail.Execute(context.Background(), Command{TimeoutSec: 10, Content: `sos@sos "laitos software test email title" laitos software test email body`})
	if result.Error != nil || result.Output != "Sending SOS" || result.ErrText() != "" {
		t.Fatal(result.Error, result.Output, result.ErrText())
	}

	// SOS emails are sent in background. Expect all of them to be sent within 5 seconds
	time.Sleep(5 * time.Second)

	// All of the following recipients are supposed to receive the SOS email
	expectedRecipients := map[string]bool{
		"lantwatch@uscg.mil":           false,
		"cnmcc@mail.eastnet.com.cn":    false,
		"aid@cad.gov.hk":               false,
		"hkmrcc@mardep.gov.hk":         false,
		"ukarcc@hmcg.gov.uk":           false,
		"ukmcc@hmcg.gov.uk":            false,
		"rccaus@amsa.gov.au":           false,
		"jrcchalifax@sarnet.dnd.ca":    false,
		"odsmrcc@morflot.ru":           false,
		"op@kaiho.mlit.go.jp":          false,
		"jrccpgr@yen.gr":               false,
		"mrcc@raja.fi":                 false,
		"rcc@mot.gov.il":               false,
		"cnmrcc@mot.gov.cn":            false,
		"operations@jrcc-stavanger.no": false,
		"mrcckorea@korea.kr":           false,
	}
	for _, mail := range sentSOSMails {
		// Check mail content
		if mail.subject != "SOS HELP laitos software test email title" {
			t.Fatal(mail)
		}
		expectedBody1 := fmt.Sprintf("SOS - send help immediately!\nComposed at %d", time.Now().Year())
		expectedBody2 := fmt.Sprintf(`by the operator of computer %s
Message: laitos software test email body
Additional info: laitos software test from TestSendMail_SendSOS`, inet.GetPublicIP())
		if !strings.Contains(mail.body, expectedBody1) || !strings.Contains(mail.body, expectedBody2) {
			t.Fatalf("\n%s\n%s\n%s\n", mail.body, expectedBody1, expectedBody2)
		}
		// Check mail recipient
		if _, exists := expectedRecipients[mail.recipient]; !exists {
			t.Fatal("unexpected recipient", mail.recipient)
		}
		expectedRecipients[mail.recipient] = true
	}
	for recipient, received := range expectedRecipients {
		if !received {
			t.Fatal("recipient did not receive", recipient)
		}
	}
}
