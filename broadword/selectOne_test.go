// Copyright (C) Serverplumber. All Rights Reserved.

package broadword

import (
	"math/rand/v2"
	"testing"
)

// naiveSelectOne is a brute-force reference implementation of selectOne,
// used as the oracle for differential testing. It has to be obviously
// correct by construction, so it should not share any of selectOne's
// broadword bit tricks.
func naiveSelectOne(x uint64, k int) int {
	if k < 0 || k >= 64 {
		return 64
	}
	s := uint64(k) + 1
	var r uint64 = 0
	for i := range 64 {
		r += (x >> i) & 1
		if r == s {
			return i
		}
	}
	return 64
}

// TestSelect64 checks select64 against known edge cases, comparing it
// against naiveSelect64 rather than hand-computed constants.
func TestSelectOne(t *testing.T) {
	tests := []struct {
		name string
		x    uint64
		k    int
		want int
	}{
		{name: "empty word",
			x: 0, k: 0, want: 64},
		{name: "first set bit",
			x: 1<<1 | 1<<3, k: 0, want: 1}, // shift ones into position.
		{name: "last valid bit for x",
			x: 1<<1 | 1<<3, k: 1, want: 3},
		{name: "all bits set, first bit",
			x: ^uint64(0), k: 0, want: 0},
		{name: "all bits set, last bit",
			x: ^uint64(0), k: 63, want: 63},
		{name: "negative k",
			x: 1<<1 | 1<<3, k: -1, want: 64},
		{name: "k >= 64",
			x: ^uint64(0), k: 64, want: 64},
		{name: "k >= popcount(x), within range",
			x: 1<<0 | 1<<1 | 1<<2 | 1<<3 | 1<<4 | 1<<5, k: 6, want: 64},
		{name: "k within a single byte",
			x: 1<<0 | 1<<1 | 1<<2 | 1<<3 | 1<<5 | 1<<7, k: 3, want: 3},
		{name: "answer crosses into second byte",
			x: 1<<13 - 1, k: 12, want: 12},
		{name: "answer skips a gap crossing further into the word",
			x: 1<<12 - 1 | 1<<13 | 1<<33, k: 13, want: 33},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SelectOne(tt.x, tt.k); got != tt.want {
				t.Errorf("select64(%#x, %d) = %d, want %d", tt.x, tt.k, got, tt.want)
			}
		})
	}
}

// FuzzSelect64 differentially tests select64 against naiveSelect64 across
// a wide range of (x, k) inputs, to cover cases a hand-written table won't
// think to include.

func FuzzSelectOne(f *testing.F) {
	f.Add(uint64(0), 0)
	f.Add(uint64(1<<1|1<<3), 0)
	f.Add(uint64(1<<13-1), 12)
	f.Fuzz(func(t *testing.T, x uint64, k int) {
		if SelectOne(x, k) != naiveSelectOne(x, k) {
			t.Fatalf("select64(%#x, %d) != naiveSelect64(%#x, %d)", x, k, x, k)
		}
	})
}

var sink int

// genPairs returns n (x, k) pairs for benchmarking selectOne.
//
// x is generated bit-by-bit, each bit independently set with probability p,
// so popcount(x) follows Binomial(64, p) -- p=0.85 mimics a word near
// defaultMaxLoad, a lower p a sparser one. k is drawn uniformly from
// [0, 64), independent of x, so whether a given pair is a hit
// (k < popcount(x)) or a miss falls out of that relationship rather than
// being chosen separately: the resulting hit rate is a consequence of p,
// not an extra parameter to juggle.
func genPairs(n int, p float64) []struct {
	x uint64
	k int
} {
	pairs := make([]struct {
		x uint64
		k int
	}, n)
	for i := range pairs {
		var x uint64
		for bit := range 64 {
			if rand.Float64() < p {
				x |= 1 << bit
			}
		}
		pairs[i].x = x
		pairs[i].k = rand.IntN(64)
	}
	return pairs
}

// BenchmarkSelectOne measures selectOne's cost per call. It sits on the
// CQF's hot path (rank/select over slot metadata words), so this is what
// would catch a regression in it going forward.
//
// Inputs are precomputed by genPairs before the timer starts, so setup
// cost isn't counted, and cycled through during the loop so the CPU's
// branch predictor doesn't just memorize a single repeated outcome for
// the early-return check at the top of selectOne.
func BenchmarkSelectOne(b *testing.B) {
	pairs := genPairs(1024, 0.85)
	b.Run("generic", func(b *testing.B) {
		var r int

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p := pairs[i%len(pairs)]
			r = updateSelectOne(p.x, p.k)
		}
		sink = r
	})
	b.Run("arch-specific", func(b *testing.B) {
		if archAvailableSelectOne() {
			var r int
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				p := pairs[i%len(pairs)]
				r = archUpdateSelectOne(p.x, p.k)
			}
			sink = r
		} else {
			b.Skip("no arch-specific selectOne")
		}
	})
}
