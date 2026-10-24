package testrepo

import (
	"testing"
	"time"
)

func TestCommitUsesFixedDatesAndAuthor(t *testing.T) {
	r := New(t)
	r.Write("a/b.txt", "hi\n")
	h1 := r.Commit("Ada", "ada@example.com", "first")
	r.Commit("Bob", "bob@example.com", "second")
	if len(h1) != 40 {
		t.Fatalf("hash %q", h1)
	}
	got := r.Git("log", "--reverse", "--format=%an|%ae|%aI|%cI|%cn")
	want := "Ada|ada@example.com|2024-01-15T12:00:00Z|2024-01-15T12:00:00Z|Fixture\n" +
		"Bob|bob@example.com|2024-02-15T12:00:00Z|2024-02-15T12:00:00Z|Fixture\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if !Date(12).Equal(time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)) {
		t.Fatal("Date(12)")
	}
}
