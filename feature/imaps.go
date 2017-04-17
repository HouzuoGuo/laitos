package feature

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/email"
	"log"
	"math/rand"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	MailboxList = "l" // Prefix string to trigger listing messages
	MailboxRead = "r" // Prefix string to trigger reading message body
)

var (
	RegexMailboxAndNumber     = regexp.MustCompile(`(\w+)[^\w]+(\d+)`)            // Capture one mailbox shortcut name and a number
	RegexMailboxAndTwoNumbers = regexp.MustCompile(`(\w+)[^\w]+(\d+)[^\d]+(\d+)`) // Capture one mailbox shortcut name and two numbers
	ErrBadMailboxParam        = fmt.Errorf("Example: %s box skip# count# | %s box to-read#", MailboxList, MailboxRead)
)

// Retrieve emails via IMAPS.
type IMAPS struct {
	Host               string `json:"Host"`               // Server name or IP address of IMAPS server
	Port               int    `json:"Port"`               // Port number of IMAPS service
	MailboxName        string `json:"MailboxName"`        // Name of mailbox (e.g. "INBOX")
	InsecureSkipVerify bool   `json:"InsecureSkipVerify"` // Do not verify server name against its certificate
	AuthUsername       string `json:"AuthUsername"`       // Username for plain authentication
	AuthPassword       string `json:"AuthPassword"`       // Password for plain authentication
	IOTimeoutSec       int    `json:"IOTimeoutSec"`       // Default IO conversation timeout in seconds

	conn    net.Conn  `json:"-"`
	tlsConn *tls.Conn `json:"-"`
}

// Return a random 10 characters long string of numbers to
func randomChallenge() string {
	return strconv.Itoa(1000000000 + rand.Intn(1000000000))
}

// Send an IMAP command and wait for a response, then return. If response status is not OK, an error is returned.
func (mbox *IMAPS) Converse(action string) (status, body string, err error) {
	var allLines bytes.Buffer
	reader := bufio.NewReader(mbox.tlsConn)
	challenge := randomChallenge()

	mbox.tlsConn.SetDeadline(time.Now().Add(time.Duration(mbox.IOTimeoutSec) * time.Second))
	_, err = mbox.tlsConn.Write([]byte(fmt.Sprintf("%s %s\r\n", challenge, action)))
	if err != nil {
		// TLS internal state is surely corrupted after a timeout of write operation
		goto badIO
	}
	for {
		var line []byte
		line, _, err = reader.ReadLine()
		if err != nil {
			/*
				Even though TLS internal state is not corrupted after a timeout of read operation,
				the IMAP conversation sequence is surely broken, hence close the connection.
			*/
			goto badIO
		}
		lowerLine := strings.TrimSpace(strings.ToLower(string(line)))
		if strings.Index(lowerLine, challenge) == 0 {
			// Conversation is finished
			body = allLines.String()
			withoutChallenge := strings.TrimSpace(lowerLine[len(challenge):])
			// Status word is at the beginning
			afterStatusWord := strings.IndexRune(withoutChallenge, ' ')
			if afterStatusWord == -1 {
				status = withoutChallenge
				err = fmt.Errorf("IMAPS.Converse: cannot find status word among line - %s", withoutChallenge)
				goto badIO
			}
			statusWord := withoutChallenge[:afterStatusWord]
			if len(withoutChallenge) > afterStatusWord {
				status = withoutChallenge[afterStatusWord:]
			}
			if statusWord != "ok" {
				err = fmt.Errorf("IMAPS.Converse: bad response status - %s", status)
			}
			return
		} else {
			// Continue to receive response body
			allLines.Write(line)
			allLines.WriteRune('\n')
		}
	}
	return
badIO:
	// Close connection so that no further conversation may take place
	mbox.tlsConn.Close()
	mbox.conn.Close()
	return
}

