package common

import (
	"errors"
	"github.com/HouzuoGuo/websh/bridge"
	"github.com/HouzuoGuo/websh/feature"
	"log"
)

var ErrBadPrefix = errors.New("Bad prefix or feature is not configured")

// Environment and configuration for running commands.
type CommandProcessor struct {
	Features       *feature.FeatureSet
	CommandBridges []bridge.CommandBridge
	ResultBridges  []bridge.ResultBridge
}

const ErrBadProcessorConfig = "Insane: " // Prefix errors in function IsSaneForInternet

/*
From the prospect of Internet-facing mail processor and Twilio hooks, check that parameters are within sane range.
Return a zero-length slice if everything looks OK.
*/
func (proc *CommandProcessor) IsSaneForInternet() (errs []error) {
	errs = make([]error, 0, 0)
	// Check for nils too, just in case.
	if proc.Features == nil {
		errs = append(errs, errors.New(ErrBadProcessorConfig+"FeatureSet is not assigned"))
	} else {
		if len(proc.Features.LookupByTrigger) == 0 {
			errs = append(errs, errors.New(ErrBadProcessorConfig+"FeatureSet is not intialised or all features are lacking configuration"))
		}
	}
	if proc.CommandBridges == nil {
		errs = append(errs, errors.New(ErrBadProcessorConfig+"CommandBridges is not assigned"))
	} else {
		// Check whether PIN bridge is sanely configured
		seenPIN := false
		for _, cmdBridge := range proc.CommandBridges {
			if pin, yes := cmdBridge.(*bridge.PINAndShortcuts); yes {
				if pin.PIN == "" && (pin.Shortcuts == nil || len(pin.Shortcuts) == 0) {
					errs = append(errs, errors.New(ErrBadProcessorConfig+"PIN is empty and there is no shortcut defined, hence no command will ever execute."))
				}
				if pin.PIN != "" && len(pin.PIN) < 5 {
					errs = append(errs, errors.New(ErrBadProcessorConfig+"PIN is too short, make it at least 5 characters long to be somewhat secure."))
				}
				seenPIN = true
				break
			}
		}
		if !seenPIN {
			errs = append(errs, errors.New(ErrBadProcessorConfig+"\"CommandPINOrShortcut\" bridge is not used, this is horribly insecure."))
		}
	}
	if proc.ResultBridges == nil {
		errs = append(errs, errors.New(ErrBadProcessorConfig+"ResultBridges is not assigned"))
	} else {
		// Check whether string linter is sanely configured
		seenLinter := false
		for _, resultBridge := range proc.ResultBridges {
			if linter, yes := resultBridge.(*bridge.LintText); yes {
				if linter.MaxLength < 35 || linter.MaxLength > 4096 {
					errs = append(errs, errors.New(ErrBadProcessorConfig+"Maximum output length is not within [35, 4096]. This may cause undesired telephone cost."))
				}
				seenLinter = true
				break
			}
		}
		if !seenLinter {
			errs = append(errs, errors.New(ErrBadProcessorConfig+"\"LintCombinedText\" bridge is not used, this may cause crashes or undesired telephone cost."))
		}
	}
	return
}

func (proc *CommandProcessor) Process(cmd feature.Command) (ret *feature.Result) {
	var bridgeErr error
	var matchedFeature feature.Feature
	// Walk the command through all bridges
	for _, cmdBridge := range proc.CommandBridges {
		cmd, bridgeErr = cmdBridge.Transform(cmd)
		if bridgeErr != nil {
			ret = &feature.Result{Error: bridgeErr}
			goto result
		}
	}
	// Trim spaces and expect non-empty command
	if ret = cmd.Trim(); ret != nil {
		goto result
	}
	// Look for command's prefix among configured features
	for prefix, configuredFeature := range proc.Features.LookupByTrigger {
		if cmd.FindAndRemovePrefix(string(prefix)) {
			matchedFeature = configuredFeature
			break
		}
	}
	// Unknown command prefix or the requested feature is not configured
	if matchedFeature == nil {
		ret = &feature.Result{Error: ErrBadPrefix}
		goto result
	}
	// Run the feature
	log.Printf("CommandProcessor.Process: going to run %+v", cmd)
	defer func() {
		log.Printf("CommandProcessor.Process: finished with %+v - %s", cmd, ret.CombinedOutput)
	}()
	ret = matchedFeature.Execute(cmd)

result:
	ret.Command = cmd
	// Walk through result bridges
	for _, resultBridge := range proc.ResultBridges {
		if err := resultBridge.Transform(ret); err != nil {
			return &feature.Result{Command: cmd, Error: bridgeErr}
		}
	}
	return
}

// Return a realistic command processor for test cases. Only shell execution feature is available from it.
func GetTestCommandProcessor() *CommandProcessor {
	// Prepare feature set - the shell execution feature should be available even without configuration
	features := &feature.FeatureSet{}
	if err := features.Initialise(); err != nil {
		panic(err)
	}
	// Prepare realistic command bridges
	commandBridges := []bridge.CommandBridge{
		&bridge.PINAndShortcuts{PIN: "verysecret"},
		&bridge.TranslateSequences{Sequences: [][]string{{"alpha", "beta"}}},
	}
	// Prepare realistic result bridges
	resultBridges := []bridge.ResultBridge{
		&bridge.ResetCombinedText{},
		&bridge.LintText{TrimSpaces: true, MaxLength: 35},
		&bridge.SayEmptyOutput{},
		&bridge.NotifyViaEmail{},
	}
	return &CommandProcessor{
		Features:       features,
		CommandBridges: commandBridges,
		ResultBridges:  resultBridges,
	}
}
