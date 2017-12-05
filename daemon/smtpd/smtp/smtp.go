package smtp

/*
smtp.go together with smtp_test.go are forked from Chris Siebenmann's smtpd (https://github.com/siebenmann/smtpd) at
commit e0fc53b0ac440fd167960c1d5bfe9095387b9893 ("README: updated because cmd/* isn't there any more"), that carries the
following license information:
====================
CREDITS

Chris Siebenmann https://github.com/siebenmann
started writing this.

COPYRIGHT:
GPL v3 for now
====================

The fork remove unnecessary features such as authentication and processing of several non-operational commands.
*/
import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"strings"
	"time"
	"unicode"
)

// Command represents known SMTP commands in encoded form.
type Command int

// Recognized SMTP commands. Not all of them do anything.
const (
	noCmd  Command = iota // artificial zero value
	BadCmd Command = iota
	HELO
	EHLO
	MAILFROM
	RCPTTO
	DATA
	QUIT
	RSET
	STARTTLS
	NOOP
	VRFY
)

// ParsedLine represents a parsed SMTP command line.  Err is set if
// there was an error, empty otherwise. Cmd may be BadCmd or a
// command, even if there was an error.
type ParsedLine struct {
	Cmd Command
	Arg string
	// Params is K=V for ESMTP MAIL FROM and RCPT TO
	Params string
	Err    string
}

// See http://www.ietf.org/rfc/rfc1869.txt for the general discussion of
// params. We do not parse them.

type cmdArgs int

const (
	noArg cmdArgs = iota
	canArg
	mustArg
	colonAddress // for ':<addr>[ options...]'
)

// Our ideal of what requires an argument is slightly relaxed from the
// RFCs, ie we will accept argumentless HELO/EHLO.
var smtpCommand = []struct {
	cmd     Command
	text    string
	argtype cmdArgs
}{
	{HELO, "HELO", canArg},
	{EHLO, "EHLO", canArg},
	{MAILFROM, "MAIL FROM", colonAddress},
	{RCPTTO, "RCPT TO", colonAddress},
	{DATA, "DATA", noArg},
	{QUIT, "QUIT", noArg},
	{RSET, "RSET", noArg},
	{STARTTLS, "STARTTLS", noArg},
	{VRFY, "VRFY", canArg},
	{NOOP, "NOOP", canArg},
}

func (v Command) String() string {
	switch v {
	case noCmd:
		return "<zero Command value>"
	case BadCmd:
		return "<bad SMTP command>"
	default:
		for _, c := range smtpCommand {
			if c.cmd == v {
				return fmt.Sprintf("<SMTP '%s'>", c.text)
			}
		}
		// ... because someday I may screw this one up.
		return fmt.Sprintf("<Command cmd val %d>", v)
	}
}

// Returns True if the argument is all 7-bit ASCII. This is what all SMTP
// commands are supposed to be, and later things are going to screw up if
// some joker hands us UTF-8 or any other equivalent.
func isall7bit(b []byte) bool {
	for _, c := range b {
		if c > 127 {
			return false
		}
	}
	return true
}

