package smtpd

import (
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/email"
	"github.com/HouzuoGuo/laitos/env"
	"github.com/HouzuoGuo/laitos/frontend/mailp"
	"github.com/HouzuoGuo/laitos/frontend/smtpd/smtp"
	"github.com/HouzuoGuo/laitos/ratelimit"
	"log"
	"net"
	"strings"
	"time"
)

const (
	RateLimitIntervalSec  = 10 // Rate limit is calculated at 10 seconds interval
	IOTimeoutSec          = 60 // IO timeout for both read and write operations
	MaxConversationLength = 64 // Only converse up to this number of messages in an SMTP connection
)

// An SMTP daemon that receives mails addressed to its domain name, and optionally forward the received mails to other addresses.
type SMTPD struct {
	ListenAddress string   `json:"ListenAddress"` // Network address to listen to, e.g. 0.0.0.0 for all network interfaces.
	ListenPort    int      `json:"ListenPort"`    // Port number to listen on
	TLSCertPath   string   `json:"TLSCertPath"`   // (Optional) serve StartTLS via this certificate
	TLSKeyPath    string   `json:"TLSCertKey"`    // (Optional) serve StartTLS via this certificte (key)
	PerIPLimit    int      `json:"PerIPLimit"`    // How many times in 10 seconds interval an IP may deliver an email to this server
	ForwardTo     []string `json:"ForwardTo"`     // Forward received mails to these addresses

	ForwardMailer email.Mailer `json:"-"` // Use this mailer to forward arrived mails
	SMTPConfig    smtp.Config  `json:"-"` // SMTP processor configuration
	MyPublicIP    string       `json:"-"` // My public IP address as discovered by external services

	Listener       net.Listener    `json:"-"` // Once daemon is started, this is its TCP listener.
	TLSCertificate tls.Certificate `json:"-"` // TLS certificate read from the certificate and key files

	MailProcessor *mailp.MailProcessor `json:"-"` // Process feature commands from incoming mails
	RateLimit     *ratelimit.RateLimit `json:"-"` // Rate limit counter per IP address
}

// Check configuration and initialise internal states.
func (smtpd *SMTPD) Initialise() error {
	if !smtpd.MailProcessor.ReplyMailer.IsConfigured() {
		return errors.New("SMTPD.Initialise: mail processor's reply mailer must be configured")
	}
	if smtpd.ListenAddress == "" {
		return errors.New("SMTPD.Initialise: listen address must not be empty")
	}
	if smtpd.ListenPort < 1 {
		return errors.New("SMTPD.Initialise: listen port must be greater than 0")
	}
	if smtpd.ForwardTo == nil || len(smtpd.ForwardTo) == 0 || !smtpd.ForwardMailer.IsConfigured() {
		return errors.New("SMTPD.Initialise: the server is not useful if forward addresses/forward mailer are not configured")
	}
	if smtpd.TLSCertPath != "" || smtpd.TLSKeyPath != "" {
		if smtpd.TLSCertPath == "" || smtpd.TLSKeyPath == "" {
			return errors.New("SMTPD.Initialise: if TLS is to be enabled, both TLS certificate and key path must be present.")
		}
		var err error
		smtpd.TLSCertificate, err = tls.LoadX509KeyPair(smtpd.TLSCertPath, smtpd.TLSKeyPath)
		if err != nil {
			return fmt.Errorf("SMTPD.Initialise: failed to read TLS certificate - %v", err)
		}
	}
	// Initialise SMTP processor configuration
	smtpd.MyPublicIP = env.GetPublicIP()
	if smtpd.MyPublicIP == "" {
		log.Print("SMTPD.Initialise: unable to determine public IP address")
	}
	smtpd.SMTPConfig = smtp.Config{
		Limits: &smtp.Limits{
			MsgSize:   1024 * 1024,                // Accept mails up to 1 MB large
			IOTimeout: IOTimeoutSec * time.Second, // IO timeout is a reasonable minute
			BadCmds:   32,                         // Abort connection after consecutive bad commands
		},
		ServerName: smtpd.MyPublicIP,
	}
	if smtpd.TLSCertPath != "" {
		smtpd.SMTPConfig.TLSConfig = &tls.Config{Certificates: []tls.Certificate{smtpd.TLSCertificate}}
	}
	smtpd.RateLimit = &ratelimit.RateLimit{
		MaxCount: smtpd.PerIPLimit,
		UnitSecs: RateLimitIntervalSec,
	}
	smtpd.RateLimit.Initialise()
	// Do not allow forward to this daemon itself
	if (strings.HasPrefix(smtpd.ForwardMailer.MTAHost, "127.") || smtpd.ForwardMailer.MTAHost == smtpd.MyPublicIP) &&
		smtpd.ForwardMailer.MTAPort == smtpd.ListenPort {
		return errors.New("SMTPD.Initialise: forward MTA must not be myself")
	}
	// Do not allow mail processor to reply to this daemon itself
	if (strings.HasPrefix(smtpd.MailProcessor.ReplyMailer.MTAHost, "127.") || smtpd.MailProcessor.ReplyMailer.MTAHost == smtpd.MyPublicIP) &&
		smtpd.MailProcessor.ReplyMailer.MTAPort == smtpd.ListenPort {
		return errors.New("SMTPD.Initialise: mail processor's reply MTA must not be myself")
	}
	return nil
}

