package main

import (
	"os"
	"testing"
)

// TestIsTTY proves that nil files, /dev/null, and pipes are not treated
// as interactive terminals.
func TestIsTTY(t *testing.T) {
	if isTTY(nil) {
		t.Fatal("expected nil file not to be TTY")
	}

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = devNull.Close() }()

	if isTTY(devNull) {
		t.Fatal("expected /dev/null not to be TTY")
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = r.Close()
		_ = w.Close()
	}()

	if isTTY(r) || isTTY(w) {
		t.Fatal("expected pipe not to be TTY")
	}
}
