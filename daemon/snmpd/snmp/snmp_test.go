package snmp

import (
	"encoding/asn1"
	"encoding/hex"
	"fmt"
	"testing"
)

type GetNextRequest struct {
	Version         int
	CommunityString string
}

func enc(i interface{}) {
	var src []byte
	var err error
	src, err = asn1.Marshal(i)
	if err != nil {
		panic(err)
	}
	for i, c := range hex.EncodeToString(src) {
		if i%2 == 0 {
			fmt.Print(" ")
		}
		fmt.Printf("%c", c)
	}
	fmt.Println()
}

func TestA(t *testing.T) {
	fmt.Println("\nget-next-request 1.3.6.1.2.1.1.1.1.0")
	/*

		0000   00 00 03 04 00 06 00 00 00 00 00 00 00 00 08 00
		0010   45 00 00 48 23 68 40 00 40 11 19 3b 7f 00 00 01
		                                           V
		0020   7f 00 00 01 3b 00 00 a1 00 34 fe 47 30 2a 02 01
		0030   01 04 06 70 75 62 6c 69 63 a1 1d 02 04 1b 6e 63
		0040   8a 02 01 00 02 01 00 30 0f 30 0d 06 09 2b 06 01
		0050   02 01 01 01 01 00 05 00
	*/
	fmt.Println("30") // ???????????
	fmt.Println("2a") // ???????????

	enc(1)                // v2
	enc([]byte("public")) // c

	fmt.Println("a1") // ????? A+"get-next-request(PDU 1)", an ordinary get request would be PDU 0
	fmt.Println("1d") // size of things below - 29

	enc(460219274) // req ID
	enc(0)         // no error
	enc(0)         // err index 0

	fmt.Println("30") // ????? "variable-bindings"
	fmt.Println("0f") // size of things below - 15

	fmt.Println("30") // variable-binding item 1
	fmt.Println("0d") // size of things below - 13

	enc(asn1.ObjectIdentifier{1, 3, 6, 1, 2, 1, 1, 1, 1, 0}) // deliberately non-existent OID
	enc(asn1.NullRawValue)

	fmt.Println("\nget-response 1.3.6.1.2.1.1.2.0")
	/*
		0000   00 00 03 04 00 06 00 00 00 00 00 00 00 00 08 00
		0010   45 00 00 51 23 69 40 00 40 11 19 31 7f 00 00 01
		                                           V
		0020   7f 00 00 01 00 a1 3b 00 00 3d fe 50 30 33 02 01
		0030   01 04 06 70 75 62 6c 69 63 a2 26 02 04 1b 6e 63
		0040   8a 02 01 00 02 01 00 30 18 30 16 06 08 2b 06 01
		0050   02 01 01 02 00 06 0a 2b 06 01 04 01 bf 08 03 02
		0060   0a
	*/
	fmt.Println("30") // ???????????
	fmt.Println("33") // ???????????

	enc(1)                // v2
	enc([]byte("public")) // c

	fmt.Println("a2") // ????? A+"get-response(PDU 2)"
	fmt.Println("26") // size of things below - 38

	enc(460219274) // req ID
	enc(0)         // no error
	enc(0)         // err index 0

	fmt.Println("30") // ????? "variable-bindings"
	fmt.Println("18") // size of things below - 24

	fmt.Println("30") // variable-binding item 1
	fmt.Println("16") // size of things below - 22

	enc(asn1.ObjectIdentifier{1, 3, 6, 1, 2, 1, 1, 2, 0})
	enc(asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 8072, 3, 2, 10})

	fmt.Println("\nget-request 1.3.6.1.2.1.1.1.1.0")
	/*
		0000   00 00 03 04 00 06 00 00 00 00 00 00 00 00 08 00
		0010   45 00 00 48 23 6a 40 00 40 11 19 39 7f 00 00 01
		                                           V
		0020   7f 00 00 01 3b 00 00 a1 00 34 fe 47 30 2a 02 01
		0030   01 04 06 70 75 62 6c 69 63 a0 1d 02 04 1b 6e 63
		0040   8b 02 01 00 02 01 00 30 0f 30 0d 06 09 2b 06 01
		0050   02 01 01 01 01 00 05 00

	*/
	fmt.Println("30") // ???????????
	fmt.Println("2a") // ??????????

	enc(1)                // v2
	enc([]byte("public")) // c

	fmt.Println("a0") // A+"get-request (PDU 0)"
	fmt.Println("1d") // size of things below - 29

	enc(460219275) // req ID (increases from the request above)
	enc(0)         // no error
	enc(0)         // err index 0

	enc(asn1.ObjectIdentifier{1, 3, 6, 1, 2, 1, 1, 1, 1, 0})
	enc(asn1.NullRawValue)

	fmt.Println("\nget-response 1.3.6.1.2.1.1.1.1.0 no such instance")
	/*
		0000   00 00 03 04 00 06 00 00 00 00 00 00 00 00 08 00
		0010   45 00 00 48 23 6b 40 00 40 11 19 38 7f 00 00 01
																							 V
		0020   7f 00 00 01 00 a1 3b 00 00 34 fe 47 30 2a 02 01
		0030   01 04 06 70 75 62 6c 69 63 a2 1d 02 04 1b 6e 63
		0040   8b 02 01 00 02 01 00 30 0f 30 0d 06 09 2b 06 01
		0050   02 01 01 01 01 00 81 00
	*/

	fmt.Println("30") // ???????????
	fmt.Println("2a") // ??????????

	enc(1)                // v2
	enc([]byte("public")) // c

	fmt.Println("a2") // A+"get-response (PDU 2)"
	fmt.Println("1d") // size of things below - 29

	enc(460219274) // req ID
	enc(0)         // no error
	enc(0)         // err index 0

	fmt.Println("30") // "variable-bindings"
	fmt.Println("0f") // size of things below - 15

	enc(asn1.ObjectIdentifier{1, 3, 6, 1, 2, 1, 1, 1, 1, 0})
	fmt.Println("81")
	fmt.Println("00") // noSuchInstance
}