// ParseCmd parses a SMTP command line and returns the result.
// The line should have the ending CR-NL already removed.
func ParseCmd(line string) ParsedLine {
	var res ParsedLine
	res.Cmd = BadCmd

	// We're going to upper-case this, which may explode on us if this
	// is UTF-8 or anything that smells like it.
	if !isall7bit([]byte(line)) {
		res.Err = "command contains non 7-bit ASCII"
		return res
	}

	// Trim trailing space from the line, because some confused people
	// send eg 'RSET ' or 'QUIT '. Probably other people put trailing
	// spaces on other commands. This is probably not completely okay
	// by the RFCs, but my view is 'real clients trump RFCs'.
	line = strings.TrimRightFunc(line, unicode.IsSpace)

	// Search in the command table for the prefix that matches. If
	// it's not found, this is definitely not a good command.
	// We search on an upper-case version of the line to make my life
	// much easier.
	found := -1
	upper := strings.ToUpper(line)
	for i := range smtpCommand {
		if strings.HasPrefix(upper, smtpCommand[i].text) {
			found = i
			break
		}
	}
	if found == -1 {
		res.Err = "unrecognized command"
		return res
	}

	// Validate that we've ended at a word boundary, either a space or
	// ':'. If we don't, this is not a valid match. Note that we now
	// work with the original-case line, not the upper-case version.
	cmd := smtpCommand[found]
	llen := len(line)
	clen := len(cmd.text)
	if !(llen == clen || line[clen] == ' ' || line[clen] == ':') {
		res.Err = "unrecognized command"
		return res
	}

	// This is a real command, so we must now perform real argument
	// extraction and validation. At this point any remaining errors
	// are command argument errors, so we set the command type in our
	// result.
	res.Cmd = cmd.cmd
	switch cmd.argtype {
	case noArg:
		if llen != clen {
			res.Err = "SMTP command does not take an argument"
			return res
		}
	case mustArg:
		if llen <= clen+1 {
			res.Err = "SMTP command requires an argument"
			return res
		}
		// Even if there are nominal characters they could be
		// all whitespace. Although we've trimmed trailing
		// whitespace before now, there could be whitespace
		// *before* the argument and we want to trim it too.
		t := strings.TrimSpace(line[clen+1:])
		if len(t) == 0 {
			res.Err = "SMTP command requires an argument"
			return res
		}
		res.Arg = t
	case canArg:
		// get rid of whitespace between command and the argument.
		if llen > clen+1 {
			res.Arg = strings.TrimSpace(line[clen+1:])
		}
	case colonAddress:
		var idx int
		// Minimum llen is clen + ':<>', three characters
		if llen < clen+3 {
			res.Err = "SMTP command requires an address"
			return res
		}
		// We explicitly check for '>' at the end of the string
		// to accept (at this point) 'MAIL FROM:<<...>>'. This will
		// fail if people also supply ESMTP parameters, of course.
		// Such is life.
		// BUG: this is imperfect because in theory I think you
		// can embed a quoted '>' inside a valid address and so
		// fool us. But I'm not putting a full RFC whatever address
		// parser in here, thanks, so we'll reject those.
		if line[llen-1] == '>' {
			idx = llen - 1
		} else {
			idx = strings.IndexByte(line, '>')
			if idx != -1 && line[idx+1] != ' ' {
				res.Err = "improper argument formatting"
				return res
			}
		}
		// NOTE: the RFC is explicit that eg 'MAIL FROM: <addr...>'
		// is not valid, ie there cannot be a space between the : and
		// the '<'. Normally we'd refuse to accept it, but a few too
		// many things invalidly generate it.
		if line[clen] != ':' || idx == -1 {
			res.Err = "improper argument formatting"
			return res
		}
		spos := clen + 1
		if line[spos] == ' ' {
			spos++
		}
		if line[spos] != '<' {
			res.Err = "improper argument formatting"
			return res
		}
		res.Arg = line[spos+1 : idx]
		// As a side effect of this we generously allow trailing
		// whitespace after RCPT TO and MAIL FROM. You're welcome.
		res.Params = strings.TrimSpace(line[idx+1 : llen])
	}
	return res
}

//
// ---
// Protocol state machine

// States of the SMTP conversation. These are bits and can be masked
// together.
type conState int

const (
	sStartup conState = iota // Must be zero value
	sInitial conState = 1 << iota
	sHelo
	sMail
	sRcpt
	sData
	sQuit // QUIT received and ack'd, we're exiting.

	// Synthetic state
	sPostData
	sAbort
)

// A command not in the states map is handled in all states (probably to
// be rejected).
var states = map[Command]struct {
	validin, next conState
}{
	HELO:     {sInitial | sHelo, sHelo},
	EHLO:     {sInitial | sHelo, sHelo},
	MAILFROM: {sHelo, sMail},
	RCPTTO:   {sMail | sRcpt, sRcpt},
	DATA:     {sRcpt, sData},
}

// Limits has the time and message limits for a Conn, as well as some
// additional options.
//
// A Conn always accepts 'BODY=[7BIT|8BITMIME]' as the sole MAIL FROM
// parameter, since it advertises support for 8BITMIME.
type Limits struct {
	IOTimeout time.Duration // timeout for read and write operations
	MsgSize   int64         // total size of an email message
	BadCmds   int           // how many unknown commands before abort
}

// Config represents the configuration for a Conn. If unset, Limits is
// DefaultLimits, ServerName is 'localhost', and SftName is 'go-smtpd'.
type Config struct {
	TLSConfig  *tls.Config // TLS configuration if TLS is to be enabled
	Limits     *Limits     // The limits applied to the connection
	ServerName string      // The local hostname to use in messages
}

