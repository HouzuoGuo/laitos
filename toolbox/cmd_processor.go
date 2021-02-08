package toolbox

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
)

const (
	//ErrBadProcessorConfig is used as the prefix string in all errors returned by "IsSaneForInternet" function.
	ErrBadProcessorConfig = "bad configuration: "

	/*
		PrefixCommandPLT is the magic string to prefix command input, in order to navigate around among the output and
		temporarily alter execution timeout. PLT stands for "position, length, timeout".
	*/
	PrefixCommandPLT = ".plt"

	// TestCommandProcessorPIN is the PIN secret used in test command processor, as returned by GetTestCommandProcessor.
	TestCommandProcessorPIN = "verysecret"

	// MaxCmdPerSecHardLimit is the hard uppper limit of the approximate maximum number of commands a command processor will process in a second.
	MaxCmdPerSecHardLimit = 1000
	// MaxCmdLength is the maximum length of a single command (including password PIN and other prefixes) that the command processor will accept.
	MaxCmdLength = 16 * 1024
)

// ErrBadPrefix is a command execution error triggered if the command does not contain a valid toolbox feature trigger.
var ErrBadPrefix = errors.New("bad prefix or feature is not configured")

// ErrBadPLT reminds user of the proper syntax to invoke PLT magic.
var ErrBadPLT = errors.New(PrefixCommandPLT + " P L T command")

// ErrCommandTooLong is a command execution error indicating that the input is too long and cannot be accepted.
var ErrCommandTooLong = fmt.Errorf("command input exceeds the maximum length of %d characters", MaxCmdLength)

// ErrRateLimitExceeded is a command execution error indicating that the internal command processing rate limit has been exceeded
var ErrRateLimitExceeded = errors.New("command processor internal rate limit has been exceeded")

// RegexCommandWithPLT parses PLT magic parameters position, length, and timeout, all of which are integers.
var RegexCommandWithPLT = regexp.MustCompile(`[^\d]*(\d+)[^\d]+(\d+)[^\d]*(\d+)(.*)`)

