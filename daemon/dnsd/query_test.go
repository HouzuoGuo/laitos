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
		{[]string{"_11"}, ""},
		{[]string{"example", "com"}, ""},
		{[]string{"_", "example", "com"}, ""},
		// Decode DTMF spaces
		{[]string{"_0", "example", "com"}, " "},
		{[]string{"_0abc", "example", "com"}, " abc"},
		// Decode DTMF numbers
		{[]string{"_a1b2", "example", "com"}, "a0ba"},
		{[]string{"_a1b2c", "example", "com"}, "a0bac"},
		{[]string{"_0a2", "example", "com"}, " aa"},
		{[]string{"_101010", "example", "com"}, "000"},
		// Decode from multiple labels
		{[]string{"_abc", "def", "example", "com"}, "abcdef"},
		{[]string{"_", "abc", "example", "com"}, "abc"},
		{[]string{"_", "11a", "12b", "13c", "example", "com"}, "1a2b3c"},
		// Decode with an empty tail label
		{[]string{"_0abc", "example", "com", ""}, " abc"},
	}
	for _, test := range tests {
		if got := DecodeDTMFCommandInput(test.labels); got != test.want {
			t.Fatalf("Labels: %v, decoded: %s, expected: %s", test.labels, got, test.want)
		}
	}
}
