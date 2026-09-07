// Copyright (C) Serverplumber. All Rights Reserved.

// Package broadword provides word-level select over uint64.
//
// SelectOne given a word and an index n, it returns the
// position of the n'th set bit. It underpins rank/select structures --
// succinct bit vectors, Elias-Fano sequences, quotient filters -- where it
// runs on the hot path and its cost sets the cost of everything above it.
//
// The portable implementation follows Vigna's broadword algorithm [1],
// treating the word as eight byte lanes and reducing across them with
// multiplication and masking rather than a lookup table. On amd64 with
// suitable hardware support, an assembly routine using BMI2's PDEP is
// selected instead. Selection happens once and is transparent to callers;
// both paths return identical results for all inputs.
//
// 64 bits is not an arbitrary choice of width here. The lanes have to tile
// the word for the prefix-sum multiply to work, and each lane has to hold a
// running count as large as the word itself while leaving its top bit free:
// lanesAtMost and lanesNonZero write their marks there, and countMarkedLanes
// sums them. Counts of 0..64 need seven bits, so eight 8-bit lanes over 64
// bits leaves exactly the one bit the reduction needs. Byte lanes carry the
// technique to a 128-bit word and stop there, since beyond that a prefix
// count can reach the marker bit and the subtraction in lanesAtMost borrows
// across the lane boundary.
//
// [1] Vigna, Broadword Implementation of Rank/Select Queries, WEA 2008.
// https://vigna.di.unimi.it/ftp/papers/Broadword.pdf
package broadword

// SelectOne returns the position of the n-th 1 in the 64-bit word x.
// n is 0-based, so n=0 returns the position of the first 1.
// The result is 64 when there is no such bit: if n is negative, or if x
// contains n or fewer one bits.
func SelectOne(x uint64, n int) int {
	return selectOne(x, n)
}
