package snmp

import (
	"encoding/asn1"
	"testing"
)

func TestGetNode(t *testing.T) {
	nodeFun, exists := GetNode(asn1.ObjectIdentifier{9})
	if nodeFun != nil || exists {
		t.Fatal(exists)
	}
	nodeFun, exists = GetNode(asn1.ObjectIdentifier{1, 3, 6})
	if nodeFun != nil || exists {
		t.Fatal(exists)
	}
	nodeFun, exists = GetNode(asn1.ObjectIdentifier{1, 3, 6, 1, 2, 1, 1, 1, 1, 0})
	if nodeFun != nil || exists {
		t.Fatal(exists)
	}
	nodeFun, exists = GetNode(asn1.ObjectIdentifier{1, 3, 6, 1, 2, 1, 1, 1, 1, 100})
	if nodeFun != nil || exists {
		t.Fatal(exists)
	}
	nodeFun, exists = GetNode(asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 52535, 121})
	if nodeFun != nil || exists {
		t.Fatal(exists)
	}
	nodeFun, exists = GetNode(asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 52535, 121, 1})
	if nodeFun != nil || exists {
		t.Fatal(exists)
	}
	nodeFun, exists = GetNode(asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 52535, 121, 100})
	if nodeFun == nil || !exists {
		t.Fatal(exists)
	}
	if publicIP := nodeFun(); publicIP == "" {
		t.Fatal("did not fetch public IP")
	}
}

func TestAllOIDNodes(t *testing.T) {
	// None of the nodes is supposed to return nil
	if len(OIDNodes) != len(OIDSuffixList) {
		t.Fatal(OIDSuffixList)
	}
	for suffix, nodeFun := range OIDNodes {
		if v := nodeFun(); v == nil {
			t.Fatal(suffix, "is not supposed to respond with nil data")
		}
	}
}

func TestGetNextNode(t *testing.T) {
	oid, endOfView := GetNextNode(asn1.ObjectIdentifier{9})
	if !oid.Equal(FirstOID) || endOfView {
		t.Fatal(oid, endOfView)
	}
	oid, endOfView = GetNextNode(asn1.ObjectIdentifier{1, 3, 6})
	if !oid.Equal(FirstOID) || endOfView {
		t.Fatal(oid, endOfView)
	}
	oid, endOfView = GetNextNode(asn1.ObjectIdentifier{1, 3, 6, 1, 2, 1, 1, 1, 1, 0})
	if !oid.Equal(FirstOID) || endOfView {
		t.Fatal(oid, endOfView)
	}
	oid, endOfView = GetNextNode(asn1.ObjectIdentifier{1, 3, 6, 1, 2, 1, 1, 1, 1, 100})
	if !oid.Equal(FirstOID) || endOfView {
		t.Fatal(oid, endOfView)
	}
	oid, endOfView = GetNextNode(asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 52535, 121})
	if !oid.Equal(FirstOID) || endOfView {
		t.Fatal(oid, endOfView)
	}
	oid, endOfView = GetNextNode(asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 52535, 121, 100})
	if !oid.Equal(asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 52535, 121, 101}) || endOfView {
		t.Fatal(oid, endOfView)
	}
	oid, endOfView = GetNextNode(asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 52535, 121, 115})
	if !oid.Equal(asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 52535, 121, 115}) || !endOfView {
		t.Fatal(oid, endOfView)
	}
	// Not entirely sure if this one conforms to SNMP standard:
	oid, endOfView = GetNextNode(asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 52535, 121, 116})
	if !oid.Equal(FirstOID) || endOfView {
		t.Fatal(oid, endOfView)
	}
}

func TestEncode(t *testing.T) {
	//b, err := asn1.Marshal(asn1.ObjectIdentifier{1,3,6,1,2,1,1,1,1,0})
	//
	b, err := asn1.Marshal(asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 52535, 121, 115})
	t.Logf("%#v, %v", b, err)
}
