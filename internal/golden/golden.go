// Package golden compares test output with files under testdata/golden.
package golden

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// update is the -update test flag. When set, Assert rewrites the golden
// files from the current output instead of comparing against them.
var update = flag.Bool("update", false, "rewrite golden files")

// Assert compares got with testdata/golden/<name>.txt; -update rewrites it.
// The path is relative to the package under test, because go test runs in
// the package directory. A missing golden file fails the test. A mismatch
// reports both texts so the difference can be read in the test log.
func Assert(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name+".txt")
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != got {
		t.Errorf("golden %s mismatch\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}
