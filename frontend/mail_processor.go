package frontend

import (
	"strings"
)

type MailProcessorConfig struct {
	PIN string
}

type MailProcessor struct {
	Features *FeatureSet
	Config   MailProcessorConfig
}
