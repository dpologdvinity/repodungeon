package dungeon

import (
	"math"
	"math/bits"
)

var Epithets = [8]string{
	"Devourer of Weekends", "Breaker of Builds", "Hoarder of Merge Conflicts",
	"the Unrefactored", "Lord of the Hotfix", "Bane of Code Review",
	"the Ever-Changing", "Whisperer of Flaky Tests",
}

func Epithet(seed uint64) string { return Epithets[seed%8] }

func isqrt(n int) int { return int(math.Sqrt(float64(n))) }

func clamp(v, lo, hi int) int { return min(max(v, lo), hi) }

func MonsterStats(lines, churn int) (hp, atk int) {
	return min(60, 2+isqrt(lines)), min(8, 1+bits.Len(uint(churn)))
}

func BossStats(lines, churn int) (hp, atk int) {
	return clamp(20+3*isqrt(lines), 30, 100), clamp(3+bits.Len(uint(churn))/2, 4, 9)
}

// RoomSize returns the interior size of a room holding files files.
func RoomSize(files int) (w, h int) {
	s := isqrt(files)
	return clamp(6+2*s, 6, 22), clamp(3+s/2, 3, 5)
}
