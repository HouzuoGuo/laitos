package main

import "testing"

func TestMysteriousInvokeAPI(t *testing.T) {
	t.Skip()
	sh := MysteriousClient{
		URL:   "FILLME",
		Addr1: "FILLME",
		Addr2: "FILLME",
		ID1:   "FILLME",
		ID2:   "FILLME",
	}
	sh.InvokeAPI("hi there")
}
