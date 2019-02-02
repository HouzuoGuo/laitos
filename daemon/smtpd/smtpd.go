package smtpd

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	netSMTP "net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/daemon/smtpd/mailcmd"
	"github.com/HouzuoGuo/laitos/daemon/smtpd/smtp"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/testingstub"
)

const (
	RateLimitIntervalSec  = 1   // Rate limit is calculated at 1 second interval
	IOTimeoutSec          = 60  // IO timeout for both read and write operations
	MaxConversationLength = 256 // Only converse up to this number of exchanges in an SMTP connection
)

// An SMTP daemon that receives mails addressed to its domain name, and optionally forward the received mails to other addresses.
type Daemon struct {
	Address     string   `json:"Address"`     // Network address to listen to, e.g. 0.0.0.0 for all network interfaces.
	Port        int      `json:"Port"`        // Port number to listen on
	TLSCertPath string   `json:"TLSCertPath"` // (Optional) serve StartTLS via this certificate
	TLSKeyPath  string   `json:"TLSKeyPath"`  // (Optional) serve StartTLS via this certificate (key)
	PerIPLimit  int      `json:"PerIPLimit"`  // PerIPLimit is approximately how many concurrent users are expected to be using the server from same IP address
	MyDomains   []string `json:"MyDomains"`   // Only accept mails addressed to these domain names
	ForwardTo   []string `json:"ForwardTo"`   // Forward received mails to these addresses

	CommandRunner     *mailcmd.CommandRunner `json:"-"` // Process feature commands from incoming mails
	ForwardMailClient inet.MailClient        `json:"-"` // ForwardMailClient is used to forward arriving emails.

	myDomainsHash map[string]struct{} // "MyDomains" values in map keys
	smtpConfig    smtp.Config         // SMTP processor configuration
	listener      net.Listener        // Once daemon is started, this is its TCP listener.
	tlsCert       tls.Certificate     // TLS certificate read from the certificate and key files
	rateLimit     *misc.RateLimit     // Rate limit counter per IP address
	logger        lalog.Logger

	// processMailTestCaseFunc works along side normal delivery routine, it offers mail message to test case for inspection.
	processMailTestCaseFunc func(string, string)
}

// Check configuration and initialise internal states.
func (daemon *Daemon) Initialise() error {
	if daemon.Address == "" {
		daemon.Address = "0.0.0.0"
	}
	if daemon.Port < 1 {
		daemon.Port = 25
	}
	if daemon.PerIPLimit < 1 {
		daemon.PerIPLimit = 4 // reasonable for receiving emails and running toolbox feature commands
	}
	daemon.logger = lalog.Logger{
		ComponentName: "smtpd",
		ComponentID:   []lalog.LoggerIDField{{Key: "Port", Value: daemon.Port}},
	}
	if daemon.ForwardTo == nil || len(daemon.ForwardTo) == 0 || !daemon.ForwardMailClient.IsConfigured() {
		return errors.New("smtpd.Initialise: forward address and forward mail client must be configured")
	}
	if daemon.MyDomains == nil || len(daemon.MyDomains) == 0 {
		return errors.New("smtpd.Initialise: my domain names must be configured")
	}
	if daemon.TLSCertPath != "" || daemon.TLSKeyPath != "" {
		if daemon.TLSCertPath == "" || daemon.TLSKeyPath == "" {
			return errors.New("smtpd.Initialise: TLS certificate or key path is missing")
		}
		var err error
		contents, _, err := misc.DecryptIfNecessary(misc.UniversalDecryptionKey, daemon.TLSCertPath, daemon.TLSKeyPath)
		if err != nil {
			return err
		}
		daemon.tlsCert, err = tls.X509KeyPair(contents[0], contents[1])
		if err != nil {
			return fmt.Errorf("smtpd.Initialise: failed to load certificate or key - %v", err)
		}
	}
	daemon.smtpConfig = smtp.Config{
		Limits: &smtp.Limits{
			MsgSize:   25 * 1024 * 1024,           // Accept mails up to 25 MB large (same as Gmail)
			IOTimeout: IOTimeoutSec * time.Second, // IO timeout is a reasonable minute
			BadCmds:   MaxConversationLength / 2,  // Abort connection after consecutive bad commands
		},
		// Greet SMTP clients with a list of domain names that this server receives emails for
		ServerName: strings.Join(daemon.MyDomains, " "),
	}
	if daemon.TLSCertPath != "" {
		daemon.smtpConfig.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{daemon.tlsCert},
		}
		daemon.smtpConfig.TLSConfig.BuildNameToCertificate()
	}

	daemon.rateLimit = &misc.RateLimit{
		MaxCount: daemon.PerIPLimit,
		UnitSecs: RateLimitIntervalSec,
		Logger:   daemon.logger,
	}
	daemon.rateLimit.Initialise()
	// Do not allow forward to this daemon itself
	myPublicIP := inet.GetPublicIP()
	if (strings.HasPrefix(daemon.ForwardMailClient.MTAHost, "127.") ||
		daemon.ForwardMailClient.MTAHost == "::1" ||
		daemon.ForwardMailClient.MTAHost == "0.0.0.0" ||
		daemon.ForwardMailClient.MTAHost == myPublicIP) &&
		daemon.ForwardMailClient.MTAPort == daemon.Port {
		return fmt.Errorf("smtpd.Initialise: forward MTA must not be myself or localhost on port %d", daemon.Port)
	}
	// Construct a hash of MyDomains addresses for fast lookup
	daemon.myDomainsHash = map[string]struct{}{}
	for _, recv := range daemon.MyDomains {
		daemon.myDomainsHash[recv] = struct{}{}
	}
	// Make sure that none of the forward addresses carries the domain name of MyDomains
	for _, fwd := range daemon.ForwardTo {
		atSign := strings.IndexRune(fwd, '@')
		if atSign == -1 {
			return fmt.Errorf("smtpd.Initialise: forward address \"%s\" must have an at sign", fwd)
		}
		if _, exists := daemon.myDomainsHash[fwd[atSign+1:]]; exists {
			return fmt.Errorf("smtpd.Initialise: forward address \"%s\" must not loop back to this mail server's domain", fwd)
		}
	}
	// Initialise the optional toolbox command runner
	if daemon.CommandRunner == nil || daemon.CommandRunner.Processor == nil || daemon.CommandRunner.Processor.IsEmpty() {
		daemon.logger.Info("Initialise", "", nil, "daemon will not be able to execute toolbox commands due to lack of command processor filter configuration")
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
		daemon.logger.Info("ProcessMail", fromAddr, nil, "successfully forwarded mail to %v", daemon.ForwardTo)
	} else {
		daemon.logger.Warning("ProcessMail", fromAddr, err, "failed to forward email")
	}
	// Offer the processed mail to test case
	if daemon.processMailTestCaseFunc != nil {
		daemon.processMailTestCaseFunc(fromAddr, mailBody)
	}
	// Run feature command from mail body
	if daemon.CommandRunner != nil && daemon.CommandRunner.Processor != nil && !daemon.CommandRunner.Processor.IsEmpty() {
		if err := daemon.CommandRunner.Process(bodyBytes); err != nil {
			daemon.logger.Warning("ProcessMail", fromAddr, err, "failed to process toolbox command from mail body")
		}
	}
}

