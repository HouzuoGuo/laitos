package toolbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/netboundfileenc"
)

const (
	// NBEResponseNoOutstandingIntents is the text response made by the network-bound file encryption app, when a command request asks for
	// outstanding unlocking intents but there are none.
	NBEResponseNoOutstandingIntents = "there are no outstanding unlocking intents"
	// NBETrigger is the network bound file encryption app's trigger prefix string.
	NBETrigger = ".nbe"
)

// NetBoundFileEncryption hosts facilities for another instance of laitos program to register an intent of obtaining unlocking password
// for its config/data files.
// The app interface implementation allows a user to query outstanding unlocking intents and record passwords for them to retrieve, the
// app is closely tied to the embedded gRPC service, which provides RPC transport for the other laitos program instances to post unlocking
// intents and retrieve passwords.
type NetBoundFileEncryption struct {
	// MaxIntents is the maximum number of password unlocking intents that will be kept in memory to be fulfilled.
	MaxIntents int `json:"MaxIntents"`
	// MaxCallsPerSec is the approximate number of RPC calls a client (identified by its IP) may make in a second.
	MaxCallsPerSec int `json:"MaxCallsPerSec"`
	*netboundfileenc.PasswordRegister
}

// Initialise always returns nil.
func (nbe *NetBoundFileEncryption) Initialise() error {
	// Apply the default config of max 5 calls/sec, max 30 unlocking intents kept in memory.
	if nbe.MaxIntents < 1 {
		nbe.MaxIntents = 30
	}
	if nbe.MaxCallsPerSec < 1 {
		nbe.MaxCallsPerSec = 5
	}
	nbe.PasswordRegister = netboundfileenc.NewPasswordRegister(nbe.MaxIntents, nbe.MaxCallsPerSec, &lalog.Logger{ComponentName: "netboundfileencryption"})
	return nil
}

// IsConfigured always returns true because this app does not use app-specific configuration.
func (nbe *NetBoundFileEncryption) IsConfigured() bool {
	return true
}

// SelfTest always returns nil.
func (nbe *NetBoundFileEncryption) SelfTest() error {
	return nil
}

// Trigger returns the app's trigger prefix string ".nbe", used by app command processor to associate a command with this app.
func (nbe *NetBoundFileEncryption) Trigger() Trigger {
	return NBETrigger
}

// Execute interprets the text command from the input, the command may query for the outstanding unlocking intents, or supply password text
// to fulfil an outstanding intent.
func (nbe *NetBoundFileEncryption) Execute(_ context.Context, cmd Command) *Result {
	if strings.TrimSpace(cmd.Content) == "" {
		// Without parameters, the command will query the outstanding unlocking intents.
		intents := nbe.GetOutstandingIntents()
		if len(intents) == 0 {
			return &Result{Output: NBEResponseNoOutstandingIntents}
		}
		resp := new(bytes.Buffer)
		// List one intent on each line
		for _, clientIntent := range nbe.GetOutstandingIntents() {
			resp.WriteString(fmt.Sprintf("%s\t%s\t%s\t%d\t%d\t%d\t%s\n",
				clientIntent.ClientIP, clientIntent.Time, clientIntent.HostName, clientIntent.PID,
				clientIntent.UserID, clientIntent.UptimeSec, clientIntent.RandomChallenge))
		}
		return &Result{Output: resp.String()}
	}
	// The command is interpreted as a fulfilment of an outstanding unlocking intent in the format of "ChallengeString[ ]Password"
	fields := regexp.MustCompile(`^\s*(\S+)\s+(\S+)\s*$`).FindStringSubmatch(cmd.Content)
	if len(fields) != 3 {
		return &Result{Error: errors.New(".nbe ClientChallengeString UnlockPassword")}
	} else if !nbe.FulfilIntent(fields[1], fields[2]) {
		return &Result{Error: fmt.Errorf("there is not an outstanding password unlock attempt carrying string challenge \"%s\"", fields[1])}
	}
	return &Result{Output: "OK"}
}
