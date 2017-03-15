package named

import "testing"

func TestDNSD_StartAndBlock(t *testing.T) {
	t.Skip()
	daemon := DNSD{ForwardTo: "8.8.8.8"}
	if err := daemon.StartAndBlock(); err != nil {
		t.Fatal(err)
	}
}
