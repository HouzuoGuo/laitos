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

func (proc *CommandProcessor) Process(cmd feature.Command) (ret *feature.Result) {
	log.Printf("CommandProcessor.Process: going to run %+v", cmd)
	defer func() {
		log.Printf("CommandProcessor.Process: finished with %+v - %s", cmd, ret.CombinedOutput)
	}()
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
	for prefix, configuredFeature := range proc.Features.LookupByPrefix {
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
