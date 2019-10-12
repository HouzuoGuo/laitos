package toolbox

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/textproto"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/inet"
)

const (
	MailboxList    = "l" // Prefix string to trigger listing messages.
	MailboxRead    = "r" // Prefix string to trigger reading message body.
	IMAPTimeoutSec = 30  // IMAPTimeoutSec is the IO timeout (in seconds) used for each IMAP conversation.
)

var (
	RegexMailboxAndNumber     = regexp.MustCompile(`(\w+)[^\w]+(\d+)`)            // Capture one mailbox shortcut name and a number
	RegexMailboxAndTwoNumbers = regexp.MustCompile(`(\w+)[^\w]+(\d+)[^\d]+(\d+)`) // Capture one mailbox shortcut name and two numbers
	ErrBadMailboxParam        = fmt.Errorf("%s box skip# count# | %s box to-read#", MailboxList, MailboxRead)
)

// IMAPSConnection is an established TLS client connection that is ready for IMAP conversations.
type IMAPSConnection struct {
	tlsConn *tls.Conn   // tlsConn is a TLS client opened toward IMAP host. It had gone through handshake external to IMAPSConnection.
	mutex   *sync.Mutex // mutex allows only one conversation to take place at a time.
}

/*
converse sends an IMAP request and waits for a response, then return IMAP response status and body.
If the response status is not OK, an error will be returned. If IO error occurs, client connection will be closed and an
error will be returned.
*/
func (conn *IMAPSConnection) converse(request string) (status, body string, err error) {
	// Expect both request and response to complete within the timeout constraint
	_ = conn.tlsConn.SetDeadline(time.Now().Add(time.Duration(IMAPTimeoutSec) * time.Second))
	// Random challenge is a string prefixed to an IMAP request
	challenge := randomChallenge()
	_, err = conn.tlsConn.Write([]byte(fmt.Sprintf("%s %s\r\n", challenge, request)))
	if err != nil {
		conn.disconnect()
		return
	}
	// IMAP protocol is very much line-oriented in both of its request and response
	var allLines bytes.Buffer
	// Allow up to 32MB of data to be received per conversation
	reader := textproto.NewReader(bufio.NewReader(io.LimitReader(conn.tlsConn, 32*1048576)))
	for {
		var line string
		line, err = reader.ReadLine()
		if err != nil {
			conn.disconnect()
			return
		}
		lowerLine := strings.TrimSpace(strings.ToLower(string(line)))
		if strings.Index(lowerLine, challenge) == 0 {
			// Conversation is finished when the response line comes with random challenge that was sent moments ago
			body = allLines.String()
			withoutChallenge := strings.TrimSpace(lowerLine[len(challenge):])
			// There is a single-word status at the beginning
			afterStatusWord := strings.IndexRune(withoutChallenge, ' ')
			if afterStatusWord == -1 {
				status = withoutChallenge
				err = fmt.Errorf("cannot find IMAP status word among line - %s", withoutChallenge)
				conn.disconnect()
				return
			}
			statusWord := withoutChallenge[:afterStatusWord]
			if len(withoutChallenge) > afterStatusWord {
				status = withoutChallenge[afterStatusWord:]
			}
			if strings.ToLower(statusWord) != "ok" {
				err = fmt.Errorf("bad IMAP response status - %s", status)
				// Bad status does not prevent further conversations from taking place
			}
			return
		} else {
			// Continue to receive response body
			allLines.WriteString(line)
			allLines.WriteRune('\n')
		}
	}
}

/*
converse sends an IMAP request and waits for a response, then return IMAP response status and body.
If the response status is not OK, an error will be returned. If IO error occurs, client connection will be closed and an
error will be returned. A mutex prevents more than one conversation from taking place at the same time.
*/
func (conn *IMAPSConnection) Converse(request string) (status, body string, err error) {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	if conn.tlsConn == nil {
		return "", "", errors.New("programming mistake - IMAPS connection is missing")
	}
	return conn.converse(request)
}

// disconnect closes client connection.
func (conn *IMAPSConnection) disconnect() {
	if conn.tlsConn == nil {
		return
	}
	conn.tlsConn.Close()
	conn.tlsConn = nil
}

