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
	// All of the following recipients are supposed to receive the SOS email
	expectedRecipients := map[string]bool{
		"aid@cad.gov.hk":                 false,
		"cnmcc@cttic.cn":                 false,
		"cnmrcc@mot.gov.cn":              false,
		"falmouthcoastguard@mcga.gov.uk": false,
		"hkmrcc@mardep.gov.hk":           false,
		"jrcchalifax@sarnet.dnd.ca":      false,
		"jrccpgr@yen.gr":                 false,
		"lantwatch@uscg.mil":             false,
		"mrcc@raja.fi":                   false,
		"mrcckorea@korea.kr":             false,
		"odsmrcc@morflot.ru":             false,
		"op@kaiho.mlit.go.jp":            false,
		"operations@jrcc-stavanger.no":   false,
		"rcc@mot.gov.il":                 false,
		// Be careful - this AU email is twice listed in SARContacts, it is shared by MSAR and ASAR.
		"rccaus@amsa.gov.au": false,
		"ukmcc@hmcg.gov.uk":  false,
	}
	wait := new(sync.WaitGroup)
	// +1 because amsa.gov.au gets two emails - one copy for MSAR and the other for ASAR.
	wait.Add(len(expectedRecipients) + 1)

	sendMail := SendMail{
		MailClient: inet.MailClient{
			MailFrom: "laitos software test case",
			MTAHost:  "example.com",
		},
		SOSPersonalInfo: "laitos software test from TestSendMail_SendSOS",
		sosTestCaseSendFun: func(subject string, body string, recipients ...string) error {
			sentSOSMailsMutex.Lock()
			defer sentSOSMailsMutex.Unlock()
			for _, recipient := range recipients {
				sentSOSMails[recipient] = sentSOSMail{
					subject:   subject,
					body:      body,
					recipient: recipient,
				}
				wait.Done()
			}
			return nil
		},
	}
	if err := sendMail.Initialise(); err != nil {
		t.Fatal(err)
	}

	result := sendMail.Execute(context.Background(), Command{TimeoutSec: 10, Content: `sos@sos "laitos software test email title" laitos software test email body`})
	if result.Error != nil || result.Output != "Sending SOS" || result.ErrText() != "" {
		t.Fatal(result.Error, result.Output, result.ErrText())
	}

	// Emails are sent in the background.
	wait.Wait()

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
