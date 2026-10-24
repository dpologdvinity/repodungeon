package dungeon

import (
	"math"
	"math/bits"
)

// Epithets are the boss title suffixes. The boss seed picks one (docs/SPEC.md
// section 5.4). The order is part of the format.
var Epithets = [8]string{
	"Devourer of Weekends", "Breaker of Builds", "Hoarder of Merge Conflicts",
	"the Unrefactored", "Lord of the Hotfix", "Bane of Code Review",
	"the Ever-Changing", "Whisperer of Flaky Tests",
}

// Epithet returns the boss title for seed: Epithets[seed mod 8].
func Epithet(seed uint64) string { return Epithets[seed%8] }

// isqrt returns the integer square root of n, rounded down. n must be >= 0.
func isqrt(n int) int { return int(math.Sqrt(float64(n))) }

// clamp limits v to the range [lo, hi].
func clamp(v, lo, hi int) int { return min(max(v, lo), hi) }

// MonsterStats returns hit points and attack for a file monster.
//
// HP grows with the square root of the line count and is capped at 60. ATK
// grows with the bit length of the churn (about log2) and is capped at 8.
// The square root and the log keep huge files from becoming unbeatable.
// See docs/SPEC.md section 5.4.
func MonsterStats(lines, churn int) (hp, atk int) {
	return min(60, 2+isqrt(lines)), min(8, 1+bits.Len(uint(churn)))
}

// BossStats returns hit points and attack for the boss.
//
// The boss is the same idea as MonsterStats but with higher floors, so even a
// tiny boss file is a real fight: HP is in [30, 100] and ATK is in [4, 9].
// See docs/SPEC.md section 5.4.
func BossStats(lines, churn int) (hp, atk int) {
	return clamp(20+3*isqrt(lines), 30, 100), clamp(3+bits.Len(uint(churn))/2, 4, 9)
}

// RoomSize returns the interior size of a room holding files files.
//
// Width is in [6, 22] and height is in [3, 5]. Both grow with the square root
// of the file count. The maximum outer size (24 x 7) fits one grid cell of the
// map; see buildLevel. See docs/SPEC.md section 5.3.
func RoomSize(files int) (w, h int) {
	s := isqrt(files)
	return clamp(6+2*s, 6, 22), clamp(3+s/2, 3, 5)
}