// LogoutDisconnect sends logout command to IMAP server, and then closes client connection.
func (conn *IMAPSConnection) LogoutDisconnect() {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	if conn.tlsConn == nil {
		return
	}
	_, _, _ = conn.converse("LOGOUT") // intentionally ignore conversation error
	conn.disconnect()
}

// GetNumberMessages returns total number of messages in the specified mail box.
func (conn *IMAPSConnection) GetNumberMessages(mailboxName string) (int, error) {
	_, body, err := conn.Converse(fmt.Sprintf("EXAMINE \"%s\"", mailboxName))
	if err != nil {
		return 0, err
	}
	// Extract number of messages from response body
	numberString := regexp.MustCompile(`(\d+) exists`).FindStringSubmatch(strings.ToLower(body))
	if len(numberString) != 2 {
		return 0, fmt.Errorf("IMAPS.GetNumberMessages: EXAMINE command did not return a number in body - %s", body)
	}
	number, err := strconv.Atoi(numberString[1])
	if err != nil || number < 0 {
		return 0, fmt.Errorf("IMAPS.GetNumberMessages: EXAMINE command did not return a valid positive integer in \"%s\" - %v", numberString[1], err)
	}
	return number, nil
}

// GetHeaders retrieves mail headers from the specified message number range.
func (conn *IMAPSConnection) GetHeaders(from, to int) (ret map[int]string, err error) {
	ret = make(map[int]string)
	if from > to || from < 1 || to < 1 {
		err = errors.New("invalid message number range")
		return
	}
	_, body, err := conn.Converse(fmt.Sprintf("FETCH %d:%d BODY.PEEK[HEADER]", from, to))
	if err != nil {
		return
	}
	// Walk through body line by line to find boundary of messages
	var thisNumber int
	var thisMessage bytes.Buffer
	for _, line := range strings.Split(body, "\n") {
		trimmedLine := strings.TrimSpace(line)
		if len(trimmedLine) == 0 {
			continue
		} else if len(trimmedLine) > 0 && trimmedLine[0] == '*' {
			// Marks beginning of a message
			// Store existing message
			if thisMessage.Len() > 0 {
				// Only store valid message
				ret[thisNumber] = thisMessage.String()
				thisNumber = 0
				thisMessage.Reset()
			}
			// Parse current message number
			thisNumberStr := regexp.MustCompile(`\d+`).FindString(trimmedLine)
			thisNumber, _ = strconv.Atoi(thisNumberStr)
		} else if trimmedLine == ")" {
			// ) on its own line signifies end of message
			if thisMessage.Len() > 0 {
				ret[thisNumber] = thisMessage.String()
			}
		} else {
			// Place the line in the current message buffer
			thisMessage.WriteString(line)
			thisMessage.WriteRune('\n')
		}
	}
	return
}

// GetMessage retrieves one mail message, including its entire headers, body content, and attachments if any.
func (conn *IMAPSConnection) GetMessage(num int) (message string, err error) {
	if num < 1 {
		err = errors.New("message number must be positive")
		return
	}
	var entireMessage bytes.Buffer
	_, body, err := conn.Converse(fmt.Sprintf("FETCH %d BODY[]", num))
	for _, line := range strings.Split(body, "\n") {
		if len(line) > 0 {
			switch line[0] {
			// Skip fetch boundary lines
			case '*', ')':
				continue
			}
		}
		entireMessage.WriteString(line)
		entireMessage.WriteRune('\n')
	}
	message = entireMessage.String()
	return
}

// Retrieve emails via IMAPS.
type IMAPS struct {
	Host               string `json:"Host"`               // Server name or IP address of IMAPS server
	Port               int    `json:"Port"`               // Port number of IMAPS service
	MailboxName        string `json:"MailboxName"`        // Name of mailbox (e.g. "INBOX")
	InsecureSkipVerify bool   `json:"InsecureSkipVerify"` // Do not verify server name against its certificate
	AuthUsername       string `json:"AuthUsername"`       // Username for plain authentication
	AuthPassword       string `json:"AuthPassword"`       // Password for plain authentication
}

// Return a random 10 characters long string of numbers to
func randomChallenge() string {
	return strconv.Itoa(1000000000 + rand.Intn(1000000000))
}

