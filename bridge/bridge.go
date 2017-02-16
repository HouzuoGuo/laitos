package bridge

import "github.com/HouzuoGuo/websh/feature"

type CommandBridge interface {
	Transform(feature.Command) feature.Command
}

type ResultBridge interface {
	Transform(string) string
}
