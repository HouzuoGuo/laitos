package inet

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

const (
	OutgoingMailSubjectKeyword = "laitos" // Outgoing emails are encouraged to carry this string in their subject
	MailIOTimeoutSec           = 10       // MailIOTimeoutSec is the timeout for contacting MTA
)

// Send emails via SMTP.
type MailClient struct {
	MailFrom       string `json:"MailFrom"`     // FROM address of the outgoing mails
	MTAHost        string `json:"MTAHost"`      // Server name or IP address of mail transportation agent
	MTAPort        int    `json:"MTAPort"`      // Port number of SMTP service on mail transportation agent
	AuthUsername   string `json:"AuthUsername"` // (Optional) Username for plain authentication, if the SMTP server requires it.
	AuthPassword   string `json:"AuthPassword"` // (Optional) Password for plain authentication, if the SMTP server requires it.
	lastTLSFailure error  // lastTLSFailure is the TLS handshake failure resulted from very latest connection attempt
}

// Return true only if all mail parameters are present.
func (client *MailClient) IsConfigured() bool {
	return client.MailFrom != "" && client.MTAHost != "" && client.MTAPort != 0
}

/*
dialMTA establishes a TCP connection to MTA and returns it. If the MTA port is not 25, the function will attempt to
establish a TLS connection; should a TLS failure occur, the ordinary TCP connection will be used.
*/
func (client *MailClient) dialMTA() (smtpClient *smtp.Client, err error) {
	client.lastTLSFailure = nil
	// Establish an ordinary TCP connection
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(client.MTAHost, strconv.Itoa(client.MTAPort)), MailIOTimeoutSec*time.Second)
	if err != nil {
		return
	}
	// Try TLS on the connection
	tlsConn := tls.Client(conn, &tls.Config{ServerName: client.MTAHost})
	if err = tlsConn.Handshake(); err == nil {
		// TLS is successful
		smtpClient, err = smtp.NewClient(tlsConn, client.MTAHost)
	} else {
		// TLS handshake failure occured, the port likely does not use TLS, re-establish the TCP connection.
		client.lastTLSFailure = err
		conn.Close()
		conn, err = net.DialTimeout("tcp", net.JoinHostPort(client.MTAHost, strconv.Itoa(client.MTAPort)), MailIOTimeoutSec*time.Second)
		if err != nil {
			return
		}
		smtpClient, err = smtp.NewClient(conn, client.MTAHost)
	}
	return
}

// sendMail connects to MTA, optionally presents client credentials for authentication, and then send a mail.
func (client *MailClient) sendMail(auth smtp.Auth, from string, recipients []string, message []byte) error {
	smtpClient, err := client.dialMTA()
	if err != nil {
		return err
	}
	defer smtpClient.Close()
	if canStartTLS, _ := smtpClient.Extension("STARTTLS"); canStartTLS {
		if err = smtpClient.StartTLS(&tls.Config{ServerName: client.MTAHost}); err != nil {
			return err
		}
	}
	if auth != nil {
		if canAuth, _ := smtpClient.Extension("AUTH"); canAuth {
			if err = smtpClient.Auth(auth); err != nil {
				return err
			}
		}
	}
	if err = smtpClient.Mail(from); err != nil {
		return err
	}
	for _, recipient := range recipients {
		if err = smtpClient.Rcpt(recipient); err != nil {
			return err
		}
	}
	smtpData, err := smtpClient.Data()
	if err != nil {
		return err
	}
	if _, err := smtpData.Write(message); err != nil {
		return err
	}
	if err := smtpData.Close(); err != nil {
		return err
	}
	return smtpClient.Quit()
}

// Deliver mail to all recipients. Block until mail is sent or an error has occurred.
func (client *MailClient) Send(subject string, textBody string, recipients ...string) error {
	if recipients == nil || len(recipients) == 0 {
		return fmt.Errorf("no recipient specified for mail \"%s\"", subject)
	}
	var auth smtp.Auth
	if client.AuthUsername != "" {
		auth = smtp.PlainAuth("", client.AuthUsername, client.AuthPassword, client.MTAHost)
	}
	// Construct appropriate mail headers
	mailBody := fmt.Sprintf("MIME-Version: 1.0\r\nContent-type: text/plain; charset=utf-8\r\nFrom: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		client.MailFrom, strings.Join(recipients, ", "), subject, textBody)
	if err := client.sendMail(auth, client.MailFrom, recipients, []byte(mailBody)); err != nil {
		return fmt.Errorf("MailClient.Send: error - %v (TLS error? %v)", err, client.lastTLSFailure)
	}
	return nil
}

// Deliver unmodified mail body to all recipients. Block until mail is sent or an error has occurred.
func (client *MailClient) SendRaw(fromAddr string, rawMailBody []byte, recipients ...string) error {
	if recipients == nil || len(recipients) == 0 {
		return fmt.Errorf("no recipient specified for mail from \"%s\"", fromAddr)
	}
	var auth smtp.Auth
	if client.AuthUsername != "" {
		auth = smtp.PlainAuth("", client.AuthUsername, client.AuthPassword, client.MTAHost)
	}
	if err := client.sendMail(auth, fromAddr, recipients, rawMailBody); err != nil {
		return fmt.Errorf("MailClient.SendRaw: error - %v (TLS error? %v)", err, client.lastTLSFailure)
	}
	return nil
}

// Try to contact MTA and see if connection is possible.
func (client *MailClient) SelfTest() error {
	smtpClient, err := client.dialMTA()
	if err != nil {
		return fmt.Errorf("MailClient.SelfTest: connection test failed - %v (TLS error? %v)", err, client.lastTLSFailure)
	}
	smtpClient.Close()
	return nil
}
