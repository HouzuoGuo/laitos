/*
smtp package implements a rather forgiving TCP server that carries on and decodes an SMTP conversation.
I would like to express my gratitude to Chris Siebenmann for his inspiring pioneer work on an implementation of SMTP
server written in go.
*/
package smtp

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
)

/*
MaxCommandLength is the maximum acceptable length of a command in the middle of an ongoing SMTP conversation.
The maximum length does not apply to mail message and attachments.
*/
const MaxCommandLength = 4096

/*
commandStage is an enumeration of stages of an SMTP conversation. The stages determine what kind of protocol verbs are
anticipated for the upcoming protocol command.
*/
type commandStage int

const (
	StageGreeting      commandStage = iota
	StageAfterGreeting commandStage = 1 << iota
	StageHello
	StateMailAddress
	StageRecipient
	StageMessageData
	StageAfterMessageData
	StageQuit
	StageAbort
)

/*
verbExpectation describes stages that are applicable to an SMTP protocol verb, as well as whether the verb provides
transition onto the next stage of SMTP conversation.
*/
type verbExpectation struct {
	ValidInStages commandStage
	NextStage     commandStage
}

/*
stageExpectations is the comprehensive mapping of SMTP verbs and conversation stages in which the verbs are expected
to appear, as well as how they move the ongoing conversation to a subsequent stage.
*/
var stageExpectations = map[ProtocolVerb]verbExpectation{
	VerbHELO:     {ValidInStages: StageAfterGreeting | StageHello, NextStage: StageHello},
	VerbEHLO:     {ValidInStages: StageAfterGreeting | StageHello, NextStage: StageHello},
	VerbMAILFROM: {ValidInStages: StageHello, NextStage: StateMailAddress},
	VerbRCPTTO:   {ValidInStages: StateMailAddress | StageRecipient, NextStage: StageRecipient},
	VerbDATA:     {ValidInStages: StageRecipient, NextStage: StageMessageData},
}

// Config provides behaviour and fault tolerance tuning to SMTP conversation connection.
type Config struct {
	// TLSConfig grants SMTP server StartTLS capability.
	TLSConfig *tls.Config
	// IOTimeout governs the timeout of each read and write operation.
	IOTimeout time.Duration
	/*
		MaxMessageLength is the maximum size (in bytes) of a mail message (and attachment) during an SMTP conversation.
		An ordinary protocol command that does not carry mail message nor attachment uses MaxCommandLength constant
		instead.
	*/
	MaxMessageLength int64
	/*
		MaxConsecutiveUnrecognisedCommands is the maximum number of unknown protocol commands to tolerate before giving
		up on the connection, which will result in the connection being closed.
	*/
	MaxConsecutiveUnrecognisedCommands int
	/*
		ServerName is the complete Internet host name of the mail server, it is used to greet mail clients. Some clients
		use the greeting to further establish authenticity of the mail server.
	*/
	ServerName string
}

/*
Connection is the server side of an SMTP connection and it offers functions for caller to interact with an SMTP
conversation, and eventually acquire the complete mail message prior to the conclusion of the connection.
*/
type Connection struct {
	// Config is supplied by caller.
	Config Config

	// TLSAttempted is an indication flag to inform caller that StartTLS has been attempted.
	TLSAttempted bool
	// TLSState helps caller to debug TLS connection issue.
	TLSState tls.ConnectionState
	// TLSHelp contains a text description that explains the latest TLS error from SMTP conversation's perspective.
	TLSHelp string

	// netConn is the underlying TCP connection
	netConn net.Conn
	// limitReader is the underlying reader used to read both simple protocol commands and mail message body.
	limitReader *io.LimitedReader
	// textReader is used on top of limitReader to interpret command content from SMTP-specific encoding
	textReader *textproto.Reader
	// consecutiveUnrecognisedCommands counts the number of consecutive unrecognised commands.
	consecutiveUnrecognisedCommands int
	// latestProtocolVerb is the protocol verb received from the latest protocol command.
	latestProtocolVerb ProtocolVerb
	// state memorises the latest stage of the ongoing SMTP conversation.
	stage commandStage
	// expectNextStage is the upcoming stage of the ongoing SMTP conversation.
	expectNextStage commandStage
	// answered is set to true after the server has successfully replied to the latest command.
	answered bool
	logger   lalog.Logger
}

/*
ConversationState represents the latest state of the ongoing SMTP conversation, determined by the latest series of
protocol commands. Caller may use the state to determine whether the conversation is carrying on, and whether mail
message has been received completely.
*/
type ConversationState int

const (
	_                   ConversationState = iota
	ConvReceivedCommand ConversationState = iota
	ConvReceivedData
	ConvCompleted
	ConvAborted
)

