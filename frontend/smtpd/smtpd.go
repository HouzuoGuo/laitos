package smtpd

import (
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/email"
	"github.com/HouzuoGuo/laitos/env"
	"github.com/HouzuoGuo/laitos/frontend/mailp"
	"github.com/HouzuoGuo/laitos/frontend/smtpd/smtp"
	"github.com/HouzuoGuo/laitos/global"
	"github.com/HouzuoGuo/laitos/ratelimit"
	"net"
	netSMTP "net/smtp"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	RateLimitIntervalSec  = 10  // Rate limit is calculated at 10 seconds interval
	IOTimeoutSec          = 120 // IO timeout for both read and write operations
	MaxConversationLength = 64  // Only converse up to this number of messages in an SMTP connection
)

// An SMTP daemon that receives mails addressed to its domain name, and optionally forward the received mails to other addresses.
type SMTPD struct {
	ListenAddress string   `json:"ListenAddress"` // Network address to listen to, e.g. 0.0.0.0 for all network interfaces.
	ListenPort    int      `json:"ListenPort"`    // Port number to listen on
	TLSCertPath   string   `json:"TLSCertPath"`   // (Optional) serve StartTLS via this certificate
	TLSKeyPath    string   `json:"TLSKeyPath"`    // (Optional) serve StartTLS via this certificate (key)
	PerIPLimit    int      `json:"PerIPLimit"`    // How many times in 10 seconds interval an IP may deliver an email to this server
	MyDomains     []string `json:"MyDomains"`     // Only accept mails addressed to these domain names
	ForwardTo     []string `json:"ForwardTo"`     // Forward received mails to these addresses

	MyDomainsHash map[string]struct{} `json:"-"` // "MyDomains" values in map keys
	ForwardMailer email.Mailer        `json:"-"` // Use this mailer to forward arrived mails
	SMTPConfig    smtp.Config         `json:"-"` // SMTP processor configuration
	MyPublicIP    string              `json:"-"` // My public IP address as discovered by external services

	Listener       net.Listener    `json:"-"` // Once daemon is started, this is its TCP listener.
	TLSCertificate tls.Certificate `json:"-"` // TLS certificate read from the certificate and key files

	MailProcessor *mailp.MailProcessor `json:"-"` // Process feature commands from incoming mails
	RateLimit     *ratelimit.RateLimit `json:"-"` // Rate limit counter per IP address
	Logger        global.Logger        `json:"-"` // Logger
}

