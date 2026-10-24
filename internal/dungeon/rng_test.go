package dungeon

import (
	"slices"
	"testing"
)

// TestRNGVectors pins the exact output of Next, Perm, IntN and SeedFromHash.
// These values are part of the dungeon format, so a change here means every
// existing dungeon changes.
func TestRNGVectors(t *testing.T) {
	r := NewRNG(0)
	for _, want := range []uint64{0xe220a8397b1dcdaf, 0x6e789e6aa1b965f4, 0x06c45d188009454f} {
		if got := r.Next(); got != want {
			t.Fatalf("Next %x want %x", got, want)
		}
	}
	if p := NewRNG(0).Perm(5); !slices.Equal(p, []int{2, 3, 1, 4, 0}) {
		t.Fatalf("Perm %v", p)
	}
	r = NewRNG(7)
	var got []int
	for i := 0; i < 5; i++ {
		got = append(got, r.IntN(10))
	}
	if !slices.Equal(got, []int{7, 4, 6, 3, 4}) {
		t.Fatalf("IntN %v", got)
	}
	if SeedFromHash("0123456789abcdef0123456789abcdef01234567") != 0x0123456789abcdef {
		t.Fatal("SeedFromHash")
	}
}

// TestStats checks the stat formulas (MonsterStats, BossStats, RoomSize,
// Epithet) at their low, middle and capped values.
func TestStats(t *testing.T) {
	for _, c := range [][4]int{{0, 0, 2, 1}, {100, 0, 12, 1}, {100, 1, 12, 2}, {9, 3, 5, 3}, {10000, 1000, 60, 8}} {
		if hp, atk := MonsterStats(c[0], c[1]); hp != c[2] || atk != c[3] {
			t.Errorf("MonsterStats(%d,%d)=%d,%d", c[0], c[1], hp, atk)
		}
	}
	for _, c := range [][4]int{{100, 40, 50, 6}, {0, 0, 30, 4}, {10000, 100000, 100, 9}} {
		if hp, atk := BossStats(c[0], c[1]); hp != c[2] || atk != c[3] {
			t.Errorf("BossStats(%d,%d)=%d,%d", c[0], c[1], hp, atk)
		}
	}
	for _, c := range [][3]int{{0, 6, 3}, {9, 12, 4}, {100, 22, 5}} {
		if w, h := RoomSize(c[0]); w != c[1] || h != c[2] {
			t.Errorf("RoomSize(%d)=%d,%d", c[0], w, h)
		}
	}
	if Epithet(0) != "Devourer of Weekends" || Epithet(15) != "Whisperer of Flaky Tests" {
		t.Error("Epithet")
	}
}