// Unconditionally forward the mail to forward addresses, then process feature commands if they are found.
func (smtpd *SMTPD) ProcessMail(fromAddr, mailBody string) {
	bodyBytes := []byte(mailBody)
	// Forward the mail
	if err := smtpd.ForwardMailer.SendRaw(smtpd.ForwardMailer.MailFrom, bodyBytes, smtpd.ForwardTo...); err == nil {
		log.Printf("SMTPD: successfully forwarded email from %s to %v", fromAddr, smtpd.ForwardTo)
	} else {
		log.Printf("SMTPD: failed to forward mail from %s - %v", fromAddr, err)
	}
	// Run feature command from mail body
	if err := smtpd.MailProcessor.Process(bodyBytes, smtpd.ForwardTo...); err != nil {
		log.Printf("SMTPD: failed to process feature commands from %s - %v", fromAddr, err)
	}
}

// Converse with SMTP client to retrieve mail, then immediately process the retrieved mail. Finally close the connection.
func (smtpd *SMTPD) ServeSMTP(clientConn net.Conn) {
	defer clientConn.Close()
	clientIP := clientConn.RemoteAddr().String()[:strings.LastIndexByte(clientConn.RemoteAddr().String(), ':')]

	var numConversations int
	var finishedNormally bool
	var finishReason string
	// SMTP conversation will tell from/to addresses and mail mailBody
	var fromAddr, mailBody string
	toAddrs := make([]string, 0, 4)
	smtpConn := smtp.NewConn(clientConn, smtpd.SMTPConfig, nil)
	rateLimitOK := smtpd.RateLimit.Add(clientIP, true)
	for ; numConversations < MaxConversationLength; numConversations++ {
		ev := smtpConn.Next()
		// Politely reject the mail if rate is exceeded
		if !rateLimitOK {
			smtpConn.ReplyRateExceeded()
			return
		}
		log.Printf("SMTPD: handle %s", clientIP)
		// Converse with the client to retrieve mail
		switch ev.What {
		case smtp.DONE:
			finishReason = "finished normally"
			finishedNormally = true
			goto conversationDone
		case smtp.ABORT:
			finishReason = "aborted"
			goto conversationDone
		case smtp.TLSERROR:
			finishReason = "TLS error"
			goto conversationDone
		case smtp.COMMAND:
			switch ev.Cmd {
			case smtp.MAILFROM:
				fromAddr = ev.Arg
			case smtp.RCPTTO:
				toAddrs = append(toAddrs, ev.Arg)
			}
		case smtp.GOTDATA:
			mailBody = ev.Arg
		}
	}
conversationDone:
	if finishedNormally {
		log.Printf("SMTPD: got a mail from %s, composed by %s, addressed to %v", clientIP, fromAddr, toAddrs)
		// Forward the mail to forward-recipients, hence the original To-Addresses are not relevant.
		smtpd.ProcessMail(fromAddr, mailBody)
		log.Printf("SMTPD: done with %s (%s) in %d conversations", clientIP, finishReason, numConversations)
	}
}

/*
You may call this function only after having called Initialise()!
Start SMTP daemon and block until daemon is told to stop.
*/
func (smtpd *SMTPD) StartAndBlock() (err error) {
	log.Printf("SMTPD.StartAndBlock: will listen for connections on %s:%d", smtpd.ListenAddress, smtpd.ListenPort)
	smtpd.Listener, err = net.Listen("tcp", fmt.Sprintf("%s:%d", smtpd.ListenAddress, smtpd.ListenPort))
	if err != nil {
		return fmt.Errorf("SMTPD.StartAndBlock: failed to listen on %s:%d - %v", smtpd.ListenAddress, smtpd.ListenPort, err)
	}
	for {
		clientConn, err := smtpd.Listener.Accept()
		if err != nil {
			// Listener is told to stop
			if strings.Contains(err.Error(), "closed") {
				return nil
			} else {
				return fmt.Errorf("SMTPD.StartAndBlock: failed to accept new connection - %v", err)
			}
		}
		go smtpd.ServeSMTP(clientConn)
	}
	return nil
}

// If SMTP daemon has started (i.e. listener is set), close the listener so that its connection loop will terminate.
func (smtpd *SMTPD) Stop() {
	if smtpd.Listener != nil {
		if err := smtpd.Listener.Close(); err != nil {
			log.Printf("SMTPD: failed to close listener - %v", err)
		}
	}
}
