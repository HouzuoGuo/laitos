package platform

import "testing"

func TestSetTermEcho(t *testing.T) {
	// just make sure it does not panic
	SetTermEcho(false)
	SetTermEcho(true)
}