// RegexSubjectReportUsing2FA matches a message processor's subject report app command invoked via 2FA.
var RegexSubjectReportUsing2FA = regexp.MustCompile(`[\d]{12}[\s]*\` + StoreAndForwardMessageProcessorTrigger)

// Pre-configured environment and configuration for processing feature commands.
type CommandProcessor struct {
	Features       *FeatureSet     // Features is the aggregation of initialised toolbox feature routines.
	CommandFilters []CommandFilter // CommandFilters are applied one by one to alter input command content and/or timeout.
	ResultFilters  []ResultFilter  // ResultFilters are applied one by one to alter command execution result.

	/*
		MaxCmdPerSec is the approximate maximum number of commands allowed to be processed per second.
		The limit is a defensive measure against an attacker trying to guess the correct password by using visiting a daemon
		from large range of source IP addresses in an attempt to bypass daemon's own per-IP rate limit mechanism.
	*/
	MaxCmdPerSec int
	rateLimit    *misc.RateLimit
	// initOnce helps to initialise the command processor in preparation for processing command for the first time.
	initOnce sync.Once

	logger lalog.Logger
}

// initialiseOnce prepares the command processor for processing command for the first time.
func (proc *CommandProcessor) initialiseOnce() {
	proc.initOnce.Do(func() {
		// Reset the maximum command per second limit
		if proc.MaxCmdPerSec < 1 || proc.MaxCmdPerSec > MaxCmdPerSecHardLimit {
			proc.MaxCmdPerSec = MaxCmdPerSecHardLimit
		}
		// Initialise command rate limit
		if proc.rateLimit == nil {
			proc.rateLimit = &misc.RateLimit{
				UnitSecs: 1,
				MaxCount: proc.MaxCmdPerSec,
				Logger:   proc.logger,
			}
			proc.rateLimit.Initialise()
		}
	})
}

// SetLogger uses the input logger to prepare the command processor and its filters.
func (proc *CommandProcessor) SetLogger(logger lalog.Logger) {
	// The command processor itself as well as the filters are going to share the same logger
	proc.logger = logger
	for _, b := range proc.ResultFilters {
		b.SetLogger(logger)
	}
}

/*
IsEmpty returns true only if the command processor does not appear to have a meaningful configuration, which means:
- It does not have a PIN filter (the password protection).
- There are no filters configured at all.
*/
func (proc *CommandProcessor) IsEmpty() bool {
	if proc.CommandFilters == nil || len(proc.CommandFilters) == 0 {
		// An empty processor does not have any filter configuration
		return true
	}
	for _, cmdFilter := range proc.CommandFilters {
		// An empty processor does not have a PIN
		if pinFilter, ok := cmdFilter.(*PINAndShortcuts); ok && len(pinFilter.Passwords) == 0 {
			return true
		}
	}
	return false
}

/*
From the prospect of Internet-facing mail processor and Twilio hooks, check that parameters are within sane range.
Return a zero-length slice if everything looks OK.
*/
func (proc *CommandProcessor) IsSaneForInternet() (errs []error) {
	errs = make([]error, 0)
	// Check for nils too, just in case.
	if proc.Features == nil {
		errs = append(errs, errors.New(ErrBadProcessorConfig+"FeatureSet is not assigned"))
	} else {
		if len(proc.Features.LookupByTrigger) == 0 {
			errs = append(errs, errors.New(ErrBadProcessorConfig+"FeatureSet is not initialised or all features are lacking configuration"))
		}
	}
	if proc.CommandFilters == nil {
		errs = append(errs, errors.New(ErrBadProcessorConfig+"CommandFilters is not assigned"))
	} else {
		// Check whether PIN bridge is sanely configured
		seenPIN := false
		for _, cmdBridge := range proc.CommandFilters {
			if pin, yes := cmdBridge.(*PINAndShortcuts); yes {
				if len(pin.Passwords) == 0 && (pin.Shortcuts == nil || len(pin.Shortcuts) == 0) {
					errs = append(errs, errors.New(ErrBadProcessorConfig+"Defined in PINAndShortcuts there has to be password PIN, command shortcuts, or both."))
				}
				for _, password := range pin.Passwords {
					if len(password) < 7 {
						errs = append(errs, errors.New(ErrBadProcessorConfig+"Each password must be at least 7 characters long"))
						break
					}
				}
				seenPIN = true
				break
			}
		}
		if !seenPIN {
			errs = append(errs, errors.New(ErrBadProcessorConfig+"\"PINAndShortcuts\" filter must be defined to set up password PIN protection or command shortcuts"))
		}
	}
	if proc.ResultFilters == nil {
		errs = append(errs, errors.New(ErrBadProcessorConfig+"ResultFilters is not assigned"))
	} else {
		// Check whether string linter is sanely configured
		seenLinter := false
		for _, resultBridge := range proc.ResultFilters {
			if linter, yes := resultBridge.(*LintText); yes {
				if linter.MaxLength < 35 || linter.MaxLength > 4096 {
					errs = append(errs, errors.New(ErrBadProcessorConfig+"Maximum output length for LintText must be within [35, 4096]"))
				}
				seenLinter = true
				break
			}
		}
		if !seenLinter {
			errs = append(errs, errors.New(ErrBadProcessorConfig+"\"LintText\" filter must be defined to restrict command output length"))
		}
	}
	return
}

/*
Process applies filters to the command, invokes toolbox feature functions to process the content, and then applies
filters to the execution result and return.
A special content prefix called "PLT prefix" alters filter settings to temporarily override timeout and max.length
settings, and it may optionally discard a number of characters from the beginning.
*/
func (proc *CommandProcessor) Process(ctx context.Context, cmd Command, runResultFilters bool) (ret *Result) {
	proc.initialiseOnce()
	// Refuse to execute a command if global lock down has been triggered
	if misc.EmergencyLockDown {
		return &Result{Error: misc.ErrEmergencyLockDown}
	}
	// Refuse to execute a command if the internal rate limit has been reached
	if !proc.rateLimit.Add("instance", true) {
		return &Result{Error: ErrRateLimitExceeded}
	}
	// Refuse to execute a command if it is exceedingly long
	if len(cmd.Content) > MaxCmdLength {
		return &Result{Error: ErrCommandTooLong}
	}
	/*
		Hacky workaround - do not run result filter for the store&forward message processor, which runs an app command
		with its own command processor and its own result filters.
	*/
	if RegexSubjectReportUsing2FA.MatchString(cmd.Content) {
		runResultFilters = false
	}

	// Put execution duration into statistics
	beginTimeNano := time.Now().UnixNano()
	var filterDisapproval error
	var matchedFeature Feature
	var overrideLintText LintText
	var hasOverrideLintText bool
	var logCommandContent string
	// Walk the command through all filters
	for _, cmdBridge := range proc.CommandFilters {
		cmd, filterDisapproval = cmdBridge.Transform(cmd)
		if filterDisapproval != nil {
			ret = &Result{Error: filterDisapproval}
			goto result
		}
	}
	// If filters approve, then the command execution is to be tracked in stats.
	defer func() {
		misc.CommandStats.Trigger(float64(time.Now().UnixNano() - beginTimeNano))
	}()
	// Trim spaces and expect non-empty command
	if ret = cmd.Trim(); ret != nil {
		goto result
	}
	// Look for PLT (position, length, timeout) override, it is going to affect LintText filter.
	if cmd.FindAndRemovePrefix(PrefixCommandPLT) {
		// Find the configured LintText bridge
		for _, resultBridge := range proc.ResultFilters {
			if aBridge, isLintText := resultBridge.(*LintText); isLintText {
				overrideLintText = *aBridge
				hasOverrideLintText = true
				break
			}
		}
		if !hasOverrideLintText {
			ret = &Result{Error: errors.New("PLT is not available because LintText is not used")}
			goto result
		}
		// Parse P. L. T. <cmd> parameters
		pltParams := RegexCommandWithPLT.FindStringSubmatch(cmd.Content)
		if len(pltParams) != 5 { // 4 groups + 1
			ret = &Result{Error: ErrBadPLT}
			goto result
		}
		var intErr error
		if overrideLintText.BeginPosition, intErr = strconv.Atoi(pltParams[1]); intErr != nil {
			ret = &Result{Error: ErrBadPLT}
			goto result
		}
		if overrideLintText.MaxLength, intErr = strconv.Atoi(pltParams[2]); intErr != nil {
			ret = &Result{Error: ErrBadPLT}
			goto result
		}
		if cmd.TimeoutSec, intErr = strconv.Atoi(pltParams[3]); intErr != nil {
			ret = &Result{Error: ErrBadPLT}
			goto result
		}
		cmd.Content = pltParams[4]
		if cmd.Content == "" {
			ret = &Result{Error: ErrBadPLT}
			goto result
		}
	}
	/*
		Now the command has gone through modifications made by command filters. Keep a copy of its content for logging
		purpose before it is further manipulated by individual feature's routine that may add or remove bits from the
		content.
	*/
	logCommandContent = cmd.Content
	// Look for command's prefix among configured features
	for prefix, configuredFeature := range proc.Features.LookupByTrigger {
		if cmd.FindAndRemovePrefix(string(prefix)) {
			// Hacky workaround - do not log content of AES decryption commands as they can reveal encryption key
			if prefix == AESDecryptTrigger || prefix == TwoFATrigger || prefix == NBETrigger {
				logCommandContent = "<hidden due to AESDecryptTrigger or TwoFATrigger or NBETrigger>"
			}
			matchedFeature = configuredFeature
			break
		}
	}
	// Unknown command prefix or the requested feature is not configured
	if matchedFeature == nil {
		ret = &Result{Error: ErrBadPrefix}
		goto result
	}
	// Run the feature
	proc.logger.Info("Process", fmt.Sprintf("%s-%s", cmd.DaemonName, cmd.ClientTag), nil, "running \"%s\" (post-process result? %v)", logCommandContent, runResultFilters)
	defer func() {
		proc.logger.Info("Process", fmt.Sprintf("%s-%s", cmd.DaemonName, cmd.ClientTag), nil, "completed \"%s\" (ok? %v post-process reslt? %v)", logCommandContent, ret.Error == nil, runResultFilters)
	}()
	ret = matchedFeature.Execute(ctx, cmd)
result:
	// Command in the result structure is mainly used for logging purpose
	ret.Command = cmd
	/*
		Features may have modified command in-place to remove certain content and it's OK to do that.
		But to make log messages more meaningful, it is better to restore command content to the modified one
		after triggering filters, and before triggering features.
	*/
	ret.Command.Content = logCommandContent
	// Set combined text for easier retrieval of result+error in one text string
	ret.ResetCombinedText()
	// Walk through result filters
	if runResultFilters {
		for _, resultFilter := range proc.ResultFilters {
			// LintText bridge may have been manipulated by override
			if _, isLintText := resultFilter.(*LintText); isLintText && hasOverrideLintText {
				resultFilter = &overrideLintText
			}
			if err := resultFilter.Transform(ret); err != nil {
				return &Result{Command: ret.Command, Error: filterDisapproval}
			}
		}
	}
	return
}

// Return a realistic command processor for test cases. The only feature made available and initialised is shell execution.
func GetTestCommandProcessor() *CommandProcessor {
	/*
		Prepare feature set - certain simple features such as shell commands and environment control will be available
		right away without configuration.
	*/
	features := &FeatureSet{}
	if err := features.Initialise(); err != nil {
		panic(err)
	}
	// Prepare realistic command bridges
	commandBridges := []CommandFilter{
		&PINAndShortcuts{Passwords: []string{TestCommandProcessorPIN}},
		&TranslateSequences{Sequences: [][]string{{"alpha", "beta"}}},
	}
	// Prepare realistic result bridges
	resultBridges := []ResultFilter{
		&LintText{TrimSpaces: true, MaxLength: 35},
		&SayEmptyOutput{},
		&NotifyViaEmail{},
	}
	return &CommandProcessor{
		Features:       features,
		CommandFilters: commandBridges,
		ResultFilters:  resultBridges,
	}
}

// Return a do-nothing yet sane command processor that has a random long password, rendering it unable to invoke any feature.
func GetEmptyCommandProcessor() *CommandProcessor {
	features := &FeatureSet{}
	if err := features.Initialise(); err != nil {
		panic(err)
	}
	randPIN := make([]byte, 128)
	if _, err := rand.Read(randPIN); err != nil {
		panic(err)
	}
	return &CommandProcessor{
		Features: features,
		CommandFilters: []CommandFilter{
			&PINAndShortcuts{Passwords: []string{strconv.FormatInt(time.Now().UnixNano(), 10) + hex.EncodeToString(randPIN)}},
		},
		ResultFilters: []ResultFilter{
			&LintText{MaxLength: 35},
		},
	}
}

/*
GetInsaneCommandProcessor returns a command processor that does not have a sane configuration for general usage.
This is a test case helper.
*/
func GetInsaneCommandProcessor() *CommandProcessor {
	features := &FeatureSet{}
	if err := features.Initialise(); err != nil {
		panic(err)
	}
	return &CommandProcessor{
		Features: features,
		CommandFilters: []CommandFilter{
			&PINAndShortcuts{Passwords: []string{"short"}},
		},
		ResultFilters: []ResultFilter{
			&LintText{MaxLength: 10},
		},
	}
}
