package smtpd

import (
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/daemon/smtpd/mailcmd"
	"github.com/HouzuoGuo/laitos/daemon/smtpd/smtp"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/testingstub"
	"net"
	netSMTP "net/smtp"
	"strconv"
	"strings"
	"time"
)

const (
	RateLimitIntervalSec  = 10  // Rate limit is calculated at 10 seconds interval
	IOTimeoutSec          = 120 // IO timeout for both read and write operations
	MaxConversationLength = 64  // Only converse up to this number of messages in an SMTP connection
)

var DurationStats = misc.NewStats() // DurationStats stores statistics of duration of all SMTP conversations.

// An SMTP daemon that receives mails addressed to its domain name, and optionally forward the received mails to other addresses.
type Daemon struct {
	Address     string   `json:"Address"`     // Network address to listen to, e.g. 0.0.0.0 for all network interfaces.
	Port        int      `json:"Port"`        // Port number to listen on
	TLSCertPath string   `json:"TLSCertPath"` // (Optional) serve StartTLS via this certificate
	TLSKeyPath  string   `json:"TLSKeyPath"`  // (Optional) serve StartTLS via this certificate (key)
	PerIPLimit  int      `json:"PerIPLimit"`  // How many times in 10 seconds interval an IP may deliver an email to this server
	MyDomains   []string `json:"MyDomains"`   // Only accept mails addressed to these domain names
	ForwardTo   []string `json:"ForwardTo"`   // Forward received mails to these addresses

	MyDomainsHash     map[string]struct{} `json:"-"` // "MyDomains" values in map keys
	ForwardMailClient inet.MailClient     `json:"-"` // ForwardMailClient is used to forward arriving emails.
	SMTPConfig        smtp.Config         `json:"-"` // SMTP processor configuration

	Listener       net.Listener    `json:"-"` // Once daemon is started, this is its TCP listener.
	TLSCertificate tls.Certificate `json:"-"` // TLS certificate read from the certificate and key files

	CommandRunner *mailcmd.CommandRunner `json:"-"` // Process feature commands from incoming mails
	RateLimit     *misc.RateLimit        `json:"-"` // Rate limit counter per IP address
	Logger        misc.Logger            `json:"-"` // Logger
}

