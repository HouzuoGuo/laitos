package dnsd

import (
	"github.com/HouzuoGuo/laitos/misc"
	"reflect"
	"testing"
)

func TestDownloadAllBlacklists(t *testing.T) {
	if names := DownloadAllBlacklists(misc.Logger{}); len(names) < 5000 {
		t.Fatal("number of names is too little")
	}
}

func TestExtractNamesFromHostsContent(t *testing.T) {
	sample := `# ha
# other formats:  https://
# policy:         https://###
#
# this name is way too short
0.0.0.0 ha
127.0.0.1 t.co

# some comments
127.0.0.1 01234.com
0.0.0.0 56789.CoM # comment haha
# some comments
`
	names := ExtractNamesFromHostsContent(sample)
	if !reflect.DeepEqual(names, []string{"t.co", "01234.com", "56789.com"}) {
		t.Fatal(names)
	}
}

func TestUniqueStrings(t *testing.T) {
	ret := UniqueStrings([]string{"a", "b"}, []string{"b", "c"}, []string{"a", "d"})
	if len(ret) != 4 {
		t.Fatal(ret)
	}
}
