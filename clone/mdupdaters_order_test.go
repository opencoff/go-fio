// mdupdaters_order_test.go -- structural guard for the metadata-restore
// pipeline order. On Linux, writing system.posix_acl_access (an xattr)
// rewrites the file's mode group bits; chmod running afterwards would
// clobber them. We enforce: chown → chmod → xattr → times.

package clone

import (
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// TestMdUpdatersOrder verifies the package-level mdUpdaters slice has
// the invariant ordering the ACL-vs-chmod correctness argument depends
// on. A later refactor that reshuffles the list without thinking through
// the Linux POSIX-ACL side-effect will trip this test.
func TestMdUpdatersOrder(t *testing.T) {
	want := []string{
		"cloneugid",
		"clonemode",
		"clonexattr",
		"clonetimes",
	}

	got := make([]string, len(mdUpdaters))
	for i, fn := range mdUpdaters {
		got[i] = funcName(fn)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mdUpdaters order:\n got  %v\n want %v", got, want)
	}
}

// funcName returns the unqualified function name of a cloner (e.g.
// "clonexattr" from "github.com/opencoff/go-fio/clone.clonexattr").
func funcName(fn cloner) string {
	name := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i+1:]
	}
	return name
}