// Check configuration and initialise internal states.
func (daemon *Daemon) Initialise() error {
	daemon.Logger = misc.Logger{ComponentName: "smtpd", ComponentID: fmt.Sprintf("%s:%d", daemon.Address, daemon.Port)}
	if daemon.Address == "" {
		return errors.New("smtpd.Initialise: listen address must not be empty")
	}
	if daemon.Port < 1 {
		return errors.New("smtpd.Initialise: listen port must be greater than 0")
	}
	if daemon.PerIPLimit < 1 {
		return errors.New("smtpd.Initialise: PerIPLimit must be greater than 0")
	}
	if daemon.ForwardTo == nil || len(daemon.ForwardTo) == 0 || !daemon.ForwardMailClient.IsConfigured() {
		return errors.New("smtpd.Initialise: the server is not useful if forward addresses/forward mail client are not configured")
	}
	if daemon.MyDomains == nil || len(daemon.MyDomains) == 0 {
		return errors.New("smtpd.Initialise: my domain names must be configured")
	}
	if daemon.TLSCertPath != "" || daemon.TLSKeyPath != "" {
		if daemon.TLSCertPath == "" || daemon.TLSKeyPath == "" {
			return errors.New("smtpd.Initialise: TLS certificate or key path is missing")
		}
		var err error
		daemon.TLSCertificate, err = tls.LoadX509KeyPair(daemon.TLSCertPath, daemon.TLSKeyPath)
		if err != nil {
			return fmt.Errorf("smtpd.Initialise: failed to read TLS certificate - %v", err)
		}
	}
	daemon.SMTPConfig = smtp.Config{
		Limits: &smtp.Limits{
			MsgSize:   2 * 1024 * 1024,            // Accept mails up to 2 MB large
			IOTimeout: IOTimeoutSec * time.Second, // IO timeout is a reasonable minute
			BadCmds:   64,                         // Abort connection after consecutive bad commands
		},
		// Greet SMTP clients with a list of domain names that this server receives emails for
		ServerName: strings.Join(daemon.MyDomains, " "),
	}
	if daemon.TLSCertPath != "" {
		daemon.SMTPConfig.TLSConfig = &tls.Config{Certificates: []tls.Certificate{daemon.TLSCertificate}}
	}
	daemon.RateLimit = &misc.RateLimit{
		MaxCount: daemon.PerIPLimit,
		UnitSecs: RateLimitIntervalSec,
		Logger:   daemon.Logger,
	}
	daemon.RateLimit.Initialise()
	// Do not allow forward to this daemon itself
	myPublicIP := inet.GetPublicIP()
	if (strings.HasPrefix(daemon.ForwardMailClient.MTAHost, "127.") ||
		daemon.ForwardMailClient.MTAHost == "::1" ||
		daemon.ForwardMailClient.MTAHost == myPublicIP) &&
		daemon.ForwardMailClient.MTAPort == daemon.Port {
		return errors.New("smtpd.Initialise: forward MTA must not be myself")
	}
	// Construct a hash of MyDomains addresses for fast lookup
	daemon.MyDomainsHash = map[string]struct{}{}
	for _, recv := range daemon.MyDomains {
		daemon.MyDomainsHash[recv] = struct{}{}
	}
	// Make sure that none of the forward addresses carries the domain name of MyDomains
	for _, fwd := range daemon.ForwardTo {
		atSign := strings.IndexRune(fwd, '@')
		if atSign == -1 {
			return fmt.Errorf("smtpd.Initialise: forward address \"%s\" must have an at sign", fwd)
		}
		if _, exists := daemon.MyDomainsHash[fwd[atSign+1:]]; exists {
			return fmt.Errorf("smtpd.Initialise: forward address \"%s\" must not loop back to this mail server's domain", fwd)
		}
	}
	// Initialise the optional toolbox command runner
	if daemon.CommandRunner == nil || daemon.CommandRunner.Processor == nil || daemon.CommandRunner.Processor.IsEmpty() {
		daemon.Logger.Printf("Initialise", "", nil, "daemon will not be able to execute toolbox commands due to lack of command processor filter configuration")
	} else {
		if err := daemon.CommandRunner.Initialise(); err != nil {
			return fmt.Errorf("smtpd.Initialise: %+v", err)
		}
		if !daemon.CommandRunner.ReplyMailClient.IsConfigured() {
			return errors.New("smtpd.Initialise: mail command runner's reply mailer must be configured")
		}
		// Do not allow mail processor to reply to this daemon itself
		if (strings.HasPrefix(daemon.CommandRunner.ReplyMailClient.MTAHost, "127.") ||
			daemon.CommandRunner.ReplyMailClient.MTAHost == "::1" ||
			daemon.CommandRunner.ReplyMailClient.MTAHost == myPublicIP) &&
			daemon.CommandRunner.ReplyMailClient.MTAPort == daemon.Port {
			return errors.New("smtpd.Initialise: mail command runner's reply MTA must not be myself")
		}
	}
	return nil
}

// Unconditionally forward the mail to forward addresses, then process feature commands if they are found.
func (daemon *Daemon) ProcessMail(fromAddr, mailBody string) {
	bodyBytes := []byte(mailBody)
	// Forward the mail
	if err := daemon.ForwardMailClient.SendRaw(daemon.ForwardMailClient.MailFrom, bodyBytes, daemon.ForwardTo...); err == nil {
		daemon.Logger.Printf("ProcessMail", fromAddr, nil, "successfully forwarded mail to %v", daemon.ForwardTo)
	} else {
		daemon.Logger.Warningf("ProcessMail", fromAddr, err, "failed to forward email")
	}
	// Run feature command from mail body
	if daemon.CommandRunner != nil && daemon.CommandRunner.Processor != nil && !daemon.CommandRunner.Processor.IsEmpty() {
		if err := daemon.CommandRunner.Process(bodyBytes, daemon.ForwardTo...); err != nil {
			daemon.Logger.Warningf("ProcessMail", fromAddr, err, "failed to process toolbox command from mail body")
		}
	}
}

