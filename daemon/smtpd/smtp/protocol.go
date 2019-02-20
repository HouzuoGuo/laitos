package smtp

import (
	"fmt"
	"strings"
	"unicode"
)

// ProtocolVerb is an enumeration of SMTP verbs supported by this server.
type ProtocolVerb int

const (
	VerbAbsent  ProtocolVerb = iota
	VerbUnknown ProtocolVerb = iota
	VerbHELO
	VerbEHLO
	VerbSTARTTLS
	VerbVRFY
	VerbMAILFROM
	VerbRCPTTO
	VerbDATA
	VerbQUIT
	VerbRSET
	VerbNOOP
)

// String returns a descriptive string representation of an SMTP Verb.
func (verb ProtocolVerb) String() string {
	switch verb {
	case VerbAbsent:
		return "(no command verb given)"
	case VerbUnknown:
		return "(unknown command Verb)"
	default:
		for _, cmd := range protocolCommands {
			if cmd.VerbEnum == verb {
				return cmd.VerbText
			}
		}
		return fmt.Sprintf("(unrecognised command Verb %d)", verb)
	}
}

// protocolCommand describes a decoded command received in an ongoing SMTP conversation with a client.
type protocolCommand struct {
	Verb      ProtocolVerb
	Parameter string
	ErrorInfo string
}

// verbParameterExpectation determines whether and what kind of parameter value is to be expected from an SMTP Verb.
type verbParameterExpectation int

const (
	expectOptionalParameter verbParameterExpectation = iota
	expectMailAddress
)

// protocolCommands is the comprehensive list of SMTP verbs and parameter expectations supported by this server.
var protocolCommands = []struct {
	VerbEnum             ProtocolVerb
	VerbText             string
	ParameterExpectation verbParameterExpectation
}{
	{VerbHELO, "HELO", expectOptionalParameter},
	{VerbEHLO, "EHLO", expectOptionalParameter},
	{VerbSTARTTLS, "STARTTLS", expectOptionalParameter},
	{VerbVRFY, "VRFY", expectOptionalParameter},
	{VerbMAILFROM, "MAIL FROM", expectMailAddress},
	{VerbRCPTTO, "RCPT TO", expectMailAddress},
	{VerbDATA, "DATA", expectOptionalParameter},
	{VerbQUIT, "QUIT", expectOptionalParameter},
	{VerbRSET, "RSET", expectOptionalParameter},
	{VerbNOOP, "NOOP", expectOptionalParameter},
}

// contains7BitAsciiOnly returns true only if the input byte array only contains byte value <=127.
func contains7BitAsciiOnly(input []byte) bool {
	for _, b := range input {
		if b > 127 {
			return false
		}
	}
	return true
}

// parseConversationCommand interprets an SMTP command during an ongoing conversation, and breaks it down into verb and parameter.
func parseConversationCommand(line string) (ret protocolCommand) {
	ret.Verb = VerbUnknown
	if !contains7BitAsciiOnly([]byte(line)) {
		ret.ErrorInfo = "verb contains non 7-bit ASCII"
		return
	}
	line = strings.TrimRightFunc(line, unicode.IsSpace)
	// Determine the verb used in the command
	verbEnum := -1
	capitalisedLine := strings.ToUpper(line)
	for i := range protocolCommands {
		if strings.HasPrefix(capitalisedLine, protocolCommands[i].VerbText) {
			verbEnum = i
			break
		}
	}
	if verbEnum == -1 {
		ret.ErrorInfo = "unrecognised verb"
		return ret
	}
	// Ensure the verb appears as a word, followed by proper word boundary.
	protCmd := protocolCommands[verbEnum]
	lengthLine := len(line)
	lengthVerb := len(protCmd.VerbText)
	if !(lengthLine == lengthVerb || line[lengthVerb] == ' ' || line[lengthVerb] == ':') {
		ret.ErrorInfo = "unrecognised verb"
		return ret
	}
	ret.Verb = protCmd.VerbEnum
	// Decode parameter
	switch protCmd.ParameterExpectation {
	case expectOptionalParameter:
		// The text trailing command verb is understood as the optional parameter
		if lengthLine > lengthVerb+1 {
			ret.Parameter = strings.TrimSpace(line[lengthVerb+1:])
		}
	case expectMailAddress:
		if lengthLine < lengthVerb+3 {
			ret.ErrorInfo = "missing mail address"
			return ret
		}
		// Mail address should look like <a@b.c>
		var indexAddressEnd int
		if line[lengthLine-1] == '>' {
			indexAddressEnd = lengthLine - 1
		} else {
			indexAddressEnd = strings.IndexByte(line, '>')
			if indexAddressEnd != -1 && line[indexAddressEnd+1] != ' ' {
				ret.ErrorInfo = "malformed mail address"
				return ret
			}
		}
		if line[lengthVerb] != ':' || indexAddressEnd == -1 {
			ret.ErrorInfo = "malformed mail address"
			return ret
		}
		indexAddressBegin := lengthVerb + 1
		if line[indexAddressBegin] == ' ' {
			indexAddressBegin++
		}
		if line[indexAddressBegin] != '<' {
			ret.ErrorInfo = "improper argument formatting"
			return ret
		}
		ret.Parameter = line[indexAddressBegin+1 : indexAddressEnd]
	}
	return ret
}