// HandleConnection converses in SMTP over the connection, process retrieved email, and eventually close the connection.
func (daemon *Daemon) HandleConnection(clientConn net.Conn) {
	// Put conversation duration (including IO time) into statistics
	beginTimeNano := time.Now().UnixNano()
	defer func() {
		common.SMTPDStats.Trigger(float64(time.Now().UnixNano() - beginTimeNano))
	}()
	defer clientConn.Close()
	clientIP := clientConn.RemoteAddr().(*net.TCPAddr).IP.String()
	var numConversations int
	// The status string is only used for logging
	var completionStatus string
	// memorise latest conversations for logging purpose
	latestConv := lalog.NewRingBuffer(4)
	// fromAddr, mailBody, and toAddrs will be filled as SMTP conversation goes on
	var fromAddr, mailBody string
	toAddrs := make([]string, 0, 4)

	smtpConn := smtp.NewConn(clientConn, daemon.smtpConfig, nil)
	rateLimitOK := daemon.rateLimit.Add(clientIP, true)
	if !rateLimitOK {
		smtpConn.Reply451()
		return
	}

	for {
		if numConversations >= MaxConversationLength {
			smtpConn.Reply451()
			completionStatus = "conversation is taking too long"
			goto done
		}
		// Carry on with conversation
		numConversations++
		ev := smtpConn.Next()
		// Memorise latest conversation for logging
		logConv := fmt.Sprintf("%v[%v](%v)", ev.What, ev.Cmd, ev.Arg)
		if len(logConv) > 80 {
			logConv = logConv[:80]
		}
		latestConv.Push(logConv)
		switch ev.What {
		case smtp.DONE:
			completionStatus = "done"
			goto done
		case smtp.ABORT:
			completionStatus = fmt.Sprintf("aborted (%s)", ev.Arg)
			goto done
		case smtp.COMMAND:
			switch ev.Cmd {
			case smtp.MAILFROM:
				fromAddr = ev.Arg
			case smtp.RCPTTO:
				atSign := strings.IndexRune(ev.Arg, '@')
				if atSign > 0 {
					if domain, exists := daemon.myDomainsHash[ev.Arg[atSign+1:]]; exists {
						toAddrs = append(toAddrs, ev.Arg)
					} else {
						completionStatus = fmt.Sprintf("rejected domain \"%s\" that is not among my accepted domains", domain)
						smtpConn.Reject()
						goto done
					}
				}
			}
		case smtp.GOTDATA:
			mailBody = ev.Arg
		}
	}
done:
	if fromAddr != "" && len(toAddrs) > 0 && mailBody != "" {
		daemon.logger.Info("HandleConnection", clientIP, nil, "received mail from \"%s\" addressed to %s", fromAddr, strings.Join(toAddrs, ", "))
		// Forward the mail to forward-recipients, hence the original To-Addresses are not relevant.
		daemon.ProcessMail(fromAddr, mailBody)
	} else {
		smtpConn.Reject()
		completionStatus += " & rejected mail due to missing parameters"
	}
	daemon.logger.Info("HandleConnection", clientIP, nil, "%s after %d conversations (TLS: %s), last commands: %s",
		completionStatus, numConversations, smtpConn.TLSHelp, strings.Join(latestConv.GetAll(), " | "))
}

