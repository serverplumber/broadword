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
// 64 bits is not an arbitrary choice of width here, it is where the
// technique fits exactly. A lane has to be wide enough to hold a count as
// large as the word itself, and the lanes have to tile the word for the
// prefix-sum multiply to work, so eight 8-bit lanes covering 64 bits leaves
// nothing spare in either direction. Byte lanes keep working up to a 255-bit
// word and stop there, since a lane can no longer hold the count.
//
// [1] Vigna, Broadword Implementation of Rank/Select Queries, WEA 2008.
// https://vigna.di.unimi.it/ftp/papers/Broadword.pdf
package broadword

// SelectOne returns the position of the n-th 1 in the 64-bit word x.
// n is 0-based, so n=0 returns the position of the first 1.
// The result is 64 if x contains n or fewer one bits.
func SelectOne(x uint64, n int) int {
	return selectOne(x, n)
}