// Check configuration and initialise internal states.
func (smtpd *SMTPD) Initialise() error {
	smtpd.Logger = global.Logger{ComponentName: "SMTPD", ComponentID: fmt.Sprintf("%s:%d", smtpd.ListenAddress, smtpd.ListenPort)}
	if !smtpd.MailProcessor.ReplyMailer.IsConfigured() {
		return errors.New("SMTPD.Initialise: mail processor's reply mailer must be configured")
	}
	if smtpd.ListenAddress == "" {
		return errors.New("SMTPD.Initialise: listen address must not be empty")
	}
	if smtpd.ListenPort < 1 {
		return errors.New("SMTPD.Initialise: listen port must be greater than 0")
	}
	if smtpd.PerIPLimit < 1 {
		return errors.New("SMTPD.Initialise: PerIPLimit must be greater than 0")
	}
	if smtpd.ForwardTo == nil || len(smtpd.ForwardTo) == 0 || !smtpd.ForwardMailer.IsConfigured() {
		return errors.New("SMTPD.Initialise: the server is not useful if forward addresses/forward mailer are not configured")
	}
	if smtpd.MyDomains == nil || len(smtpd.MyDomains) == 0 {
		return errors.New("SMTPD.Initialise: my domain names must be configured")
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
	// Public IP is used to to greet SMTP client
	smtpd.MyPublicIP = env.GetPublicIP()
	if smtpd.MyPublicIP == "" {
		// Not a fatal error
		// Although "localhost" is not an IP address, it really is only used as a greeting banner, so it won't matter.
		smtpd.MyPublicIP = "localhost"
		smtpd.Logger.Warningf("Initialise", "", nil, "unable to determine public IP address, some SMTP conversations will be off-standard.")
	}
	smtpd.SMTPConfig = smtp.Config{
		Limits: &smtp.Limits{
			MsgSize:   2 * 1024 * 1024,            // Accept mails up to 2 MB large
			IOTimeout: IOTimeoutSec * time.Second, // IO timeout is a reasonable minute
			BadCmds:   64,                         // Abort connection after consecutive bad commands
		},
		ServerName: smtpd.MyPublicIP,
	}
	if smtpd.TLSCertPath != "" {
		smtpd.SMTPConfig.TLSConfig = &tls.Config{Certificates: []tls.Certificate{smtpd.TLSCertificate}}
	}
	smtpd.RateLimit = &ratelimit.RateLimit{
		MaxCount: smtpd.PerIPLimit,
		UnitSecs: RateLimitIntervalSec,
		Logger:   smtpd.Logger,
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
	// Construct a hash of MyDomains addresses for fast lookup
	smtpd.MyDomainsHash = map[string]struct{}{}
	for _, recv := range smtpd.MyDomains {
		smtpd.MyDomainsHash[recv] = struct{}{}
	}
	// Make sure that none of the forward addresses carries the domain name of MyDomains
	for _, fwd := range smtpd.ForwardTo {
		atSign := strings.IndexRune(fwd, '@')
		if atSign == -1 {
			return fmt.Errorf("SMTPD.Initialise: forward address \"%s\" must have an at sign", fwd)
		}
		if _, exists := smtpd.MyDomainsHash[fwd[atSign+1:]]; exists {
			return fmt.Errorf("SMTPD.Initialise: forward address \"%s\" must not loop back to this mail server's domain", fwd)
		}
	}
	return nil
}

// Unconditionally forward the mail to forward addresses, then process feature commands if they are found.
func (smtpd *SMTPD) ProcessMail(fromAddr, mailBody string) {
	bodyBytes := []byte(mailBody)
	// Forward the mail
	if err := smtpd.ForwardMailer.SendRaw(smtpd.ForwardMailer.MailFrom, bodyBytes, smtpd.ForwardTo...); err == nil {
		smtpd.Logger.Printf("ProcessMail", fromAddr, nil, "successfully forwarded mail to %v", smtpd.ForwardTo)
	} else {
		smtpd.Logger.Warningf("ProcessMail", fromAddr, err, "failed to forward email")
	}
	// Run feature command from mail body
	if err := smtpd.MailProcessor.Process(bodyBytes, smtpd.ForwardTo...); err != nil {
		smtpd.Logger.Warningf("ProcessMail", fromAddr, err, "failed to process feature command")
	}
}

// Converse with SMTP client to retrieve mail, then immediately process the retrieved mail. Finally close the connection.
func (smtpd *SMTPD) HandleConnection(clientConn net.Conn) {
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
				atSign := strings.IndexRune(ev.Arg, '@')
				if atSign == -1 {
					finishReason = "rejected for malformed address"
					smtpConn.Reject()
					goto conversationDone
				}
				if _, exists := smtpd.MyDomainsHash[ev.Arg[atSign+1:]]; !exists {
					finishReason = "rejected for wrong domain"
					smtpConn.Reject()
					goto conversationDone
				}
				toAddrs = append(toAddrs, ev.Arg)
			}
		case smtp.GOTDATA:
			mailBody = ev.Arg
		}
	}
conversationDone:
	if fromAddr == "" || len(toAddrs) == 0 {
		finishedNormally = false
		finishReason = "discarded malformed mail"
	}
	if finishedNormally {
		smtpd.Logger.Printf("HandleConnection", clientIP, nil, "got a mail from \"%s\" addressed to %v", fromAddr, toAddrs)
		// Forward the mail to forward-recipients, hence the original To-Addresses are not relevant.
		smtpd.ProcessMail(fromAddr, mailBody)
		smtpd.Logger.Printf("HandleConnection", clientIP, nil, "%s after %d conversations", finishReason, numConversations)
	} else {
		smtpd.Logger.Warningf("HandleConnection", clientIP, nil, "%s after %d conversations", finishReason, numConversations)
	}
}

