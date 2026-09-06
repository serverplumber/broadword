// Copyright (C) Serverplumber. All Rights Reserved.

package broadword

// Package-level constants. lowN has bit 0 set in every N-bit lane; multiplying
// a small value v by lowN replicates v into all lanes, which is how every mask
// below is written. highN is the top bit of each lane.
//
//	0xA * low4 == 0xAAAAAAAAAAAAAAAA
//	0xF * low8 == 0x0F0F0F0F0F0F0F0F
const (
	low4  = 0x1111111111111111
	low8  = 0x0101010101010101
	high8 = 0x80 * low8

	// Bit j set in lane j, nothing else. Broadcasting a byte across all eight
	// lanes and masking with this leaves lane j holding just bit j of that byte
	// -- the diagonal of the 8x8 matrix the broadcast produced.
	diagonal8 = 0x8040201008040201
)

// lanesAtMost marks bit 7 of lane i when prefix[i] <= threshold[i].
//
// Vigna's general <=_8 needs eight operators because a lane's top bit may be
// set on either side. Here it never is: prefix lanes hold counts <= 64 and
// thresholds are ranks <= 63, so per lane the subtraction evaluates to
// 128+t-c, which lands in [64,191] -- never borrows, never leaves the lane,
// and bit 7 is set exactly when t >= c. Three operators.
//
// Both operands MUST have every lane below 128. Widening the lanes or feeding
// this unbounded values breaks it silently.
func lanesAtMost(prefix, threshold uint64) uint64 {
	return ((threshold | high8) - prefix) & high8
}

// lanesNonZero marks bit 7 of each lane that holds a nonzero value.
func lanesNonZero(v uint64) uint64 {
	return (((v | high8) - low8) | v) & high8
}

// countMarkedLanes totals the marks lanesAtMost and lanesNonZero produce.
// The multiply is the prefix-sum trick again: lane 7 of the product holds the
// sum of all eight lanes.
func countMarkedLanes(marks uint64) uint64 {
	return (marks >> 7) * low8 >> 56
}

// updateSelectOne returns the position of the n-th 1 in the 64-bit word x.
// k is 0-based, so k=0 returns the position of the first 1.
// The result is 64 if x contains n or fewer one bits.
//
// Uses the broadword selection algorithm by Vigna [1]
// [https://vigna.di.unimi.it/ftp/papers/Broadword.pdf]
//
// Vigna's algorithm is obsoleted on most x86 systems but will work
// very well on ARM because ARM gives us logical immediate encoding.
// AArch64's bitmask immediates encode any pattern which repeats with
// period 2,4,8,16,32,64. This is exactly the shape of every SWAR mask.
func genericSelectOne(x uint64, n int) int {

	// TODO(arm64): shifted register operands should make this cheaper than
	// its instruction count suggests. `SUB Xd, Xn, Xm, LSR #1` folds a shift
	// into the ALU op for free, and Vigna's reduction is full of
	// `x - ((x & m) >> 1)` and `(x >> 2) & m` patterns that Go's arm64
	// backend has SUBshiftRL/ANDshiftRL/ORshiftLL rules to match.
	// Verify with `go build -gcflags=-S` that the fused forms are actually
	// being emitted before relying on this.

	// phase 1: lane i := popcount of byte i of word.
	laneOnes := x - ((x & (0xA * low4)) >> 1)
	laneOnes = (laneOnes & (0x3 * low4)) + ((laneOnes >> 2) & (0x3 * low4))
	laneOnes = (laneOnes + (laneOnes >> 4)) & (0xF * low8)

	// phase 2: lane i := ones in bytes 0..i. Eight inclusive prefix sums in one
	// multiply; nothing overflows a lane because the total is at most 64.

	prefixOnes := laneOnes * low8

	// prefixOnes, lane 7, is word's popcount. So we gate here instead of running
	// it separately at the start
	rank := uint64(n)
	if rank >= prefixOnes>>56 {
		return 64
	}

	// A lane is marked when its running total has not yet passed rank. When it
	// sits strictly before the lane holding the answer. So the number of marks
	// is exactly the index of the lane holding the answer.
	lanesBefore := lanesAtMost(prefixOnes, rank*low8)
	laneShift := countMarkedLanes(lanesBefore) * 8

	// phase 3: rank within the lane. Shifting prefixOnes up one lane puts the
	// count before the target lane where the target lane's own count was;
	// the zero shifted in handles laneShift == 0 without a branch
	rankInLane := rank - ((prefixOnes << 8) >> laneShift & 0xFF)

	// phase 4: same trick one level down. Broadcast the target byte, mask the
	// diagonal so lane j carries bit j alone, reduce each lane to 0 or 1,
	// prefix-sum, and count the lanes that haven't yet reached rankInLane.
	bitsPerLane := (x >> laneShift & 0xFF) * low8 & diagonal8
	prefixBits := (lanesNonZero(bitsPerLane) >> 7) * low8

	return int(laneShift + countMarkedLanes(lanesAtMost(prefixBits, rankInLane*low8)))
}
