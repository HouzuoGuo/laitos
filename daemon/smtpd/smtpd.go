package smtpd

import (
	"crypto/tls"
	"errors"
	"fmt"
	"math/rand"
	"net"
	netSMTP "net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/daemon/smtpd/mailcmd"
	"github.com/HouzuoGuo/laitos/daemon/smtpd/smtp"
	"github.com/HouzuoGuo/laitos/datastruct"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/testingstub"
)

const (
	IOTimeoutSec          = 60  // IO timeout for both read and write operations
	MaxConversationLength = 256 // Only converse up to this number of exchanges in an SMTP connection
	MaxNumRecipients      = 100 // MaxNumRecipients is the maximum number of recipients an SMTP conversation will accept
)

// Daemon implements an SMTP server that receives mails addressed to configured set of domain names, and optionally forward the received mails to other addresses.
type Daemon struct {
	Address     string `json:"Address"`     // Address is the TCP address listen to, e.g. 0.0.0.0 for all network interfaces.
	Port        int    `json:"Port"`        // Port number to listen on.
	TLSCertPath string `json:"TLSCertPath"` // TLSCertPath is the path to server's TLS certificate for StartTLS operation. This is optional.
	TLSKeyPath  string `json:"TLSKeyPath"`  // TLSCertPath is the path to server's TLS certificate key for StartTLS operation. This is optional.
	PerIPLimit  int    `json:"PerIPLimit"`  // PerIPLimit is the maximum number of approximately how many concurrent users are expected to be using the server from same IP address
	// MyDomains is an array of domain names that this SMTP server receives mails for. Mails addressed to domain names other than these will be rejected.
	MyDomains []string `json:"MyDomains"`
	// ForwardTo are the recipients (email addresses) to receive emails that are delivered to this SMTP server.
	ForwardTo []string `json:"ForwardTo"`

	CommandRunner     *mailcmd.CommandRunner `json:"-"` // Process feature commands from incoming mails
	ForwardMailClient inet.MailClient        `json:"-"` // ForwardMailClient is used to forward arriving emails.

	myDomainsHash map[string]struct{} // myDomainHash has "MyDomains" in map keys
	smtpConfig    smtp.Config
	tlsCert       tls.Certificate
	tcpServer     *common.TCPServer
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
		contents, _, err := misc.DecryptIfNecessary(misc.ProgramDataDecryptionPassword, daemon.TLSCertPath, daemon.TLSKeyPath)
		if err != nil {
			return err
		}
		daemon.tlsCert, err = tls.X509KeyPair(contents[0], contents[1])
		if err != nil {
			return fmt.Errorf("smtpd.Initialise: failed to load certificate or key - %v", err)
		}
	}
	daemon.smtpConfig = smtp.Config{
		IOTimeout:                          IOTimeoutSec * time.Second, // IO timeout is a reasonable minute
		MaxMessageLength:                   inet.MaxMailBodySize,
		MaxConsecutiveUnrecognisedCommands: MaxConversationLength / 2, // Abort connection after consecutive bad commands
		// Greet SMTP clients with a list of domain names that this server receives emails for
		ServerName: strings.Join(daemon.MyDomains, " "),
	}
	if daemon.TLSCertPath != "" {
		daemon.smtpConfig.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{daemon.tlsCert},
		}
	}

	// Do not allow forward to this daemon itself
	myPublicIP := inet.GetPublicIP()
	if (strings.HasPrefix(daemon.ForwardMailClient.MTAHost, "127.") ||
		daemon.ForwardMailClient.MTAHost == "::1" ||
		daemon.ForwardMailClient.MTAHost == "0.0.0.0" ||
		daemon.ForwardMailClient.MTAHost == myPublicIP.String()) &&
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
		daemon.logger.Info("", nil, "daemon will not be able to execute toolbox commands due to lack of command processor filter configuration")
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
			daemon.CommandRunner.ReplyMailClient.MTAHost == myPublicIP.String()) &&
			daemon.CommandRunner.ReplyMailClient.MTAPort == daemon.Port {
			return errors.New("smtpd.Initialise: mail command runner's reply MTA must not be myself")
		}
	}
	// Configure and initialise TCP server
	daemon.tcpServer = &common.TCPServer{
		ListenAddr:  daemon.Address,
		ListenPort:  daemon.Port,
		AppName:     "smtpd",
		App:         daemon,
		LimitPerSec: daemon.PerIPLimit,
	}
	daemon.tcpServer.Initialise()
	return nil
}