// HandleConnection converses in SMTP over the connection, process retrieved email, and eventually close the connection.
func (daemon *Daemon) HandleConnection(clientConn net.Conn) {
	// Put conversation duration (including IO time) into statistics
	beginTimeNano := time.Now().UnixNano()
	defer func() {
		DurationStats.Trigger(float64(time.Now().UnixNano() - beginTimeNano))
	}()
	defer clientConn.Close()
	clientIP := clientConn.RemoteAddr().(*net.TCPAddr).IP.String()
	var numConversations int
	var finishedNormally bool
	var lastConversation, finishReason string
	// The SMTP conversation carried out by client will fill in these mail parameters
	var fromAddr, mailBody string
	toAddrs := make([]string, 0, 4)

	smtpConn := smtp.NewConn(clientConn, daemon.SMTPConfig, nil)
	rateLimitOK := daemon.RateLimit.Add(clientIP, true)
	for {
		// Politely reject the mail if rate limit is exceeded or too many conversations have taken place
		if !rateLimitOK || numConversations >= MaxConversationLength {
			smtpConn.Reply451()
			finishReason = "rate limit exceeded or too many conversations"
			goto done
		}
		// Continue conversation to retrieve incoming mail
		numConversations++
		ev := smtpConn.Next()
		// Remember the latest conversation for logging purpose
		lastConversation = fmt.Sprintf("%v[%v]:%v", ev.What, ev.Cmd, ev.Arg)
		switch ev.What {
		case smtp.DONE:
			finishReason = "done"
			finishedNormally = true
			goto done
		case smtp.ABORT:
			finishReason = "aborted"
			goto done
		case smtp.TLSERROR:
			finishReason = "TLS error"
			goto done
		case smtp.COMMAND:
			switch ev.Cmd {
			case smtp.MAILFROM:
				fromAddr = ev.Arg
			case smtp.RCPTTO:
				atSign := strings.IndexRune(ev.Arg, '@')
				if atSign == -1 {
					finishReason = fmt.Sprintf("rejected address \"%s\" does not contain at-sign", ev.Arg)
					smtpConn.Reject()
					goto done
				}
				if domain, exists := daemon.MyDomainsHash[ev.Arg[atSign+1:]]; !exists {
					finishReason = fmt.Sprintf("rejected domain \"%s\" that is not among my accepted domains", domain)
					smtpConn.Reject()
					goto done
				}
				toAddrs = append(toAddrs, ev.Arg)
			}
		case smtp.GOTDATA:
			mailBody = ev.Arg
		}
	}
done:
	if fromAddr == "" || len(toAddrs) == 0 {
		finishedNormally = false
		finishReason = "rejected mail due to missing parameters"
		smtpConn.Reject()
	}
	if finishedNormally {
		daemon.Logger.Printf("HandleConnection", clientIP, nil, "received mail from \"%s\" addressed to %v", fromAddr, toAddrs)
		// Forward the mail to forward-recipients, hence the original To-Addresses are not relevant.
		daemon.ProcessMail(fromAddr, mailBody)
		daemon.Logger.Printf("HandleConnection", clientIP, nil, "%s after %d conversations, last of which is: %s", finishReason, numConversations, lastConversation)
	} else {
		daemon.Logger.Warningf("HandleConnection", clientIP, nil, "%s after %d conversations, last of which is: %s", finishReason, numConversations, lastConversation)
	}
}

/*
You may call this function only after having called Initialise()!
Start SMTP daemon and block until daemon is told to stop.
*/
func (daemon *Daemon) StartAndBlock() (err error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", daemon.Address, daemon.Port))
	if err != nil {
		return fmt.Errorf("smtpd.StartAndBlock: failed to listen on %s:%d - %v", daemon.Address, daemon.Port, err)
	}
	defer listener.Close()
	daemon.Listener = listener
	// Process incoming TCP connections
	daemon.Logger.Printf("StartAndBlock", "", nil, "going to listen for connections")
	for {
		if misc.EmergencyLockDown {
			return misc.ErrEmergencyLockDown
		}
		clientConn, err := daemon.Listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "closed") {
				return nil
			}
			return fmt.Errorf("SMTPD.StartAndBlock: failed to accept new connection - %v", err)
		}
		go daemon.HandleConnection(clientConn)
	}
	return nil
}

// If SMTP daemon has started (i.e. listener is set), close the listener so that its connection loop will terminate.
func (daemon *Daemon) Stop() {
	if listener := daemon.Listener; listener != nil {
		if err := listener.Close(); err != nil {
			daemon.Logger.Warningf("Stop", "", err, "failed to close listener")
		}
	}
}

// Run unit tests on Daemon. See TestSMTPD_StartAndBlock for daemon setup.
func TestSMTPD(smtpd *Daemon, t testingstub.T) {
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
	addr := smtpd.Address + ":" + strconv.Itoa(smtpd.Port)
	// This really should be misc.HTTPPublicIPTimeoutSec * time.Second, but that would be too long.
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
