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
func naiveSelectOne(x uint64, n int) int {
	if n < 0 || n >= 64 {
		return 64
	}
	s := uint64(n) + 1
	var r uint64 = 0
	for i := range 64 {
		r += (x >> i) & 1
		if r == s {
			return i
		}
	}
	return 64
}

// selectOneCases is the edge-case table, shared by TestSelectOne here and by
// TestSelectPDEP in selectOne_amd64_test.go.
//
// It lives in this file precisely because this file carries no build
// constraint, so it compiles into every configuration and there is exactly one
// copy. A duplicate in the tagged file would drift the first time either side
// gained a case, and the amd64 path would quietly stop testing it.
//
// Unlike the fuzz targets, these want values are hand-computed rather than
// taken from naiveSelectOne. That is deliberate: an oracle shared by every
// assertion cannot catch a misconception baked into the oracle itself, so the
// table is the one place the expected answers are written out independently.
var selectOneCases = []struct {
	name string
	x    uint64
	n    int
	want int
}{
	{name: "empty word",
		x: 0, n: 0, want: 64},
	{name: "first set bit",
		x: 1<<1 | 1<<3, n: 0, want: 1}, // shift ones into position.
	{name: "last valid bit for x",
		x: 1<<1 | 1<<3, n: 1, want: 3},
	{name: "all bits set, first bit",
		x: ^uint64(0), n: 0, want: 0},
	{name: "all bits set, last bit",
		x: ^uint64(0), n: 63, want: 63},
	{name: "negative n",
		x: 1<<1 | 1<<3, n: -1, want: 64},
	{name: "n >= 64",
		x: ^uint64(0), n: 64, want: 64},
	{name: "n >= popcount(x), within range",
		x: 1<<0 | 1<<1 | 1<<2 | 1<<3 | 1<<4 | 1<<5, n: 6, want: 64},
	{name: "n within a single byte",
		x: 1<<0 | 1<<1 | 1<<2 | 1<<3 | 1<<5 | 1<<7, n: 3, want: 3},
	{name: "answer crosses into second byte",
		x: 1<<13 - 1, n: 12, want: 12},
	{name: "answer skips a gap crossing further into the word",
		x: 1<<12 - 1 | 1<<13 | 1<<33, n: 13, want: 33},
}

// TestSelectOne checks the portable implementation and the exported entry
// point against selectOneCases. Both run everywhere: genericSelectOne is the
// implementation under test, and SelectOne exercises whatever init picked,
// which on a non-amd64 build is the only coverage the exported API gets.
func TestSelectOne(t *testing.T) {
	for _, tt := range selectOneCases {
		t.Run("generic/"+tt.name, func(t *testing.T) {
			if got := genericSelectOne(tt.x, tt.n); got != tt.want {
				t.Errorf("genericSelectOne(%#x, %d) = %d, want %d", tt.x, tt.n, got, tt.want)
			}
		})
		t.Run("dispatch/"+tt.name, func(t *testing.T) {
			if got := SelectOne(tt.x, tt.n); got != tt.want {
				t.Errorf("SelectOne(%#x, %d) = %d, want %d", tt.x, tt.n, got, tt.want)
			}
		})
	}
}

// FuzzGenericSelectOne differentially tests genericSelectOne against naiveSelectOne across
// a wide range of (x, n) inputs, to cover cases a hand-written table won't
// think to include.
func FuzzGenericSelectOne(f *testing.F) {
	f.Add(uint64(0), 0)
	f.Add(uint64(1<<1|1<<3), 0)
	f.Add(uint64(1<<13-1), 12)
	f.Fuzz(func(t *testing.T, x uint64, n int) {
		want := naiveSelectOne(x, n)
		if genericSelectOne(x, n) != want {
			t.Fatalf("genericSelectOne(%#x, %d) != naiveSelectOne(%#x, %d)", x, n, x, n)
		}
	})
}

// sink keeps a benchmark's final result reachable so the loop body can't be
// optimised away. b.Loop already stops the compiler from eliding the calls
// themselves, but the baseline sub-benchmarks make no call, and they need to
// measure the same loop overhead the others carry.
var sink int

