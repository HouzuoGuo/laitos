// Daemon services that act as IO front-end to features, while using bridges to transform IO content.
package frontend

import (
	"encoding/json"
	"github.com/HouzuoGuo/websh/bridge"
	"github.com/HouzuoGuo/websh/feature"
)

// Expose features over HTTP API endpoint.
type FeaturesHTTPEndpoint struct {
	Features       *feature.FeatureSet
	CommandBridges []bridge.CommandBridge
	ResultBridges  []bridge.ResultBridge
}

func (httpapi *FeaturesHTTPEndpoint) Initialise() {

}

func (httpapi *FeaturesHTTPEndpoint) RunCommand(cmd feature.Command) (ret *feature.Result) {
	return nil
}

// A front-end daemon is capable of starting, stopping, self-testing.
type FrontendDaemon interface {
	InitialiseByJSON(json.RawMessage) error
	StartAndBlock() error
	SelfTest() error
	Stop() error
}
