// Package dungeon generates a dungeon from a scanned repo.
//
// Generation is pure and deterministic: the same repo.Repo always gives the
// same Dungeon. The package does no I/O, reads no clock and uses no global
// random state. All randomness comes from a seeded RNG (see RNG), and the seed
// comes from the head commit hash. See docs/SPEC.md section 5.
package dungeon

import "strconv"

// RNG is a SplitMix64 generator. Its output is stable across Go releases.
//
// We own this generator instead of using math/rand because the dungeon is part
// of the product format: the same commit must give the same map on every
// machine and every Go version. math/rand makes no such promise. The order of
// RNG calls during generation is also part of the format. Adding, removing or
// reordering a call changes every dungeon, so do not change it. See
// docs/SPEC.md section 5.1.
type RNG struct{ s uint64 }

// NewRNG returns a generator whose state starts at seed.
func NewRNG(seed uint64) *RNG { return &RNG{s: seed} }

// Next advances the state and returns the next 64-bit value.
//
// This is the standard SplitMix64 step: add the golden-ratio increment, then
// mix the bits with two multiply-xorshift rounds.
func (r *RNG) Next() uint64 {
	r.s += 0x9E3779B97F4A7C15
	z := r.s
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// IntN returns a value in [0, n); n must be > 0.
//
// It uses a plain modulo, so it has a tiny bias. That is fine here, and it
// keeps the format simple to reproduce. It consumes exactly one Next call.
func (r *RNG) IntN(n int) int { return int(r.Next() % uint64(n)) }

// Perm returns a permutation of 0..n-1 (Fisher-Yates, downwards).
//
// It consumes exactly n-1 IntN calls (none for n <= 1).
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
//
// The hash must have at least 16 bytes, or the slice panics. A parse error is
// ignored on purpose: it gives 0 or a partial value, which is still a valid
// seed.
func SeedFromHash(hash string) uint64 {
	v, _ := strconv.ParseUint(hash[:16], 16, 64)
	return v
}
