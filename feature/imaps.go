package feature

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Retrieve emails via IMAPS.
type IMAPS struct {
	Host               string `json:"Host"`               // Server name or IP address of IMAPS server
	Port               int    `json:"Port"`               // Port number of IMAPS service
	MailboxName        string `json:"MailboxName"`        // Name of mailbox (e.g. "INBOX")
	InsecureSkipVerify bool   `json:"InsecureSkipVerify"` // Do not verify server name against its certificate
	AuthUsername       string `json:"AuthUsername"`       // Username for plain authentication
	AuthPassword       string `json:"AuthPassword"`       // Password for plain authentication
	IOTimeoutSec       int    `json:"IOTimeoutSec"`       //

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
				err = fmt.Errorf("Cannot find status word among line - %s", withoutChallenge)
				goto badIO
			}
			statusWord := withoutChallenge[:afterStatusWord]
			if len(withoutChallenge) > afterStatusWord {
				status = withoutChallenge[afterStatusWord:]
			}
			if statusWord != "ok" {
				err = fmt.Errorf("Bad status - %s", status)
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

// Get number of messages in the mail box.
func (mbox *IMAPS) GetNumberMessages() (int, error) {
	_, body, err := mbox.Converse(fmt.Sprintf("STATUS %s (MESSAGES)", mbox.MailboxName))
	if err != nil {
		return 0, err
	}
	// Extract number of messages from response body
	numberString := regexp.MustCompile(`\d+`).FindString(body)
	if numberString == "" {
		return 0, fmt.Errorf("STATUS command did not return a number in body - %s", body)
	}
	number, err := strconv.Atoi(numberString)
	if err != nil || number < 0 {
		return 0, fmt.Errorf("STATUS command did not return a valid positive integer in \"%s\" - %v", numberString, err)
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
	// Fetch the entire header section
	headerSection, err := mbox.GetHeaders(num, num)
	if err != nil {
		return
	}
	entireHeader, found := headerSection[num]
	if !found {
		err = fmt.Errorf("Failed to retrieve message %d header", num)
		return
	}
	// There is a "\r\n" in between header and body in a full message
	var entireMessage bytes.Buffer
	entireMessage.WriteString(entireHeader)
	entireMessage.WriteString("\r\n")
	_, body, err := mbox.Converse(fmt.Sprintf("FETCH %d BODY[TEXT]", num))
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
		return fmt.Errorf("Connection error - %v", err)
	}
	mbox.tlsConn = tls.Client(mbox.conn, &tls.Config{
		ServerName:         mbox.Host,
		InsecureSkipVerify: mbox.InsecureSkipVerify,
	})
	if err = mbox.tlsConn.Handshake(); err != nil {
		return fmt.Errorf("TLS connection error - %v", err)
	}
	// Absorb the connection greeting message sent by server
	reader := bufio.NewReader(mbox.tlsConn)
	_, _, err = reader.ReadLine()
	if err != nil {
		return fmt.Errorf("Failed to read server greeting - %v", err)
	}
	// LOGIN && SELECT
	_, _, err = mbox.Converse(fmt.Sprintf("LOGIN %s %s", mbox.AuthUsername, mbox.AuthPassword))
	if err != nil {
		return fmt.Errorf("LOGIN command failed - %v", err)
	}
	_, _, err = mbox.Converse(fmt.Sprintf("SELECT %s", mbox.MailboxName))
	if err != nil {
		return fmt.Errorf("SELECT command failed - %v", err)
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
	Accounts map[string]*IMAPS `json:"Accounts"` // IMAP account name vs account connectivity details
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
			return fmt.Errorf("Account \"%s\" connection error - %v", name, err)
		}
		defer account.DisconnectLogout()
		if _, err := account.GetNumberMessages(); err != nil {
			return fmt.Errorf("Account \"%s\" test error - %v", name, err)
		}
	}
	return nil
}

func (imap *IMAPAccounts) Initialise() error {
	for name, account := range imap.Accounts {
		if err := account.ConnectLoginSelect(); err != nil {
			return fmt.Errorf("Account \"%s\" connection error - %v", name, err)
		}
	}
	return nil
}

func (imap *IMAPAccounts) Trigger() Trigger {
	return ".i"
}

func (imap *IMAPAccounts) Execute(cmd Command) *Result {
	return nil
}
