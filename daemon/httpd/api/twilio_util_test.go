package api

import (
	"testing"
)

func TestDTMFDecode(t *testing.T) {
	if s := DTMFDecode(""); s != "" {
		t.Fatal(s)
	}
	if s := DTMFDecode(" awzxv<>?  \n\t<w;'{}   -=  "); s != "" {
		t.Fatal(s)
	}
	if s := DTMFDecode("02033004440009999"); s != " ae i  z" {
		t.Fatal(s)
	}
	if s := DTMFDecode("020330044400099990000"); s != " ae i  z   " {
		t.Fatal(s)
	}
	if s := DTMFDecode("20*330*4440**9999"); s != "aEiz" {
		t.Fatal(s)
	}
	if s := DTMFDecode("2010*220*1102220120*3013"); s != "a0B1c2D3" {
		t.Fatal(s)
	}
	goodMsg := "1234567890!@#$%^&*(`~)-_=+[{]}\\|;:'\",<.>/?abcABC"
	if s := DTMFDecode(`
	11012013014015016017018019010
	111011201130114011501160117011801190
	121012201230124012501260127012801290
	131013201330134013501360137013801390
	14101420143014401450
	202202220
	*202202220*
	`); s != goodMsg {
		t.Fatal(s)
	}
	// Some unusual typing techniques
	if s := DTMFDecode(`2*2*22*222*`); s != "aAbC" {
		t.Fatal(s)
	}
	if s := DTMFDecode(`23344456677789999`); s != "aeijnrtz" {
		t.Fatal(s)
	}
	if s := DTMFDecode(`
	2334440
	110
	566777*
	11100
	800
	9999*`); s != "aei1jnr! T Z" {
		t.Fatal(s)
	}
}

func TestSpellOutOrNot(t *testing.T) {
	if s := SpellPhonetically(""); s != "" {
		t.Fatal(s)
	}
	sample := "account1  \tme@gmail.com \n7&!x$NRj&T"
	sampleOut := `alpha, charlie, charlie, oscar, uniform, november, tango, one, space, mike, echo, at, golf, mike, alpha, india, lima, dot, charlie, oscar, mike, space, seven, ampersand, exclamation, xray, dollar, capital november, capital romeo, juliet, ampersand, capital tango`
	if s := SpellPhonetically(sample); s != sampleOut {
		t.Fatal(s)
	}
}
