package smtpd

import (
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/email"
	"github.com/HouzuoGuo/laitos/env"
	"github.com/HouzuoGuo/laitos/frontend/common"
	"github.com/HouzuoGuo/laitos/frontend/smtpd/smtp"
	"github.com/HouzuoGuo/laitos/ratelimit"
	"log"
	"net"
	"time"
)

const RateLimitIntervalSec = 30 // Rate limit is calculated at 30 seconds interval

// An SMTP daemon that receives mails addressed to its domain name, and optionally forward the received mails to other addresses.
type SMTPD struct {
	ListenAddress string       `json:"ListenAddress"` // Network address to listen to, e.g. 0.0.0.0 for all network interfaces.
	ListenPort    int          `json:"ListenPort"`    // Port number to listen on
	TLSCertPath   string       `json:"TLSCertPath"`   // (Optional) serve StartTLS via this certificate
	TLSKeyPath    string       `json:"TLSCertKey"`    // (Optional) serve StartTLS via this certificte (key)
	IPLimit       int          `json:"IPLimit"`       // How many times in 30 seconds interval an IP may deliver an email to this server
	MyDomain      string       `json:"MyDomain"`      // Only receive mails addressed to this domain name
	ForwardTo     []string     `json:"ForwardTo"`     // Forward received mails to these addresses
	ForwardMailer email.Mailer `json:"ForwardMailer"` // Use this mailer to forward emails

	Processor      *common.CommandProcessor `json:"-"` // Feature command processor
	SMTPConfig     smtp.Config              `json:"-"` // SMTP processor configuration
	TLSCertificate tls.Certificate          `json:"-"` // TLS certificate read from the certificate and key files
	RateLimit      *ratelimit.RateLimit     `json:"-"` // Rate limit counter per IP address
}

// Check configuration and initialise internal states.
func (smtpd *SMTPD) Initialise() error {
	if errs := smtpd.Processor.IsSaneForInternet(); len(errs) > 0 {
		return fmt.Errorf("SMTPD.Initialise: %+v", errs)
	}
	if smtpd.ListenAddress == "" {
		return errors.New("SMTPD.Initialise: listen address is empty")
	}
	if smtpd.ListenPort == 0 {
		return errors.New("SMTPD.Initialise: listen port must not be empty or 0")
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
	greetingIP := env.GetPublicIP()
	if greetingIP == "" {
		greetingIP = "server" // just a dummy value
		log.Print("SMTPD.Initialise: unable to determine greeting hostname from public IP address, so use a dummy one.")
	}
	smtpd.SMTPConfig = smtp.Config{
		Limits: &smtp.Limits{
			MsgSize:   4 * 1048576,      // Accept mails up to 4 MB large
			IOTimeout: 60 * time.Second, // IO timeout is a reasonable minute
			BadCmds:   12,               // Abort connection after 12 consecutive bad commands
		},
		ServerName: greetingIP,
	}
	if smtpd.TLSCertPath != "" {
		smtpd.SMTPConfig.TLSConfig = &tls.Config{Certificates: []tls.Certificate{smtpd.TLSCertificate}}
	}
	smtpd.RateLimit = &ratelimit.RateLimit{
		MaxCount: smtpd.IPLimit,
		UnitSecs: RateLimitIntervalSec,
	}
	return nil
}

func (smtpd *SMTPD) ServeSMTP(clientConn net.Conn) {
	clientAddr := clientConn.RemoteAddr().String()
	log.Printf("SMTPD: handle %s", clientAddr)

	// Reject with 421 - server name ?
	//rateLimitExceeded := !smtpd.RateLimit.Add(clientAddr, true)
	/*
		var fromAddr, body, doneReason string
		toAddrs := make([]string, 0, 4)
		smtpConn := smtp.NewConn(clientConn, smtpd.SMTPConfig, nil)
		for {
			ev := smtpConn.Next()
			switch ev.What {
			case smtp.DONE:
				doneReason = "client finished normally"
				break
			case smtp.ABORT:
				doneReason = "client aborted"
				break
			case smtp.TLSERROR:
				doneReason = "client encountered TLS error"
				break
			case smtp.COMMAND:
				switch ev.Cmd {
				case smtp.MAILFROM:
					fromAddr = ev.Arg
				case smtp.RCPTTO:
					toAddrs = append(toAddrs, ev.Arg)
				}
			case smtp.GOTDATA:
				body = ev.Arg
			}
		}
		log.Printf("SMTPD: done with %s - %s", clientConn.RemoteAddr().String(), doneReason)*/
}

/*
You may call this function only after having called Initialise()!
Start SMTP daemon and block until this program exits.
*/
func (smtpd *SMTPD) StartAndBlock() error {
	log.Printf("SMTPD.StartAndBlock: will listen for connections on %s:%d", smtpd.ListenAddress, smtpd.ListenPort)
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", smtpd.ListenAddress, smtpd.ListenPort))
	if err != nil {
		return fmt.Errorf("SMTPD.StartAndBlock: failed to listen on %s:%d - %v", smtpd.ListenAddress, smtpd.ListenPort, err)
	}
	for {
		clientConn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("SMTPD.StartAndBlock: failed to accept new connection - %v", err)
		}
		go smtpd.ServeSMTP(clientConn)
	}
	return nil
}