/*
You may call this function only after having called Initialise()!
Start SMTP daemon and block until daemon is told to stop.
*/
func (smtpd *SMTPD) StartAndBlock() (err error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", smtpd.ListenAddress, smtpd.ListenPort))
	if err != nil {
		return fmt.Errorf("SMTPD.StartAndBlock: failed to listen on %s:%d - %v", smtpd.ListenAddress, smtpd.ListenPort, err)
	}
	defer listener.Close()
	smtpd.Listener = listener
	// Process incoming TCP connections
	smtpd.Logger.Printf("StartAndBlock", "", nil, "going to listen for connections")
	for {
		if global.EmergencyLockDown {
			return global.ErrEmergencyLockDown
		}
		clientConn, err := smtpd.Listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "closed") {
				return nil
			}
			return fmt.Errorf("SMTPD.StartAndBlock: failed to accept new connection - %v", err)
		}
		go smtpd.HandleConnection(clientConn)
	}
	return nil
}

// If SMTP daemon has started (i.e. listener is set), close the listener so that its connection loop will terminate.
func (smtpd *SMTPD) Stop() {
	if listener := smtpd.Listener; listener != nil {
		if err := listener.Close(); err != nil {
			smtpd.Logger.Warningf("Stop", "", err, "failed to close listener")
		}
	}
}

// Run unit tests on SMTPD. See TestSMTPD_StartAndBlock for daemon setup.
func TestSMTPD(smtpd *SMTPD, t *testing.T) {
	/*
		SMTP daemon is expected to start in a few seconds, it may take a short while because
		the daemon has to figure out its public IP address.
	*/
	var stoppedNormally bool
	go func() {
		if err := smtpd.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
		stoppedNormally = true
	}()
	addr := smtpd.ListenAddress + ":" + strconv.Itoa(smtpd.ListenPort)
	// This really should be env.HTTPPublicIPTimeout * time.Second, but that would be too long.
	time.Sleep(3 * time.Second)
	// Try to exceed rate limit
	testMessage := "Content-type: text/plain; charset=utf-8\r\nFrom: MsgFrom@whatever\r\nTo: MsgTo@whatever\r\nSubject: text subject\r\n\r\ntest body"
	success := 0
	for i := 0; i < 100; i++ {
		if err := netSMTP.SendMail(addr, nil, "ClientFrom@localhost", []string{"ClientTo@howard.name"}, []byte(testMessage)); err == nil {
			success++
		}
	}
	if success < 5 || success > 15 {
		t.Fatal("delivered", success)
	}
	// Wait till rate limit expires
	time.Sleep(RateLimitIntervalSec * time.Second)
	// Send an ordinary mail to the daemon
	testMessage = "Content-type: text/plain; charset=utf-8\r\nFrom: MsgFrom@whatever\r\nTo: MsgTo@whatever\r\nSubject: text subject\r\n\r\ntest body"
	if err := netSMTP.SendMail(addr, nil, "ClientFrom@localhost", []string{"ClientTo@example.com"}, []byte(testMessage)); err != nil {
		t.Fatal(err)
	}
	// Send a mail that does not belong to this server's domain
	testMessage = "Content-type: text/plain; charset=utf-8\r\nFrom: MsgFrom@whatever\r\nTo: MsgTo@whatever\r\nSubject: text subject\r\n\r\ntest body"
	if err := netSMTP.SendMail(addr, nil, "ClientFrom@localhost", []string{"ClientTo@not-my-domain"}, []byte(testMessage)); strings.Index(err.Error(), "Bad address") == -1 {
		t.Fatal(err)
	}
	// Try run a command via email
	testMessage = "Content-type: text/plain; charset=utf-8\r\nFrom: MsgFrom@whatever\r\nTo: MsgTo@whatever\r\nSubject: command subject\r\n\r\nverysecret.s echo hi"
	if err := netSMTP.SendMail(addr, nil, "ClientFrom@localhost", []string{"ClientTo@howard.name"}, []byte(testMessage)); err != nil {
		t.Fatal(err)
	}
	t.Log("Check howard@localhost and root@localhost mailbox")
	// Daemon must stop in a second
	smtpd.Stop()
	time.Sleep(1 * time.Second)
	if !stoppedNormally {
		t.Fatal("did not stop")
	}
	// Repeatedly stopping the daemon should have no negative consequence
	smtpd.Stop()
	smtpd.Stop()
}
