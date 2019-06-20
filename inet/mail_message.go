package inet

import (
	"bytes"
	"github.com/HouzuoGuo/laitos/misc"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"regexp"
	"strings"
)

const (
	/*
		MaxMailBodySize is the maximum size of a single mail message acceptable by popular Internet email services.
		The number defined here is slightly more generous than the norm.
	*/
	MaxMailBodySize = 32 * 1048576
)

// RegexMailAddress finds *@*.* that looks much like an Email address
var RegexMailAddress = regexp.MustCompile(`[a-zA-Z0-9!#$%&'*+-/=?_{|}~.^]+@[a-zA-Z0-9!#$%&'*+-/=?_{|}~.^]+.[a-zA-Z0-9!#$%&'*+-/=?_{|}~.^]+`)

/*
Mail properties such as subject and content type.
If the mail is a multi-part mail, the ContentType string will be able to tell the correct content type to a multipart reader.
*/
type BasicMail struct {
	Subject      string // Mail subject
	FromAddress  string // From address of mail, minus person's name.
	ReplyAddress string // Address to which a reply to this mail shall be delivered
	ContentType  string // Mail content type
}

// Parse headers of the mail message and return some basic properties about the mail.
func ReadMailMessage(mailMessage []byte) (prop BasicMail, parsedMail *mail.Message, err error) {
	if len(mailMessage) > MaxMailBodySize {
		mailMessage = mailMessage[:MaxMailBodySize]
	}
	// Retrieve headers using standard library function
	parsedMail, err = mail.ReadMessage(bytes.NewReader(mailMessage))
	if err != nil {
		return
	}
	prop.Subject = strings.TrimSpace(parsedMail.Header.Get("Subject"))
	prop.ContentType = strings.TrimSpace(parsedMail.Header.Get("Content-Type"))
	// Extract mail address using regex
	if fromAddr := RegexMailAddress.FindString(parsedMail.Header.Get("From")); fromAddr != "" {
		prop.FromAddress = strings.TrimSpace(fromAddr)
	}
	if replyAddr := RegexMailAddress.FindString(parsedMail.Header.Get("Reply-To")); replyAddr != "" {
		prop.ReplyAddress = strings.TrimSpace(replyAddr)
	}
	if prop.ReplyAddress == "" {
		// If there is no Reply-To address, use From address instead.
		prop.ReplyAddress = strings.TrimSpace(prop.FromAddress)
	}
	return
}

/*
WalkMailMessage dissects input mail message:
If input message is a multipart message, run the function against each part individually.
If input message is not a multipart mail message, run the function against the entire message.

As a special case, should the input mail message be encoded as "quoted-printable", the mail body
will be rid of the quotes before passing into the function.

The function parameters are:
MailProperties - properties of the entire mail message or part of multipart message.
[]byte - body of current mail part.

The function returns two parameters:
bool - if true, continue processing the next part, otherwise cease processing.
error - stop processing and return this error immediately.
*/
func WalkMailMessage(mailMessage []byte, fun func(BasicMail, []byte) (bool, error)) error {
	prop, parsedMail, err := ReadMailMessage(mailMessage)
	if err != nil {
		return err
	}
	mediaType, multipartParams, err := mime.ParseMediaType(prop.ContentType)
	if err != nil {
		return err
	}
	if strings.HasPrefix(mediaType, "multipart/") {
		// Walk through each part individually
		partReader := multipart.NewReader(parsedMail.Body, multipartParams["boundary"])
		for {
			part, err := partReader.NextPart()
			// Stop at the end of all parts
			if err == io.EOF {
				return nil
			} else if err != nil {
				return err
			}
			// For the convenience of consumer, process quoted text and remove those quotes.
			var contentReader io.Reader = part
			if strings.Contains(part.Header.Get("Content-Transfer-Encoding"), "quoted-printable") {
				contentReader = quotedprintable.NewReader(contentReader)
			}
			// Read body of the current part
			body, err := misc.ReadAllUpTo(contentReader, MaxMailBodySize)
			if err != nil {
				return err
			}
			// Invoke function with properties of the current part
			partProp := prop
			partProp.ContentType = part.Header.Get("Content-Type")
			next, err := fun(partProp, body)
			if err != nil {
				return err
			}
			// Stop processing further parts if the function return value asks so
			if !next {
				return nil
			}
		}
	} else {
		// Use the entire message on function
		// For the convenience of consumer, process quoted text and remove those quotes.
		var contentReader io.Reader = parsedMail.Body
		if strings.Contains(parsedMail.Header.Get("Content-Transfer-Encoding"), "quoted-printable") {
			contentReader = quotedprintable.NewReader(contentReader)
		}
		body, err := misc.ReadAllUpTo(contentReader, MaxMailBodySize)
		if err != nil {
			return err
		}
		_, err = fun(prop, body)
		return err
	}
}
