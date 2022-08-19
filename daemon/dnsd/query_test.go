package dnsd

import (
	"testing"
)

func TestDecodeDTMFCommandInput(t *testing.T) {
	var tests = []struct {
		labels []string
		want   string
	}{
		{nil, ""},
		{[]string{"_"}, ""},
		{[]string{"_11"}, "1"},
		// Decode DTMF spaces
		{[]string{"_0"}, " "},
		{[]string{"_0abc"}, " abc"},
		// Decode DTMF numbers
		{[]string{"_a1b2"}, "a0ba"},
		{[]string{"_a1b2c"}, "a0bac"},
		{[]string{"_0a2"}, " aa"},
		{[]string{"_101010"}, "000"},
		// Decode from multiple labels
		{[]string{"_abc", "def"}, "abcdef"},
		{[]string{"_", "abc"}, "abc"},
		{[]string{"_", "11a", "12b", "13c"}, "1a2b3c"},
	}
	for _, test := range tests {
		if got := DecodeDTMFCommandInput(test.labels); got != test.want {
			t.Errorf("Labels: %v, decoded: %s, expected: %s", test.labels, got, test.want)
		}
	}
}

func TestCountNameLabels(t *testing.T) {
	var tests = []struct {
		in   string
		want int
	}{
		{"", 0},
		{".", 0},
		{"a.", 1},
		{"a.b", 2},
		{".a.b", 2},
		{".a.b.", 2},
	}
	for _, test := range tests {
		if got := CountNameLabels(test.in); got != test.want {
			t.Errorf("CountNameLabels(%q): got %v, want %v", test.in, got, test.want)
		}
	}
}