// Set up TLS connection to IMAPS server and log the user in.
func (mbox *IMAPS) ConnectLoginSelect() (conn *IMAPSConnection, err error) {
	clientConn, err := net.DialTimeout(
		"tcp",
		net.JoinHostPort(mbox.Host, strconv.Itoa(mbox.Port)),
		time.Duration(IMAPTimeoutSec)*time.Second)
	if err != nil {
		return nil, fmt.Errorf("IMAPS.ConnectLoginSelect: connection error - %v", err)
	}
	tlsWrapper := tls.Client(clientConn, &tls.Config{
		ServerName:         mbox.Host,
		InsecureSkipVerify: mbox.InsecureSkipVerify,
	})
	if err = tlsWrapper.Handshake(); err != nil {
		clientConn.Close()
		return nil, fmt.Errorf("IMAPS.ConnectLoginSelect: TLS connection error - %v", err)
	}
	// Absorb the connection greeting message sent by server
	_ = tlsWrapper.SetReadDeadline(time.Now().Add(time.Duration(IMAPTimeoutSec) * time.Second))
	reader := bufio.NewReader(tlsWrapper)
	_, _, err = reader.ReadLine()
	if err != nil {
		clientConn.Close()
		return nil, fmt.Errorf("IMAPS.ConnectLoginSelect: failed to read server greeting - %v", err)
	}
	// It is now ready for IMAP conversations
	conn = &IMAPSConnection{
		tlsConn: tlsWrapper,
		mutex:   new(sync.Mutex),
	}
	// LOGIN && SELECT
	_, _, err = conn.Converse(fmt.Sprintf("LOGIN %s %s", mbox.AuthUsername, mbox.AuthPassword))
	if err != nil {
		conn.disconnect()
		return nil, fmt.Errorf("IMAPS.ConnectLoginSelect: LOGIN command failed - %v", err)
	}
	_, _, err = conn.Converse(fmt.Sprintf("SELECT \"%s\"", mbox.MailboxName))
	if err != nil {
		conn.LogoutDisconnect()
		return nil, fmt.Errorf("IMAPS.ConnectLoginSelect: SELECT command failed - %v", err)
	}
	return
}

// Correspond IMAP account connection details to account names.
type IMAPAccounts struct {
	Accounts map[string]*IMAPS `json:"Accounts"` // IMAP account name vs account connectivity details
}

var TestIMAPAccounts = IMAPAccounts{} // Account details are set by init_feature_test.go

func (imap *IMAPAccounts) IsConfigured() bool {
	if imap.Accounts == nil || len(imap.Accounts) == 0 {
		return false
	}
	for _, account := range imap.Accounts {
		if account.Host == "" || account.AuthPassword == "" || account.AuthUsername == "" {
			return false
		}
	}
	return true
}

func (imap *IMAPAccounts) SelfTest() error {
	if !imap.IsConfigured() {
		return ErrIncompleteConfig
	}
	for name, account := range imap.Accounts {
		imapConn, err := account.ConnectLoginSelect()
		if err != nil {
			return fmt.Errorf("IMAPAccounts.SelfTest: account \"%s\" has connection error - %v", name, err)
		}
		if _, err := imapConn.GetNumberMessages(account.MailboxName); err != nil {
			imapConn.LogoutDisconnect()
			return fmt.Errorf("IMAPAccounts.SelfTest: account \"%s\" test error - %v", name, err)
		}
		imapConn.LogoutDisconnect()
	}
	return nil
}

func (imap *IMAPAccounts) Initialise() error {
	// Use default port number 993 and default mailbox name INBOX
	for _, account := range imap.Accounts {
		if account.Port < 1 {
			account.Port = 993
		}
		if account.MailboxName == "" {
			account.MailboxName = "INBOX"
		}
	}
	return nil
}

func (imap *IMAPAccounts) Trigger() Trigger {
	return ".i"
}

