package dnspe

// State is the transmission control connection state.
type State int

const (
	StateInitial = State(0)
	StateClosed  = State(100)
)
