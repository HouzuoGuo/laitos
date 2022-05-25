package tcpoverdns

// State is the transmission control stream's state.
type State int

const (
	StateEmpty       = State(0)
	StateSynSent     = State(1)
	StateSynReceived = State(2)
	StatePeerAck     = State(3)
	StateEstablished = State(4)
	// TODO: add FIN and perhaps FIN ACK.
	StateClosed = State(100)
)

// Flag is transmitted with each segment, it is the data type of an individual
// flag bit while also used to represent an entire collection of flags.
// Transmission control and its peer use flags to communicate transition between
// states.
type Flag uint16

const (
	FlagSyn     = Flag(1)
	FlagAck     = Flag(2)
	FlagReset   = Flag(3)
	FlagFinnish = Flag(4)
)

func (flag Flag) Has(f Flag) bool {
	return flag^f != 0
}

func (flag *Flag) Set(f Flag) {
	*flag |= f
}

func (flag *Flag) Clear(f Flag) {
	*flag &^= f
}