/*
Command comes from the result of interpretation of latest protocol command in an ongoing conversation. In addition to
stating whether the SMTP conversation is carrying on, it also records the latest protocol command verb and parameter
value.
*/
type Command struct {
	State     ConversationState
	Verb      ProtocolVerb
	Parameter string
}

// readCommand reads an SMTP command, the command anticipated must not contain mail data.
func (conn *Connection) readCommand() (cmd string) {
	conn.limitReader.N = MaxCommandLength
	conn.logger.MaybeMinorError(conn.netConn.SetReadDeadline(time.Now().Add(conn.Config.IOTimeout)))
	cmd, err := conn.textReader.ReadLine()
	if err != nil || conn.limitReader.N == 0 {
		conn.stage = StageAbort
	}
	return
}

// readMailData reads mail data that arrives in dot-encoding.
func (conn *Connection) readMailData() string {
	conn.limitReader.N = conn.Config.MaxMessageLength
	conn.logger.MaybeMinorError(conn.netConn.SetReadDeadline(time.Now().Add(conn.Config.IOTimeout)))
	decodedBytes, err := conn.textReader.ReadDotBytes()
	if err != nil || conn.limitReader.N == 0 {
		conn.stage = StageAbort
		return ""
	}
	return string(decodedBytes)
}

/*
reply is used internally to reply to a protocol command that has just been received. Should any error occurs during the
write operation, the SMTP conversation will no longer go on.
*/
func (conn *Connection) reply(format string, a ...interface{}) {
	if conn.stage != StageAbort {
		conn.logger.MaybeMinorError(conn.netConn.SetWriteDeadline(time.Now().Add(conn.Config.IOTimeout)))
		_, err := conn.netConn.Write([]byte(fmt.Sprintf(format+"\r\n", a...)))
		if err != nil {
			conn.stage = StageAbort
		}
	}
}

/*
answerOK produces an positive reply appropriate to the stage of SMTP conversation, and move the conversation onward to
the next stage.
*/
func (conn *Connection) answerOK() {
	if conn.answered {
		return
	}
	switch conn.latestProtocolVerb {
	case VerbHELO:
		conn.reply("250 %s", conn.Config.ServerName)
	case VerbEHLO:
		conn.reply("250-%s", conn.Config.ServerName)
		conn.reply("250-8BITMIME")
		conn.reply("250-PIPELINING")
		if conn.Config.TLSConfig != nil && !conn.TLSAttempted {
			conn.reply("250-STARTTLS")
		}
		conn.reply("250 OK")
	case VerbMAILFROM:
		conn.reply("250 2.1.0 OK")
	case VerbRCPTTO:
		conn.reply("250 2.1.5 OK")
	case VerbDATA:
		if conn.stage == StageRecipient {
			conn.reply("354 Start mail input; end with <CRLF>.<CRLF>")
		} else {
			conn.reply("250 2.0.0 OK")
		}
	default:
		conn.reply("250 OK")
	}
	conn.stage = conn.expectNextStage
	conn.answered = true
}

// AnswerNegative produces a negative reply appropriate for the stage of SMTP conversation.
func (conn *Connection) AnswerNegative() {
	switch conn.latestProtocolVerb {
	case VerbHELO, VerbEHLO:
		conn.reply("550 Not accepted")
	case VerbMAILFROM, VerbRCPTTO:
		conn.reply("550 Bad address")
	case VerbDATA:
		conn.reply("554 Not accepted")
	}
	conn.answered = true
}

/*
AnswerRateLimited produces a negative answer to the SMTP conversation to inform SMTP client that it has been rate
limited. The connection is closed afterwards.
*/
func (conn *Connection) AnswerRateLimited() {
	conn.reply("451 Try again later rate limit exceeded or too many conversations")
	conn.answered = true
	conn.stage = StageAbort
}

// setupReaders initialises text reader and limit reader to operate on the underlying network connection.
func (conn *Connection) setupReaders(netConn net.Conn) {
	conn.netConn = netConn
	conn.limitReader = io.LimitReader(netConn, MaxCommandLength).(*io.LimitedReader)
	conn.textReader = textproto.NewReader(bufio.NewReader(conn.limitReader))
}

