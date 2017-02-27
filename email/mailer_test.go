package email

import (
	"net"
	"testing"
)

func TestMailer_Send(t *testing.T) {
	m := Mailer{}
	if m.IsConfigured() {
		t.Fatal("not correct")
	}
	m.MailFrom = "howard@localhost"
	// Hopefully nobody buys the domain name to mess with this test
	m.MTAHost = "waundnvbeuunixnfvncueiawnxzvkjdd.rich"
	m.MTAPort = 25
	if err := m.Send("test subject", "test body", m.MailFrom); err == nil {
		t.Fatal("did not error")
	}

	// Send a real email via real MTA
	if _, err := net.Dial("tcp", "localhost:25"); err == nil {
		m.MTAHost = "localhost"
		m.MTAPort = 25
		if err := m.Send("test subject", "test body", m.MailFrom); err != nil {
			t.Fatal(err)
		}
		t.Log("Check howard@localhost mail box")
	}
}
