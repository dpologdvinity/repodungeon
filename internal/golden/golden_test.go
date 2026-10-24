package golden

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoldenAssert(t *testing.T) {
	dir := filepath.Join("testdata", "golden")
	_ = os.MkdirAll(dir, 0o755)
	file := filepath.Join(dir, "test_golden.txt")
	_ = os.WriteFile(file, []byte("hello\n"), 0o644)
	t.Cleanup(func() {
		_ = os.Remove(file)
		_ = os.Remove(dir)
	})

	// Match
	Assert(t, "test_golden", "hello\n")

	// Update branch
	oldUpdate := *update
	defer func() { *update = oldUpdate }()
	*update = true
	Assert(t, "test_golden", "updated\n")

	data, err := os.ReadFile(file)
	if err != nil || string(data) != "updated\n" {
		t.Fatalf("expected updated content, got %s, err %v", string(data), err)
	}
}

func TestGoldenMismatch(t *testing.T) {
	dir := filepath.Join("testdata", "golden")
	_ = os.MkdirAll(dir, 0o755)
	file := filepath.Join(dir, "mismatch_test.txt")
	_ = os.WriteFile(file, []byte("expected\n"), 0o644)
	t.Cleanup(func() {
		_ = os.Remove(file)
	})

	fake := &testing.T{}
	Assert(fake, "mismatch_test", "actual\n")
	if !fake.Failed() {
		t.Fatal("expected Assert to report mismatch")
	}
}
