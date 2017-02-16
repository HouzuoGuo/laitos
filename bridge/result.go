package bridge

// Return string up to a certain length, and optionally remove non-visible non-7bit characters.
type StringLint struct {
	MaxLength               int
	KeepVisible7BitCharOnly bool
}

func (str *StringLint) Transform(in string) string {
	return
}
