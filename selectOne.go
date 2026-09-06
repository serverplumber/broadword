// Copyright (C) Serverplumber. All Rights Reserved.

package broadword

var selectOne func(x uint64, n int) int

func init() {
	if archSelectOne != nil && archAvailableSelectOne() {
		selectOne = archSelectOne
	} else {
		selectOne = genericSelectOne
	}
}

// SelectOne returns the position of the n-th 1 in the 64-bit word x.
// n is 0-based, so n=0 returns the position of the first 1.
// The result is 64 if x contains n or fewer one bits.
func SelectOne(x uint64, n int) int {
	return selectOne(x, n)
}