// Unconditionally forward the mail to forward addresses, then process feature commands if they are found.
func (daemon *Daemon) ProcessMail(clientIP, fromAddr, mailBody string) {
	bodyBytes := []byte(mailBody)
	// Run feature command from mail body
	if daemon.CommandRunner != nil && daemon.CommandRunner.Processor != nil && !daemon.CommandRunner.Processor.IsEmpty() {
		if err := daemon.CommandRunner.Process(clientIP, bodyBytes); err != nil {
			daemon.logger.Info(fromAddr, nil, "failed to process toolbox command from mail body - %v", err)
		}
	}
	// Determine whether the sender enforces DMARC policy
	fromAddrWithoutDmarc := GetFromAddressWithDmarcWorkaround(fromAddr, rand.Intn(100000))
	if fromAddrWithoutDmarc != fromAddr {
		// Change the sender's domain to the non-existent domain without a DMARC policy
		daemon.logger.Info(fromAddr, nil, "rewriting From address from %s to %s to evade DMARC validation", fromAddr, fromAddrWithoutDmarc)
		fromAddr = fromAddrWithoutDmarc
		// Change the sender's domain in "From:" header
		bodyBytes = WithHeaderFromAddr(bodyBytes, fromAddrWithoutDmarc)
	}
	// Forward the mail to all recipients
	if err := daemon.ForwardMailClient.SendRaw(daemon.ForwardMailClient.MailFrom, bodyBytes, daemon.ForwardTo...); err == nil {
		daemon.logger.Info(fromAddr, nil, "successfully forwarded mail to %v", daemon.ForwardTo)
	} else {
		daemon.logger.Warning(fromAddr, err, "failed to forward email")
	}
	// Offer the processed mail to test case
	if daemon.processMailTestCaseFunc != nil {
		daemon.processMailTestCaseFunc(fromAddr, string(bodyBytes))
	}
}

// GetTCPStatsCollector returns the stats collector that counts and times client connections for the TCP application.
func (daemon *Daemon) GetTCPStatsCollector() *misc.Stats {
	return misc.SMTPDStats
}