func (imap *IMAPAccounts) ListMails(cmd Command) *Result {
	// Find one string parameter and two numeric parameters among the content
	params := RegexMailboxAndTwoNumbers.FindStringSubmatch(cmd.Content)
	if len(params) < 4 {
		return &Result{Error: ErrBadMailboxParam}
	}
	var mbox string
	var skip, count int
	mbox = params[1]
	var intErr error
	skip, intErr = strconv.Atoi(params[2])
	if intErr != nil {
		return &Result{Error: ErrBadMailboxParam}
	}
	count, intErr = strconv.Atoi(params[3])
	if intErr != nil {
		return &Result{Error: ErrBadMailboxParam}
	}
	// Artificially do not allow retrieving more than 200 message headers at a time
	if skip < 0 {
		skip = 0
	}
	if count > 200 {
		count = 200
	}
	if count < 1 {
		count = 1
	}
	// Let IMAP magic begin!
	account, found := imap.Accounts[mbox]
	if !found {
		return &Result{Error: fmt.Errorf("IMAPAccounts.ListMails: cannot find mailbox \"%s\"", mbox)}
	}
	conn, err := account.ConnectLoginSelect()
	if err != nil {
		return &Result{Error: err}
	}
	defer conn.LogoutDisconnect()
	totalNumber, err := conn.GetNumberMessages(account.MailboxName)
	if err != nil {
		return &Result{Error: err}
	}
	if skip >= totalNumber {
		return &Result{Error: fmt.Errorf("IMAPAccounts.ListMails: skip must be less than %d", totalNumber)}
	}
	// If count is greater than total number, retrieve all of the mails.
	if skip+count > totalNumber {
		count = totalNumber - skip
	}
	fromNum := totalNumber - count - skip + 1
	toNum := totalNumber - skip
	headers, err := conn.GetHeaders(fromNum, toNum)
	if err != nil {
		return &Result{Error: err}
	}
	var output bytes.Buffer
	for i := toNum; i >= fromNum; i-- {
		header, found := headers[i]
		if !found {
			continue
		}
		// Append \r\n\r\n to make it look like a complete message with empty body
		prop, _, err := inet.ReadMailMessage([]byte(header + "\r\n\r\n"))
		if err != nil {
			continue
		}
		output.WriteString(fmt.Sprintf("%d %s %s\n", i, prop.FromAddress, prop.Subject))
	}
	return &Result{Output: output.String()}
}

func (imap *IMAPAccounts) ReadMessage(cmd Command) *Result {
	// Find one string parameter and one numeric parameter among the content
	params := RegexMailboxAndNumber.FindStringSubmatch(cmd.Content)
	if len(params) < 3 {
		return &Result{Error: ErrBadMailboxParam}
	}
	var mbox string
	var number int
	mbox = params[1]
	var intErr error
	number, intErr = strconv.Atoi(params[2])
	if intErr != nil {
		return &Result{Error: ErrBadMailboxParam}
	}
	// Let IMAP magic begin!
	account, found := imap.Accounts[mbox]
	if !found {
		return &Result{Error: fmt.Errorf("IMAPAccounts.ReadMessage: cannot find mailbox \"%s\"", mbox)}
	}
	conn, err := account.ConnectLoginSelect()
	if err != nil {
		return &Result{Error: err}
	}
	defer conn.LogoutDisconnect()
	entireMessage, err := conn.GetMessage(number)
	if err != nil {
		return &Result{Error: err}
	}
	// If mail is multi-part, prefer to retrieve the plain text mail body.
	var anyText, plainText string
	err = inet.WalkMailMessage([]byte(entireMessage), func(prop inet.BasicMail, body []byte) (bool, error) {
		if !strings.Contains(prop.ContentType, "plain") {
			anyText = string(body)
		} else {
			plainText = string(body)
		}
		return true, nil
	})
	if err != nil {
		return &Result{Error: err}
	}
	if plainText == "" {
		return &Result{Output: anyText}
	} else {
		return &Result{Output: plainText}
	}
}

func (imap *IMAPAccounts) Execute(cmd Command) (ret *Result) {
	if errResult := cmd.Trim(); errResult != nil {
		return errResult
	}
	if cmd.FindAndRemovePrefix(MailboxList) {
		ret = imap.ListMails(cmd)
	} else if cmd.FindAndRemovePrefix(MailboxRead) {
		ret = imap.ReadMessage(cmd)
	} else {
		ret = &Result{Error: ErrBadMailboxParam}
	}
	return
}