// Get total number of messages in the mail box.
func (mbox *IMAPS) GetNumberMessages() (int, error) {
	_, body, err := mbox.Converse(fmt.Sprintf("EXAMINE \"%s\"", mbox.MailboxName))
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

// Retrieve mail header from specified message number range.
func (mbox *IMAPS) GetHeaders(from, to int) (ret map[int]string, err error) {
	ret = make(map[int]string)
	if from > to || from < 1 || to < 1 {
		err = errors.New("From number must be less or equal to To number, and both must be positive.")
		return
	}
	_, body, err := mbox.Converse(fmt.Sprintf("FETCH %d:%d BODY.PEEK[HEADER]", from, to))
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

// Retrieve an entire mail message including header and body.
func (mbox *IMAPS) GetMessage(num int) (message string, err error) {
	if num < 1 {
		err = errors.New("Message number must be positive")
		return
	}
	var entireMessage bytes.Buffer
	_, body, err := mbox.Converse(fmt.Sprintf("FETCH %d BODY[]", num))
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

// Set up TLS connection to IMAPS server and log the user in.
func (mbox *IMAPS) ConnectLoginSelect() (err error) {
	mbox.conn, err = net.DialTimeout(
		"tcp",
		fmt.Sprintf("%s:%d", mbox.Host, mbox.Port),
		time.Duration(mbox.IOTimeoutSec)*time.Second)
	if err != nil {
		return fmt.Errorf("IMAPS.ConnectLoginSelect: connection error - %v", err)
	}
	mbox.tlsConn = tls.Client(mbox.conn, &tls.Config{
		ServerName:         mbox.Host,
		InsecureSkipVerify: mbox.InsecureSkipVerify,
	})
	if err = mbox.tlsConn.Handshake(); err != nil {
		return fmt.Errorf("IMAPS.ConnectLoginSelect: TLS connection error - %v", err)
	}
	// Absorb the connection greeting message sent by server
	reader := bufio.NewReader(mbox.tlsConn)
	_, _, err = reader.ReadLine()
	if err != nil {
		return fmt.Errorf("IMAPS.ConnectLoginSelect: failed to read server greeting - %v", err)
	}
	// LOGIN && SELECT
	_, _, err = mbox.Converse(fmt.Sprintf("LOGIN %s %s", mbox.AuthUsername, mbox.AuthPassword))
	if err != nil {
		return fmt.Errorf("IMAPS.ConnectLoginSelect: LOGIN command failed - %v", err)
	}
	_, _, err = mbox.Converse(fmt.Sprintf("SELECT \"%s\"", mbox.MailboxName))
	if err != nil {
		return fmt.Errorf("IMAPS.ConnectLoginSelect: SELECT command failed - %v", err)
	}
	return
}

func (mbox *IMAPS) DisconnectLogout() {
	if _, _, err := mbox.Converse("LOGOUT"); err != nil {
		log.Printf("IMAPS.DisconnectLogout: LOGOUT command failed - %v", err)
	}
	mbox.tlsConn.Close()
	mbox.conn.Close()
}

// Correspond IMAP account connection details to account names.
type IMAPAccounts struct {
	Accounts map[string]IMAPS `json:"Accounts"` // IMAP account name vs account connectivity details
}

var TestIMAPAccounts = IMAPAccounts{} // Account details are set by init_feature_test.go

func (imap *IMAPAccounts) IsConfigured() bool {
	if imap.Accounts == nil || len(imap.Accounts) == 0 {
		return false
	}
	for _, account := range imap.Accounts {
		if account.Host == "" || account.Port == 0 || account.MailboxName == "" {
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
		if err := account.ConnectLoginSelect(); err != nil {
			return fmt.Errorf("IMAPAccounts.SelfTest: account \"%s\" has connection error - %v", name, err)
		}
		defer account.DisconnectLogout()
		if _, err := account.GetNumberMessages(); err != nil {
			return fmt.Errorf("IMAPAccounts.SelfTest: account \"%s\" test error - %v", name, err)
		}
	}
	return nil
}

func (imap *IMAPAccounts) Initialise() error {
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
		return &Result{Error: fmt.Errorf("IMAPAccounts.ListMails: cannot find box \"%s\"", mbox)}
	}
	account.IOTimeoutSec = cmd.TimeoutSec // this is nowhere near good enough, but how to make it accurate and race-free?
	if err := account.ConnectLoginSelect(); err != nil {
		return &Result{Error: err}
	}
	defer account.DisconnectLogout()
	totalNumber, err := account.GetNumberMessages()
	if err != nil {
		return &Result{Error: err}
	}
	if skip+count > totalNumber {
		return &Result{Error: fmt.Errorf("IMAPAccounts.ListMails: skip+count should be less or equal to %d", totalNumber)}
	}
	fromNum := totalNumber - count - skip + 1
	toNum := totalNumber - skip
	headers, err := account.GetHeaders(fromNum, toNum)
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
		prop, _, err := email.ReadMessage([]byte(header + "\r\n\r\n"))
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
		return &Result{Error: fmt.Errorf("IMAPAccounts.ReadMessage: cannot find box \"%s\"", mbox)}
	}
	account.IOTimeoutSec = cmd.TimeoutSec // this is nowhere near good enough, but how to make it accurate and race-free?
	if err := account.ConnectLoginSelect(); err != nil {
		return &Result{Error: err}
	}
	defer account.DisconnectLogout()
	entireMessage, err := account.GetMessage(number)
	if err != nil {
		return &Result{Error: err}
	}
	// If mail is multi-part, prefer to retrieve the plain text mail body.
	var anyText, plainText string
	err = email.WalkMessage([]byte(entireMessage), func(prop email.BasicProperties, body []byte) (bool, error) {
		if strings.Index(prop.ContentType, "plain") == -1 {
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