// HandleTCPConnection converses with the SMTP client. The client connection is closed by server upon returning from the implementation.
func (daemon *Daemon) HandleTCPConnection(logger lalog.Logger, ip string, client *net.TCPConn) {
	var numCommands int
	// The status string is only used for logging
	var completionStatus string
	// memorise latest conversations for logging purpose
	latestConv := datastruct.NewRingBuffer(4)
	// fromAddr, mailBody, and toAddrs will be filled as SMTP conversation goes on
	var fromAddr, mailBody string
	toAddrs := make([]string, 0, 4)

	smtpConn := smtp.NewConnection(client, daemon.smtpConfig, nil)
	for {
		if misc.EmergencyLockDown {
			daemon.logger.Warning("", misc.ErrEmergencyLockDown, "")
			return
		}
		if numCommands >= MaxConversationLength {
			smtpConn.AnswerRateLimited()
			completionStatus = "conversation is taking too long"
			goto done
		}
		// Carry on with conversation
		numCommands++
		ev := smtpConn.CarryOn()
		// Memorise latest conversation for logging
		logConv := fmt.Sprintf("%v[%v](%v)", ev.State, ev.Verb, ev.Parameter)
		if len(logConv) > 80 {
			logConv = logConv[:80]
		}
		latestConv.Push(logConv)
		switch ev.State {
		case smtp.ConvCompleted:
			completionStatus = "done"
			goto done
		case smtp.ConvAborted:
			completionStatus = fmt.Sprintf("aborted (%s)", ev.Parameter)
			goto done
		case smtp.ConvReceivedCommand:
			switch ev.Verb {
			case smtp.VerbMAILFROM:
				fromAddr = ev.Parameter
			case smtp.VerbRCPTTO:
				atSign := strings.IndexRune(ev.Parameter, '@')
				if atSign > 0 {
					if domain, exists := daemon.myDomainsHash[ev.Parameter[atSign+1:]]; exists {
						if len(toAddrs) < MaxNumRecipients {
							toAddrs = append(toAddrs, ev.Parameter)
						}
					} else {
						completionStatus = fmt.Sprintf("rejected domain \"%s\" that is not among my accepted domains", domain)
						smtpConn.AnswerNegative()
						goto done
					}
				}
			}
		case smtp.ConvReceivedData:
			mailBody = ev.Parameter
		}
	}
done:
	if fromAddr != "" && len(toAddrs) > 0 && mailBody != "" {
		daemon.logger.Info(ip, nil, "received mail from \"%s\" addressed to %s", fromAddr, strings.Join(toAddrs, ", "))
		// Check sender IP against blacklist, do not proceed further if the sender IP has been blacklisted.
		if blacklistDomainName := IsSuspectIPBlacklisted(ip); blacklistDomainName == "" {
			// Forward the mail to forward-recipients, hence the original To-Addresses are not relevant.
			daemon.ProcessMail(ip, fromAddr, mailBody)
		} else {
			completionStatus += " & rejected mail due to blacklist"
			daemon.logger.Warning(ip, nil, "not going to process the mail further because the client IP was blacklisted by %s. The mail content was: %s", blacklistDomainName, mailBody)
			smtpConn.AnswerNegative()
		}
	} else {
		smtpConn.AnswerNegative()
		completionStatus += " & rejected mail due to missing parameters or blacklist"
	}
	daemon.logger.Info(ip, nil, "%s after %d conversations (TLS: %s), last commands: %s",
		completionStatus, numCommands, smtpConn.TLSHelp, strings.Join(latestConv.GetAll(), " | "))
}

/*
You may call this function only after having called Initialise()!
Start SMTP daemon and block until daemon is told to stop.
*/
func (daemon *Daemon) StartAndBlock() (err error) {
	return daemon.tcpServer.StartAndBlock()
}

// If SMTP daemon has started (i.e. listener is set), close the listener so that its connection loop will terminate.
func (daemon *Daemon) Stop() {
	daemon.tcpServer.Stop()
}