// Conn represents an ongoing SMTP connection. The TLS fields are
// read-only.
//
// Note that this structure cannot be created by hand. Call NewConn.
//
// Conn connections always advertise support for PIPELINING and
// 8BITMIME.  STARTTLS is advertised if the Config passed to NewConn()
// has a non-nil TLSConfig.
//
// Conn.Config can be altered to some degree after Conn is created in
// order to manipulate features on the fly. Note that Conn.Config.Limits
// is a pointer and so its fields should not be altered unless you
// know what you're doing and it's your Limits to start with.
type Conn struct {
	conn   net.Conn
	lr     *io.LimitedReader // wraps conn as a reader
	rdr    *textproto.Reader // wraps lr
	logger io.Writer

	Config Config // Connection configuration

	state   conState
	badcmds int // count of bad commands so far

	// queued event returned by a forthcoming Next call
	nextEvent *EventInfo

	// used for state tracking for Accept()/Reject()/Tempfail().
	curcmd  Command
	replied bool
	nstate  conState // next state if command is accepted.

	TLSOn    bool                // TLS is on in this connection
	TLSState tls.ConnectionState // TLS connection state
	TLSHelp  string              // TLSHelp is a log friendly string indicating the status of TLS negotiation
}

// An Event is the sort of event that is returned by Conn.Next().
type Event int

// The different types of SMTP events returned by Next()
const (
	_        Event = iota // make uninitialized Event an error.
	COMMAND  Event = iota
	GOTDATA        // received DATA
	DONE           // client sent QUIT
	ABORT          // input or output error or timeout.
	TLSERROR       // error during TLS setup. Connection is dead.
)

// EventInfo is what Conn.Next() returns to represent events.
// Cmd and Arg come from ParsedLine.
type EventInfo struct {
	What Event
	Cmd  Command
	Arg  string
}

func (c *Conn) reply(format string, elems ...interface{}) {
	var err error
	s := fmt.Sprintf(format, elems...)
	b := []byte(s + "\r\n")
	// we can ignore the length returned, because Write()'s contract
	// is that it returns a non-nil err if n < len(b).
	// We are cautious about our write deadline.
	c.conn.SetWriteDeadline(time.Now().Add(c.Config.Limits.IOTimeout))
	_, err = c.conn.Write(b)
	if err != nil {
		c.state = sAbort
	}
}

// This is a crude hack for EHLO writing. It skips emitting the reply
// line if we've already aborted (which is assumed to be because of a
// write error). Some clients close the connection as we're writing
// our multi-line EHLO reply out, which otherwise produces one error
// per EHLO line instead of stopping immediately.
//
// This is kind of a code smell in that we're doing the EHLO reply
// in the wrong way, but doing it the current way is also the easiest
// and simplest way. Such is life.
func (c *Conn) replyMore(format string, elems ...interface{}) {
	if c.state != sAbort {
		c.reply(format, elems...)
	}
}

func (c *Conn) replyMulti(code int, format string, elems ...interface{}) {
	rs := strings.Trim(fmt.Sprintf(format, elems...), " \t\n")
	sl := strings.Split(rs, "\n")
	cont := '-'
	for i := range sl {
		if i == len(sl)-1 {
			cont = ' '
		}
		c.reply("%3d%c%s", code, cont, sl[i])
		if c.state == sAbort {
			break
		}
	}
}

func (c *Conn) readCmd() string {
	// This is much bigger than the RFC requires.
	c.lr.N = 2048
	// Allow two minutes per command.
	c.conn.SetReadDeadline(time.Now().Add(c.Config.Limits.IOTimeout))
	line, err := c.rdr.ReadLine()
	// abort not just on errors but if the line length is exhausted.
	if err != nil || c.lr.N == 0 {
		c.state = sAbort
		line = ""
	}
	return line
}

func (c *Conn) readData() string {
	c.conn.SetReadDeadline(time.Now().Add(c.Config.Limits.IOTimeout))
	c.lr.N = c.Config.Limits.MsgSize
	b, err := c.rdr.ReadDotBytes()
	if err != nil || c.lr.N == 0 {
		c.state = sAbort
		b = nil
	}
	return string(b)
}

func (c *Conn) stopme() bool {
	return c.state == sAbort || c.badcmds > c.Config.Limits.BadCmds || c.state == sQuit
}

