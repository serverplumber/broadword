//go:build amd64 && !purego

// Copyright (C) Serverplumber. All Rights Reserved.

package broadword

import (
	"testing"
)

// TestSelectPDEP checks the amd64 assembly against selectOneCases, which is
// defined in selectOne_test.go -- that file has no build constraint, so it
// compiles alongside this one and both tests read from a single table.
func TestSelectPDEP(t *testing.T) {
	if !hasBMI() {
		t.Skip("BMI/BMI2 unavailable. PDEP cannot execute.")
	}
	for _, tt := range selectOneCases {
		t.Run("assembly/"+tt.name, func(t *testing.T) {
			if got := selectPDEP(tt.x, tt.n); got != tt.want {
				t.Errorf("selectPDEP(%#x, %d) = %d, want %d", tt.x, tt.n, got, tt.want)
			}
		})
	}
}

func FuzzSelectPDEP(f *testing.F) {
	if !hasBMI() {
		f.Skip("BMI/BMI2 unavailable. PDEP cannot execute.")
	}
	f.Add(uint64(0), 0)
	f.Add(uint64(1<<1|1<<3), 0)
	f.Add(uint64(1<<13-1), 12)
	f.Fuzz(func(t *testing.T, x uint64, n int) {
		want := naiveSelectOne(x, n)
		if selectPDEP(x, n) != want {
			t.Fatalf("selectPDEP(%#x, %d) != naiveSelectOne(%#x, %d)", x, n, x, n)
		}
	})
}

// BenchmarkSelectPDEP measures the amd64 assembly through a direct call, so it
// can be compared against BenchmarkSelectOne/throughput/generic without a
// function-pointer indirection on one side and not the other.
//
// It lives in a build-tagged file because selectPDEP is only declared under
// amd64 && !purego. Test files honour build constraints exactly as ordinary
// files do, and every test file in a package shares the package's scope, so
// genPairs, pair, sink and the bench constants all come from selectOne_test.go.
//
// See BenchmarkSelectOne for what the sub-benchmarks mean and how to pair them
// up.
func BenchmarkSelectPDEP(b *testing.B) {
	if !archAvailableSelectOne() {
		b.Skip("BMI2 unavailable, or PDEP is microcoded on this CPU")
	}
	pairs := genPairs(benchN, benchDensity)

	b.Run("throughput", func(b *testing.B) {
		i, r := 0, 0
		for b.Loop() {
			p := pairs[i&benchMask]
			i++
			r = selectPDEP(p.x, p.n)
		}
		sink = r
	})

	b.Run("latency", func(b *testing.B) {
		i, r := 0, 0
		for b.Loop() {
			p := pairs[(i+r)&benchMask]
			i++
			r = selectPDEP(p.x, p.n)
		}
		sink = r
	})
}
