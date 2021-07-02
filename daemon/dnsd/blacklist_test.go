package dnsd

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/HouzuoGuo/laitos/lalog"
)

func TestDownloadAllBlacklists(t *testing.T) {
	names := DownloadAllBlacklists(BlacklistMaxEntries, lalog.Logger{})
	if len(names) < 5000 {
		t.Fatal("number of names is too little")
	}
	for _, name := range names {
		for _, allowed := range Whitelist {
			if name == allowed {
				t.Fatal("did not remove white listed name ", name)
			}
		}
	}
}

func TestExtractNamesFromHostsContent(t *testing.T) {
	sample := fmt.Sprintf(`# ha
# other formats:  https://
# policy:         https://###
#
# this name is way too short
0.0.0.0 ha
127.0.0.1 t.co

# some comments
127.0.0.1 01234.com
0.0.0.0 56789.CoM # comment haha
1.2.3.4 1234.CoM%c # conains invalid NULL character
# some comments
`, 0)
	names := ExtractNamesFromHostsContent(sample)
	if !reflect.DeepEqual(names, []string{"t.co", "01234.com", "56789.com"}) {
		t.Fatal(names)
	}
}