// Accept accepts the current SMTP command, ie gives an appropriate
// 2xx reply to the client.
func (c *Conn) Accept() {
	if c.replied {
		return
	}
	oldstate := c.state
	c.state = c.nstate
	switch c.curcmd {
	case HELO:
		c.reply("250 %s", c.Config.ServerName)
	case EHLO:
		c.reply("250-%s", c.Config.ServerName)
		// We advertise 8BITMIME per
		// http://cr.yp.to/smtp/8bitmime.html
		c.replyMore("250-8BITMIME")
		c.replyMore("250-PIPELINING")
		// STARTTLS RFC says: MUST NOT advertise STARTTLS
		// after TLS is on.
		if c.Config.TLSConfig != nil && !c.TLSOn {
			c.replyMore("250-STARTTLS")
		}
		// We do not advertise SIZE because our size limits
		// are different from the size limits that RFC 1870
		// wants us to use. We impose a flat byte limit while
		// RFC 1870 wants us to not count quoted dots.
		// Advertising SIZE would also require us to parse
		// SIZE=... on MAIL FROM in order to 552 any too-large
		// sizes.
		// On the whole: pass. Cannot implement.
		// (In general SIZE is hella annoying if you read the
		// RFC religiously.)
		c.replyMore("250 Ok")
	case MAILFROM:
		c.reply("250 2.1.0 Ok")
	case RCPTTO:
		c.reply("250 2.1.5 Ok")
	case DATA:
		// c.curcmd == DATA both when we've received the
		// initial DATA and when we've actually received the
		// data-block. We tell them apart based on the old
		// state, which is sRcpt or sPostData respectively.
		if oldstate == sRcpt {
			c.reply("354 End data with <CR><LF>.<CR><LF>")
		} else {
			c.reply("250 2.0.0 Ok")
		}
	}
	c.replied = true
}

// Reject rejects the curent SMTP command, ie gives the client an
// appropriate 5xx message.
func (c *Conn) Reject() {
	switch c.curcmd {
	case HELO, EHLO:
		c.reply("550 Not accepted")
	case MAILFROM, RCPTTO:
		c.reply("550 Bad address")
	case DATA:
		c.reply("554 Not accepted")
	}
	c.replied = true
}

// Reply451 sends a 451 status response and tells client that rate/conversation limit may have been exceeded.
func (c *Conn) Reply451() {
	c.reply("451 Try again later rate limit exceeded or too many conversations")
	c.replied = true
	c.state = sAbort
}

