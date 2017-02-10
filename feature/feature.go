package feature

// Represent a useful feature that is capable of execution and provide execution result as feedback.
type Feature interface {
	InitAndTest() error        // Prepare internal states by running configuration and tests
	TriggerPrefix() string     // Command prefix string to trigger the feature
	Execute(cmd string) Result // Feature execution and return the result
}

// Feedback from command execution that has human readable output and error - if any.
type Result interface {
	Err() error           // Execution error if there is any
	ErrText() string      // Human readable error text
	OutText() string      // Human readable normal output excluding error text
	CombinedText() string // Combined normal and error text
}