const (
	// benchN is how many precomputed inputs the benchmarks cycle through.
	// It MUST be a power of two: the loops index with i&benchMask, not
	// i%benchN, because len() of a slice is not a compile-time constant and
	// a modulo against one compiles to a hardware DIVQ. That divide costs
	// more than either implementation being measured, and it sits inside the
	// timed region, hiding the difference the benchmark exists to show.
	benchN    = 1024
	benchMask = benchN - 1

	// benchDensity is the probability that any given bit of a generated word
	// is set. 0.85 is the fill factor of a loaded counting quotient filter,
	// which is the workload this density is meant to mimic. Lower it for
	// sparser words; the generic path takes its early return more often on a
	// miss, so a sparse workload will look faster.
	benchDensity = 0.85
)

type pair struct {
	x uint64
	n int
}

// genPairs returns count (x, n) pairs for the benchmarks.
//
// Each bit of x is set independently with probability p, so popcount(x)
// follows Binomial(64, p). n is drawn uniformly from [0, 64) and independently
// of x, which makes the hit rate -- the fraction of pairs where n <
// popcount(x), so the answer is a real position rather than 64 -- a
// consequence of p rather than a second knob to juggle:
// P(hit) = E[popcount]/64 = p, exactly.
//
// The generator is seeded explicitly. math/rand/v2's top-level functions draw
// from a per-process randomly seeded source, which would hand every run a
// different input set and fold that variance into any A/B comparison.
func genPairs(count int, p float64) []pair {
	rng := rand.New(rand.NewPCG(1, 2))
	pairs := make([]pair, count)
	for i := range pairs {
		var x uint64
		for bit := range 64 {
			if rng.Float64() < p {
				x |= 1 << bit
			}
		}
		pairs[i] = pair{x: x, n: rng.IntN(64)}
	}
	return pairs
}

// BenchmarkSelectOne measures the portable implementation and the exported
// entry point. The amd64 assembly is measured by BenchmarkSelectPDEP, in
// selectOne_amd64_test.go; naming selectPDEP from this file is impossible, and
// reaching it through a function value would fold an indirect call into one
// side of the comparison and not the other.
//
// Read the results in pairs:
//
//	throughput/generic vs BenchmarkSelectPDEP/throughput
//	    the two implementations, each behind a direct call. This is the
//	    apples-to-apples comparison.
//	BenchmarkSelectPDEP/throughput vs throughput/dispatch
//	    the price of resolving selectOne through a package variable, which
//	    the compiler cannot devirtualise.
//	throughput/baseline
//	    the loop with no select in it at all. Subtract it before quoting a
//	    ratio: at these magnitudes it is not negligible.
//
// Throughput vs latency is the other axis. The throughput loops issue
// independent calls and the CPU overlaps them, which is what a bulk scan over
// a bit vector sees. The latency loops feed each result back into the next
// index, serialising the calls, which is closer to a pointer-chasing walk down
// a rank/select structure. A latency iteration also contains one dependent L1
// load, so read those numbers against each other and against their baseline,
// not as a cycle count for the function alone.
//
// Inputs are precomputed before the loop. b.Loop starts the timer on its first
// call, so setup is excluded without an explicit b.ResetTimer.
func BenchmarkSelectOne(b *testing.B) {
	pairs := genPairs(benchN, benchDensity)

	b.Run("throughput", func(b *testing.B) {
		b.Run("baseline", func(b *testing.B) {
			i, r := 0, 0
			for b.Loop() {
				r = pairs[i&benchMask].n
				i++
			}
			sink = r
		})
		b.Run("generic", func(b *testing.B) {
			i, r := 0, 0
			for b.Loop() {
				p := pairs[i&benchMask]
				i++
				r = genericSelectOne(p.x, p.n)
			}
			sink = r
		})
		b.Run("dispatch", func(b *testing.B) {
			i, r := 0, 0
			for b.Loop() {
				p := pairs[i&benchMask]
				i++
				r = SelectOne(p.x, p.n)
			}
			sink = r
		})
	})

	b.Run("latency", func(b *testing.B) {
		b.Run("baseline", func(b *testing.B) {
			i, r := 0, 0
			for b.Loop() {
				r = pairs[(i+r)&benchMask].n
				i++
			}
			sink = r
		})
		b.Run("generic", func(b *testing.B) {
			i, r := 0, 0
			for b.Loop() {
				p := pairs[(i+r)&benchMask]
				i++
				r = genericSelectOne(p.x, p.n)
			}
			sink = r
		})
		b.Run("dispatch", func(b *testing.B) {
			i, r := 0, 0
			for b.Loop() {
				p := pairs[(i+r)&benchMask]
				i++
				r = SelectOne(p.x, p.n)
			}
			sink = r
		})
	})
}
