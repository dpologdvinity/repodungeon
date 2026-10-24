// Package dungeon generates a deterministic dungeon from a scanned repo.
package dungeon

import "strconv"

// RNG is a SplitMix64 generator. Its output is stable across Go releases.
type RNG struct{ s uint64 }

func NewRNG(seed uint64) *RNG { return &RNG{s: seed} }

func (r *RNG) Next() uint64 {
	r.s += 0x9E3779B97F4A7C15
	z := r.s
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// IntN returns a value in [0, n); n must be > 0.
func (r *RNG) IntN(n int) int { return int(r.Next() % uint64(n)) }

// Perm returns a permutation of 0..n-1 (Fisher-Yates, downwards).
func (r *RNG) Perm(n int) []int {
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	for i := n - 1; i >= 1; i-- {
		j := r.IntN(i + 1)
		p[i], p[j] = p[j], p[i]
	}
	return p
}

// SeedFromHash parses the first 16 hex digits of a commit hash.
func SeedFromHash(hash string) uint64 {
	v, _ := strconv.ParseUint(hash[:16], 16, 64)
	return v
}