/*
You may call this function only after having called Initialise()!
Start SMTP daemon and block until daemon is told to stop.
*/
func (daemon *Daemon) StartAndBlock() (err error) {
	listener, err := net.Listen("tcp", net.JoinHostPort(daemon.Address, strconv.Itoa(daemon.Port)))
	if err != nil {
		return fmt.Errorf("smtpd.StartAndBlock: failed to listen on %s:%d - %v", daemon.Address, daemon.Port, err)
	}
	defer listener.Close()
	daemon.listener = listener
	// Process incoming TCP connections
	daemon.logger.Info("StartAndBlock", "", nil, "going to listen for connections")
	for {
		if misc.EmergencyLockDown {
			return misc.ErrEmergencyLockDown
		}
		clientConn, err := daemon.listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "closed") {
				return nil
			}
			return fmt.Errorf("SMTPD.StartAndBlock: failed to accept new connection - %v", err)
		}
		go daemon.HandleConnection(clientConn)
	}
}

// If SMTP daemon has started (i.e. listener is set), close the listener so that its connection loop will terminate.
func (daemon *Daemon) Stop() {
	if listener := daemon.listener; listener != nil {
		if err := listener.Close(); err != nil {
			daemon.logger.Warning("Stop", "", err, "failed to close listener")
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
	// This really should be inet.HTTPPublicIPTimeoutSec * time.Second, but that would be too long.
	time.Sleep(3 * time.Second)
	// Try to exceed rate limit
	testMessage := "Content-type: text/plain; charset=utf-8\r\nFrom: MsgFrom@whatever\r\nTo: MsgTo@whatever\r\nSubject: text subject\r\n\r\ntest body"
	success := 0
	for i := 0; i < 100; i++ {
		if err := netSMTP.SendMail(addr, nil, "ClientFrom@localhost", []string{"ClientTo@howard.name"}, []byte(testMessage)); err == nil {
			success++
		}
	}
	if success < 1 || success > smtpd.PerIPLimit*2 {
		t.Fatal("delivered", success)
	}
	// Wait till rate limit expires (leave 3 seconds buffer for pending transfer)
	time.Sleep((RateLimitIntervalSec + 3) * time.Second)
	// Send an ordinary mail to the daemon
	var lastEmailFrom, lastEmailBody string
	smtpd.processMailTestCaseFunc = func(from string, body string) {
		lastEmailFrom = from
		lastEmailBody = body
	}
	testMessage = "Content-type: text/plain; charset=utf-8\r\nFrom: MsgFrom@whatever\r\nTo: MsgTo@whatever\r\nSubject: text subject\r\n\r\ntest body\r\n"
	lastEmailFrom = ""
	lastEmailBody = ""
	if err := netSMTP.SendMail(addr, nil, "ClientFrom@localhost", []string{"ClientTo@example.com"}, []byte(testMessage)); err != nil {
		t.Fatal(err)
	} else if lastEmailFrom != "ClientFrom@localhost" || lastEmailBody != strings.Replace(testMessage, "\r\n", "\n", -1) {
		// Keep in mind that server reads input mail message through the textproto.DotReader
		t.Fatalf("%+v\n'%+v'\n'%+v'\n", lastEmailFrom, []byte(testMessage), []byte(lastEmailBody))
	}
	// Send a mail that does not belong to this server's domain, which will be simply discarded.
	testMessage = "Content-type: text/plain; charset=utf-8\r\nFrom: MsgFrom@whatever\r\nTo: MsgTo@whatever\r\nSubject: text subject\r\n\r\ntest body\r\n"
	lastEmailFrom = ""
	lastEmailBody = ""
	if err := netSMTP.SendMail(addr, nil, "ClientFrom@localhost", []string{"ClientTo@not-my-domain"}, []byte(testMessage)); strings.Index(err.Error(), "Bad address") == -1 {
		t.Fatal(err)
	} else if lastEmailFrom != "" || lastEmailBody != "" {
		t.Fatal(lastEmailFrom, lastEmailBody)
	}
	// Try run a command via email
	testMessage = "Content-type: text/plain; charset=utf-8\r\nFrom: MsgFrom@whatever\r\nTo: MsgTo@whatever\r\nSubject: command subject\r\n\r\nverysecret.s echo hi\r\n"
	lastEmailFrom = ""
	lastEmailBody = ""
	if err := netSMTP.SendMail(addr, nil, "ClientFrom@localhost", []string{"ClientTo@howard.name"}, []byte(testMessage)); err != nil {
		t.Fatal(err)
	} else if lastEmailFrom != "ClientFrom@localhost" || lastEmailBody != strings.Replace(testMessage, "\r\n", "\n", -1) {
		// Keep in mind that server reads input mail message through the textproto.DotReader
		t.Fatal(lastEmailFrom, lastEmailBody)
	}
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