/*
CarryOn continues the SMTP conversation until the next stage is reached, at which point the latest command (such as mail
address or mail data) is returned to caller.
*/
func (conn *Connection) CarryOn() Command {
	var latestCmd Command

	// Respond to the latest command
	if !conn.answered && conn.latestProtocolVerb != VerbAbsent {
		conn.answerOK()
	}

	// For a newly established conversation, send the server greeting.
	if conn.stage == StageGreeting {
		conn.reply("220 %s ESMTP", conn.Config.ServerName)
		conn.stage = StageAfterGreeting
	} else if conn.stage == StageMessageData {
		/*
			For an already established conversation, reach the stage at which mail data has been received completely,
			and return the received data to caller.
		*/
		data := conn.readMailData()
		if len(data) > 0 {
			latestCmd.State = ConvReceivedData
			latestCmd.Parameter = data
			conn.answered = false
			conn.stage = StageAfterMessageData
			conn.expectNextStage = StageHello
			return latestCmd
		}
	}

	// Carry on with the conversation until it reaches the next stage
	for {
		if conn.stage == StageAbort || conn.stage == StageQuit || conn.consecutiveUnrecognisedCommands >= conn.Config.MaxConsecutiveUnrecognisedCommands {
			break
		}
		cmdLine := conn.readCommand()
		if cmdLine == "" {
			break
		}
		thisCmd := parseConversationCommand(cmdLine)
		if thisCmd.Verb == VerbUnknown {
			conn.consecutiveUnrecognisedCommands++
			/*
				According to RFC, status 500 is appropriate for a verb unknown to the server, while status 502 indicate
				that a verb is recognised but not implemented. Over here, the server tries hard to be very forgiving.
			*/
			conn.reply("502 Command not implemented")
			continue
		}
		// If verb is OK, then parser error can only be a case of malformed mail address, hence status code 553.
		if len(thisCmd.ErrorInfo) > 0 {
			conn.reply("553 %s", thisCmd.ErrorInfo)
			continue
		}
		// Move the conversation onward to the next stage (if any)
		verbStage := stageExpectations[thisCmd.Verb]
		if verbStage.ValidInStages != 0 && (verbStage.ValidInStages&conn.stage) == 0 {
			conn.reply("503 Bad sequence of commands")
			continue
		}
		if verbStage.ValidInStages == 0 {
			switch thisCmd.Verb {
			case VerbRSET:
				if conn.stage != StageAfterGreeting {
					conn.stage = StageHello
				}
				conn.reply("250 OK")
			case VerbNOOP:
				conn.reply("250 OK")
			case VerbVRFY:
				conn.reply("252 OK")
			case VerbQUIT:
				conn.stage = StageQuit
				conn.reply("221 2.0.0 Bye")
			case VerbSTARTTLS:
				if conn.Config.TLSConfig == nil || conn.TLSAttempted {
					conn.reply("502 Command not implemented")
					conn.TLSHelp = "client asked but this server does not support TLS"
					continue
				}
				conn.reply("220 Ready to start TLS")
				if conn.stage == StageAbort {
					conn.TLSHelp = "connection aborted before negotiation"
					break
				}
				conn.logger.MaybeMinorError(conn.netConn.SetDeadline(time.Now().Add(conn.Config.IOTimeout)))
				tlsConn := tls.Server(conn.netConn, conn.Config.TLSConfig)
				err := tlsConn.Handshake()
				if err == nil {
					// Upon successful handshake, substitute the underlying connection by the new TLS connection.
					conn.TLSHelp = "handshake was successful"
					conn.logger.MaybeMinorError(conn.netConn.SetReadDeadline(time.Time{}))
					conn.setupReaders(tlsConn)
					conn.TLSAttempted = true
					conn.TLSState = tlsConn.ConnectionState()
					conn.stage = StageAfterGreeting
				} else {
					conn.TLSHelp = "handshake failure - " + err.Error()
					latestCmd.Parameter = fmt.Sprintf("%v", err)
					conn.reply("454 TLS handshake failure")
					// No way to carry on from a broken handshake
					conn.stage = StageAbort
					break
				}
			default:
				conn.consecutiveUnrecognisedCommands++
				conn.reply("502 Command not implemented")
			}
			continue
		}

		conn.expectNextStage = verbStage.NextStage
		conn.answered = false
		conn.latestProtocolVerb = thisCmd.Verb

		latestCmd.State = ConvReceivedCommand
		latestCmd.Verb = thisCmd.Verb
		latestCmd.Parameter = thisCmd.Parameter
		return latestCmd
	}

	latestCmd.Parameter = ""
	if conn.consecutiveUnrecognisedCommands > conn.Config.MaxConsecutiveUnrecognisedCommands {
		latestCmd.Parameter = "too many unknown commands"
		conn.reply("554 Too many unknown commands")
		conn.stage = StageAbort
	}
	if conn.stage == StageQuit {
		latestCmd.State = ConvCompleted
	}
	return latestCmd
}

func NewConnection(conn net.Conn, cfg Config, log io.Writer) *Connection {
	c := &Connection{stage: StageGreeting, Config: cfg, TLSHelp: "not used"}
	c.setupReaders(conn)
	if c.Config.MaxConsecutiveUnrecognisedCommands < 1 || c.Config.MaxMessageLength < 1 || c.Config.IOTimeout < 1 {
		panic("missing configuration of protocol limits")
	}
	if c.Config.ServerName == "" {
		panic("server name configuration must not be empty")
	}
	return c
}