// Run unit tests on Daemon. See TestSMTPD_StartAndBlock for daemon setup.
func TestSMTPD(smtpd *Daemon, t testingstub.T) {
	/*
		SMTP daemon is expected to start in a few seconds, it may take a short while because
		the daemon has to figure out its public IP address.
	*/
	serverStopped := make(chan struct{}, 1)
	go func() {
		if err := smtpd.StartAndBlock(); err != nil {
			t.Error(err)
			return
		}
		serverStopped <- struct{}{}
	}()
	if !misc.ProbePort(30*time.Second, smtpd.Address, smtpd.Port) {
		t.Fatal("daemon did not start in time")
	}

	// Send an ordinary mail to the daemon
	addr := smtpd.Address + ":" + strconv.Itoa(smtpd.Port)
	lastEmailFrom := make(chan string, 100)
	lastEmailBody := make(chan string, 100)
	smtpd.processMailTestCaseFunc = func(from string, body string) {
		lastEmailFrom <- from
		lastEmailBody <- body
	}
	testMessage := "Content-type: text/plain; charset=utf-8\r\nFrom: MsgFrom@whatever\r\nTo: MsgTo@whatever\r\nSubject: text subject\r\n\r\ntest body\r\n"
	if err := netSMTP.SendMail(addr, nil, "ClientFrom@localhost", []string{"ClientTo@example.com"}, []byte(testMessage)); err != nil {
		t.Fatal(err)
	}
	// Due to unknown reason, netSMTP.SendMail always returns prematurely before it has completed the conversation with SMTP server.
	time.Sleep((DNSBlackListQueryTimeoutSec + 1) * time.Second)
	if from, body := <-lastEmailFrom, <-lastEmailBody; from != "ClientFrom@localhost" || body != strings.Replace(testMessage, "\r\n", "\n", -1) {
		// Keep in mind that server reads input mail message through the textproto.DotReader
		t.Fatalf("%+v\n'%+v'\n'%+v'\n", lastEmailFrom, testMessage, lastEmailBody)
	}

	// Send a mail with a From address of a DMARC-enforcing domain
	testMessage = "Content-type: text/plain; charset=utf-8\r\nFrom: MsgFrom@microsoft.com\r\nTo: MsgTo@whatever\r\nSubject: text subject\r\n\r\ntest body\r\n"
	if err := netSMTP.SendMail(addr, nil, "MsgFrom@microsoft.com", []string{"ClientTo@example.com"}, []byte(testMessage)); err != nil {
		t.Fatal(err)
	}
	time.Sleep((DNSBlackListQueryTimeoutSec + 1) * time.Second)
	if from, body := <-lastEmailFrom, <-lastEmailBody; !strings.HasPrefix(from, "MsgFrom@microsoft-laitos-nodmarc-") || !strings.HasSuffix(from, ".com") ||
		!strings.Contains(body, "From: "+from) || !strings.Contains(body, "Subject: text subject\n\ntest body") {
		// Keep in mind that server reads input mail message through the textproto.DotReader
		t.Fatalf("%+v\n'%+v'\n'%+v'\n", lastEmailFrom, testMessage, lastEmailBody)
	}

	// Send a mail that does not belong to this server's domain, which will be simply discarded.
	testMessage = "Content-type: text/plain; charset=utf-8\r\nFrom: MsgFrom@whatever\r\nTo: MsgTo@whatever\r\nSubject: text subject\r\n\r\ntest body\r\n"
	if err := netSMTP.SendMail(addr, nil, "ClientFrom@localhost", []string{"ClientTo@not-my-domain"}, []byte(testMessage)); !strings.Contains(err.Error(), "Bad address") {
		t.Fatal(err)
	}
	select {
	case <-time.After((DNSBlackListQueryTimeoutSec + 1) * time.Second):
		// Good, the test function did not get this email, which does not belong to any of the domains handled by SMTP server.
	case from := <-lastEmailFrom:
		t.Fatalf("should not have handled email from %s", from)
	}

	// Try run a command via email
	testMessage = "From: MsgFrom@whatever\r\nTo: MsgTo@whatever\r\nSubject: command subject\r\n\r\n  \tverysecret.s echo hi\r\n"
	if err := netSMTP.SendMail(addr, nil, "ClientFrom@localhost", []string{"ClientTo@howard.name"}, []byte(testMessage)); err != nil {
		t.Fatal(err)
	}
	time.Sleep((DNSBlackListQueryTimeoutSec + 1) * time.Second)
	if from, body := <-lastEmailFrom, <-lastEmailBody; from != "ClientFrom@localhost" || body != strings.Replace(testMessage, "\r\n", "\n", -1) {
		// Keep in mind that server reads input mail message through the textproto.DotReader
		t.Fatal(lastEmailFrom, lastEmailBody)
	}

	smtpd.Stop()
	<-serverStopped
	// Repeatedly stopping the daemon should have no negative consequence
	smtpd.Stop()
	smtpd.Stop()
}