// Next returns the next high-level event from the SMTP connection.
//
// Next() guarantees that the SMTP protocol ordering requirements are
// followed and only returns HELO/EHLO, MAIL FROM, RCPT TO, and DATA
// commands, and the actual message submitted. The caller must reset
// all accumulated information about a message when it sees either
// EHLO/HELO or MAIL FROM.
//
// For commands and GOTDATA, the caller may call Reject() or
// Tempfail() to reject or tempfail the command. Calling Accept() is
// optional; Next() will do it for you implicitly.
// It is invalid to call Next() after it has returned a DONE or ABORT
// event.
//
// Next() does almost no checks on the value of EHLO/HELO, MAIL FROM,
// and RCPT TO. For MAIL FROM and RCPT TO it requires them to
// actually be present, but that's about it. It will accept blank
// EHLO/HELO (ie, no argument at all).  It is up to the caller to do
// more validation and then call Reject() (or Tempfail()) as
// appropriate.  MAIL FROM addresses may be blank (""), indicating the
// null sender ('<>'). RCPT TO addresses cannot be; Next() will fail
// those itself.
//
// TLSERROR is returned if the client tried STARTTLS on a TLS-enabled
// connection but the TLS setup failed for some reason (eg the client
// only supports SSLv2). The caller can use this to, eg, decide not to
// offer TLS to that client in the future. No further activity can
// happen on a connection once TLSERROR is returned; the connection is
// considered dead and calling .Next() again will yield an ABORT
// event. The Arg of a TLSERROR event is the TLS error in string form.
func (c *Conn) Next() EventInfo {
	var evt EventInfo

	if c.nextEvent != nil {
		evt = *c.nextEvent
		c.nextEvent = nil
		return evt
	}
	if !c.replied && c.curcmd != noCmd {
		c.Accept()
	}
	if c.state == sStartup {
		c.state = sInitial
		// log preceeds the banner in case the banner hits an error.
		c.replyMulti(220, "%s ESMTP", c.Config.ServerName)
	}

	// Read DATA chunk if called for.
	if c.state == sData {
		data := c.readData()
		if len(data) > 0 {
			evt.What = GOTDATA
			evt.Arg = data
			c.replied = false
			// This is technically correct; only a *successful*
			// DATA block ends the mail transaction according to
			// the RFCs. An unsuccessful one must be RSET.
			c.state = sPostData
			c.nstate = sHelo
			return evt
		}
		// If the data read failed, c.state will be sAbort and we
		// will exit in the main loop.
	}

	// Main command loop.
	for {
		if c.stopme() {
			break
		}

		line := c.readCmd()
		if line == "" {
			break
		}

		res := ParseCmd(line)
		if res.Cmd == BadCmd {
			c.badcmds++
			c.reply("501 Bad: %s", res.Err)
			continue
		}
		// Is this command valid in this state at all?
		// Since we implicitly support PIPELINING, which can
		// result in out of sequence commands when earlier ones
		// fail, we don't count out of sequence commands as bad
		// commands.
		t := states[res.Cmd]
		if t.validin != 0 && (t.validin&c.state) == 0 {
			c.reply("503 Out of sequence command")
			continue
		}
		// Error in command?
		if len(res.Err) > 0 {
			c.reply("553 Garbled command: %s", res.Err)
			continue
		}

		// The command is legitimate. Handle it for real.

		// Handle simple commands that are valid in all states.
		if t.validin == 0 {
			switch res.Cmd {
			case RSET:
				// It's valid to RSET before EHLO and
				// doing so can't skip EHLO.
				if c.state != sInitial {
					c.state = sHelo
				}
				c.reply("250 Ok")
				// RSETs are not delivered to higher levels;
				// they are implicit in sudden MAIL FROMs.
			case VRFY:
				// Will not reveal user information
				c.reply("252 Ok")
			case NOOP:
				c.reply("250 Ok")
			case QUIT:
				c.state = sQuit
				c.reply("221 2.0.0 Bye")
			case STARTTLS:
				if c.Config.TLSConfig == nil || c.TLSOn {
					c.reply("502 Not supported")
					c.TLSHelp = "client asked but this server does not support TLS"
					continue
				}
				c.reply("220 Ready to start TLS")
				if c.state == sAbort {
					c.TLSHelp = "connection aborted before negotiation"
					continue
				}
				// Since we're about to start chattering on
				// conn outside of our normal framework, we
				// must reset both read and write timeouts
				// to our TLS setup timeout.
				c.conn.SetDeadline(time.Now().Add(c.Config.Limits.IOTimeout))
				tlsConn := tls.Server(c.conn, c.Config.TLSConfig)
				err := tlsConn.Handshake()
				if err == nil {
					c.TLSHelp = "handshake was successful"
					// With TLS set up, we now want no read and
					// write deadlines on the underlying
					// connection. So cancel all deadlines by
					// providing a zero value.
					c.conn.SetReadDeadline(time.Time{})
					// switch c.conn to tlsConn.
					c.setupConn(tlsConn)
					c.TLSOn = true
					c.TLSState = tlsConn.ConnectionState()
					// By the STARTTLS RFC, we return to our state
					// immediately after the greeting banner
					// and clients must re-EHLO.
					c.state = sInitial
				} else {
					c.TLSHelp = "handshake failure - " + err.Error()
					evt.What = TLSERROR
					evt.Arg = fmt.Sprintf("%v", err)
					c.reply("454 TLS handshake failure")
				}

			default:
				c.reply("502 Not supported")
			}
			continue
		}

		// Full state commands
		c.nstate = t.next
		c.replied = false
		c.curcmd = res.Cmd

		// Real, valid, in sequence command. Deliver it to our
		// caller.
		evt.What = COMMAND
		evt.Cmd = res.Cmd
		evt.Arg = res.Arg
		return evt
	}

	// Explicitly mark and notify too many bad commands. This is
	// an out of sequence 'reply', but so what, the client will
	// see it if they send anything more. It will also go in the
	// SMTP command log.
	evt.Arg = ""
	if c.badcmds > c.Config.Limits.BadCmds {
		c.reply("554 Too many bad commands")
		c.state = sAbort
		evt.Arg = "too many bad commands"
	}
	if c.state == sQuit {
		evt.What = DONE
	}
	return evt
}

// We need this for re-setting up the connection on TLS start.
func (c *Conn) setupConn(conn net.Conn) {
	c.conn = conn
	// io.LimitReader() returns a Reader, not a LimitedReader, and
	// we want access to the public lr.N field so we can manipulate
	// it.
	c.lr = io.LimitReader(conn, 4096).(*io.LimitedReader)
	c.rdr = textproto.NewReader(bufio.NewReader(c.lr))
}

// NewConn creates a new SMTP conversation from conn, the underlying
// network connection involved.  servername is the server name
// displayed in the greeting banner.  A trace of SMTP commands and
// responses (but not email messages) will be written to log if it's
// non-nil.
//
// Log messages start with a character, then a space, then the
// message.  'r' means read from network (client input), 'w' means
// written to the network (server replies), '!'  means an error, and
// '#' is tracking information for the start or the end of the
// connection. Further information is up to whatever is behind 'log'
// to add.
func NewConn(conn net.Conn, cfg Config, log io.Writer) *Conn {
	c := &Conn{state: sStartup, Config: cfg, logger: log, TLSHelp: "not used"}
	c.setupConn(conn)
	if c.Config.Limits == nil {
		panic("Limits are not configured")
	}
	if c.Config.ServerName == "" {
		panic("Server name is empty")
	}
	return c
}
