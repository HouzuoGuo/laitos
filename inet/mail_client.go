package inet

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/HouzuoGuo/laitos/misc"
	"math/rand"
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

/*
dialMTA establishes a TCP connection to MTA and returns it. If the MTA port is not 25, the function will attempt to
establish a TLS connection; should a TLS failure occur, the ordinary TCP connection will be used.
*/
func dialMTA(host string, serverTLSName string, port int) (smtpClient *smtp.Client, tlsErr, err error) {
	// Establish an ordinary TCP connection
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), MailIOTimeoutSec*time.Second)
	if err != nil {
		return
	}
	// Try TLS on the connection
	tlsConn := tls.Client(conn, &tls.Config{ServerName: serverTLSName})
	if err = tlsConn.Handshake(); err == nil {
		// TLS is successful
		smtpClient, err = smtp.NewClient(tlsConn, host)
	} else {
		// TLS handshake failure occurred, the port likely does not use TLS, re-establish the TCP connection.
		tlsErr = err
		conn.Close()
		conn, err = net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), MailIOTimeoutSec*time.Second)
		if err != nil {
			return
		}
		smtpClient, err = smtp.NewClient(conn, host)
	}
	return
}

// sendMail connects to MTA, optionally presents client credentials for authentication, and then send a mail.
func sendMail(smtpClient *smtp.Client, serverTLSName string, auth smtp.Auth, from string, recipients []string, message []byte) error {
	defer smtpClient.Close()
	if canStartTLS, _ := smtpClient.Extension("STARTTLS"); canStartTLS {
		if err := smtpClient.StartTLS(&tls.Config{ServerName: serverTLSName}); err != nil {
			return err
		}
	}
	if auth != nil {
		if canAuth, _ := smtpClient.Extension("AUTH"); canAuth {
			if err := smtpClient.Auth(auth); err != nil {
				return err
			}
		}
	}
	if err := smtpClient.Mail(from); err != nil {
		return err
	}
	for _, recipient := range recipients {
		if err := smtpClient.Rcpt(recipient); err != nil {
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

var CommonMailLogger = misc.Logger{
	ComponentName: "MailClient",
	ComponentID:   "Common",
} // CommonMailLogger is shared by all mail clients to log mail delivery progress.

// Send emails via SMTP.
type MailClient struct {
	MailFrom     string `json:"MailFrom"`     // FROM address of the outgoing mails
	MTAHost      string `json:"MTAHost"`      // Server name or IP address of mail transportation agent
	MTAPort      int    `json:"MTAPort"`      // Port number of SMTP service on mail transportation agent
	AuthUsername string `json:"AuthUsername"` // (Optional) Username for plain authentication, if the SMTP server requires it.
	AuthPassword string `json:"AuthPassword"` // (Optional) Password for plain authentication, if the SMTP server requires it.
}

// Return true only if all mail parameters are present.
func (client *MailClient) IsConfigured() bool {
	return client.MailFrom != "" && client.MTAHost != "" && client.MTAPort != 0
}

/*
sendMailWithRetry collects addresses of the MTA host via DNS lookup, and tries to deliver the input mail using a
randomly selected MTA IP for up to 12 times within couple of days. The function blocks caller until it has exhausted
all delivery attempts.
*/
func (client *MailClient) sendMailWithRetry(auth smtp.Auth, from string, recipients []string, message []byte) {
	CommonMailLogger.Info("sendMailWithRetry", from, nil, "attempting to deliver mail to %v", recipients)
	// Retry mail delivery up to couple of days, introduce a random initial delay to avoid triggering MTA's rate limit.
	sleep := time.Duration(30+rand.Intn(30)) * time.Second
	for i := 0; i < 12; i++ {
		var smtpClient *smtp.Client
		var tlsErr error

		// Find the latest set of IP addresses belonging to the MTA
		timeout, _ := context.WithTimeout(context.Background(), MailIOTimeoutSec*time.Second)
		mtaIPs, err := net.DefaultResolver.LookupIPAddr(timeout, client.MTAHost)
		if err != nil {
			goto sleepAndRetry
		}
		// Try connecting to one of the MTA's IP addresses to deliver the mail
		smtpClient, tlsErr, err = dialMTA(mtaIPs[i%len(mtaIPs)].IP.String(), client.MTAHost, client.MTAPort)
		if err != nil {
			goto sleepAndRetry
		}
		if err = sendMail(smtpClient, client.MTAHost, auth, from, recipients, message); err != nil {
			smtpClient.Close()
			goto sleepAndRetry
		}
		// Success!
		CommonMailLogger.Info("sendMailWithRetry", from, nil, "successfully delivered mail to %v", recipients)
		smtpClient.Close()
		return
	sleepAndRetry:
		CommonMailLogger.Warning("sendMailWithRetry", from, err, "failed to deliver mail to %v in the latest attempt (tls error? %v)", recipients, tlsErr)
		time.Sleep(sleep)
		sleep = sleep * 2
	}
	CommonMailLogger.Warning("sendMailWithRetry", from, nil, "all attempts ultimately failed to deliver mail to %v", recipients)
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
	go client.sendMailWithRetry(auth, client.MailFrom, recipients, []byte(mailBody))
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
	go client.sendMailWithRetry(auth, client.MailFrom, recipients, rawMailBody)
	return nil
}

// Try to contact MTA and see if connection is possible.
func (client *MailClient) SelfTest() error {
	smtpClient, tlsErr, err := dialMTA(client.MTAHost, client.MTAHost, client.MTAPort)
	if err != nil {
		return fmt.Errorf("MailClient.SelfTest: connection test failed - %v (TLS error? %v)", err, tlsErr)
	}
	smtpClient.Close()
	return nil
}
