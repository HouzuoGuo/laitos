package feature

import (
	"bufio"
	"bytes"
	"crypto/tls"
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

// Send an IMAP command and wait for a response, then return.
func (mbox *IMAPS) Converse(action string) (ok bool, status, body string, err error) {
	var allLines bytes.Buffer
	reader := bufio.NewReader(mbox.tlsConn)
	challenge := randomChallenge()

	mbox.tlsConn.SetDeadline(time.Now().Add(time.Duration(mbox.IOTimeoutSec) * time.Second))
	_, err = mbox.tlsConn.Write([]byte(fmt.Sprintf("%s %s\r\n", challenge, action)))
	if err != nil {
		// TLS internal state is surely corrupted after a timeout of write operation
		goto badConversation
	}
	for {
		var line []byte
		line, _, err = reader.ReadLine()
		fmt.Println("Read line ", string(line))
		if err != nil {
			/*
				Even though TLS internal state is not corrupted after a timeout of read operation,
				the IMAP conversation sequence is surely broken, hence close the connection.
			*/
			goto badConversation
		}
		lowerLine := strings.TrimSpace(strings.ToLower(string(line)))
		if strings.Index(lowerLine, challenge) == 0 {
			// Conversation is finished
			body = allLines.String()
			fmt.Println("Body is", body)
			withoutChallenge := strings.TrimSpace(lowerLine[len(challenge):])
			fmt.Println("Without challenge", withoutChallenge)
			// Status word is at the beginning
			afterStatusWord := strings.IndexRune(withoutChallenge, ' ')
			if afterStatusWord == -1 {
				status = withoutChallenge
				err = fmt.Errorf("Cannot find status word among line - %s", withoutChallenge)
				goto badConversation
			}
			statusWord := withoutChallenge[:afterStatusWord]
			fmt.Println("Status word", statusWord)
			if statusWord == "ok" {
				ok = true
			}
			if len(withoutChallenge) > afterStatusWord {
				status = withoutChallenge[afterStatusWord:]
			}
			return
		} else {
			// Continue to receive response body
			allLines.Write(line)
		}
	}
	return
badConversation:
// Close connection so that no further conversation may take place
	mbox.tlsConn.Close()
	mbox.conn.Close()
	return
}

// Get number of messages in the mail box.
func (mbox *IMAPS) GetNumberMessages() (int, error) {
	ok, status, body, err := mbox.Converse(fmt.Sprintf("STATUS %s (MESSAGES)", mbox.MailboxName))
	if err != nil {
		return 0, err
	} else if !ok {
		return 0, fmt.Errorf("STATUS command failed - %s", status)
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

// Set up TLS connection to IMAPS server and log the user in.
func (mbox *IMAPS) ConnectLogin() (err error) {
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
	ok, status, _, err := mbox.Converse(fmt.Sprintf("LOGIN %s %s", mbox.AuthUsername, mbox.AuthPassword))
	if err != nil {
		return fmt.Errorf("Failed to send login request - %v", err)
	} else if !ok {
		return fmt.Errorf("LOGIN command failed - %s", status)
	}
	return
}

func (mbox *IMAPS) DisconnectLogout() error {
	mbox.Converse("LOGOUT")
}

func main() {
	mbox := IMAPS{
		Host:               "A",
		Port:               993,
		InsecureSkipVerify: true,
		AuthUsername:       "B",
		AuthPassword:       "C",
		IOTimeoutSec:       10,
	}
	if err := mbox.ConnectLogin(); err != nil {
		log.Fatal(err)
	}

	conn, err := net.Dial("tcp", "A:993")
	if err != nil {
		log.Fatal(err)
	}
	tlsConn := tls.Client(conn, &tls.Config{InsecureSkipVerify: true})
	if err := tlsConn.Handshake(); err != nil {
		log.Fatal(err)
	}
	go func() {
		reader := bufio.NewReader(tlsConn)
		for {
			line, isPrefix, err := reader.ReadLine()
			if err != nil {
				log.Fatal("Read", err)
			}
			fmt.Printf("%v %s\n", isPrefix, line)
		}
	}()
	writer := bufio.NewWriter(tlsConn)
	time.Sleep(1 * time.Second)
	fmt.Println("LOGIN")
	if _, err := writer.WriteString("9223372036854775808 LOGIN A B\r\n"); err != nil {
		log.Fatal(err)
	}
	writer.Flush()
	time.Sleep(1 * time.Second)
	fmt.Println("SELECT")
	if _, err := writer.WriteString("? SELECT INBOX\r\n"); err != nil {
		log.Fatal(err)
	}
	writer.Flush()
	time.Sleep(1 * time.Second)
	fmt.Println("FETCH")
	if _, err := writer.WriteString("? FETCH 1:2 BODY.PEEK[HEADER]\r\n"); err != nil {
		log.Fatal(err)
	}
	writer.Flush()
	time.Sleep(1 * time.Second)
	fmt.Println("FETCH BODY")
	if _, err := writer.WriteString("? FETCH 1:2 BODY[TEXT]\r\n"); err != nil {
		log.Fatal(err)
	}
	time.Sleep(1 * time.Second)
	fmt.Println("CLOSE")
	if _, err := writer.WriteString("? CLOSE INBOX\r\n"); err != nil {
		log.Fatal(err)
	}
	writer.Flush()
	time.Sleep(1 * time.Second)
	fmt.Println("LOGOUT")
	if _, err := writer.WriteString("? LOGOUT\r\n"); err != nil {
		log.Fatal(err)
	}
	writer.Flush()
	fmt.Println("Finished and waiting")
	time.Sleep(2 * time.Second)
}

// Correspond IMAP account connection details to account names.
type IMAPAccounts struct {
	Accounts map[string]*IMAPS `json:"Accounts"` // IMAP account name vs account connectivity details
}

var TestIMAPAccounts = IMAPAccounts{} // Account details are set by init_feature_test.go

func (imap *IMAPAccounts) IsConfigured() bool {
	return imap.Accounts != nil && len(imap.Accounts) > 0
}

func (imap *IMAPAccounts) SelfTest() error {
	if !imap.IsConfigured() {
		return ErrIncompleteConfig
	}
	for name, account := range imap.Accounts {
		if err := account.ConnectLogin(); err != nil {
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
		if err := account.ConnectLogin(); err != nil {
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
