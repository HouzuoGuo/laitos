package inet

import (
	"net"
	"testing"
)

func TestMailer_Send(t *testing.T) {
	m := MailClient{}
	if m.IsConfigured() {
		t.Fatal("not correct")
	}
	m.MailFrom = "howard@localhost"
	// Hopefully nobody buys the domain name to mess with this test
	m.MTAHost = "waundnvbeuunixnfvncueiawnxzvkjdd.rich"
	m.MTAPort = 25
	if err := m.Send("laitos mailer test subject", "test body", m.MailFrom); err == nil {
		t.Fatal("did not error")
	}
	if err := m.SelfTest(); err == nil {
		t.Fatal("did not error")
	}

	// Send a real email via real MTA
	if _, err := net.Dial("tcp", "localhost:25"); err == nil {
		m.MTAHost = "localhost"
		m.MTAPort = 25
		if err := m.Send("laitos mailer test subject", "test body", m.MailFrom); err != nil {
			t.Fatal(err)
		}
		rawBody := "From: FromAddr@localhost\r\nTo: ToAddr@localhost\r\nSubject: laitos mailer test raw subject\r\n\r\nrawBody"
		if err := m.SendRaw("howard@localhost", []byte(rawBody), "howard@localhost"); err != nil {
			t.Fatal(err)
		}
		t.Log("Check howard@localhost mail box")
		if err := m.SelfTest(); err != nil {
			t.Fatal(err)
		}
	}
}
