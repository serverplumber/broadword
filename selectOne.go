// Copyright (C) Serverplumber. All Rights Reserved.

package broadword

import "sync"

var selectOne func(x uint64, n int) int

var selectOneInitOnce = sync.OnceFunc(func() {
	if archAvailableSelectOne() {
		selectOne = archUpdateSelectOne
	} else {
		selectOne = updateSelectOne
	}
},
)

// SelectOne returns the position of the k-th 1 in the 64-bit word x.
// k is 0-based, so k=0 returns the position of the first 1.
// The result is 64 if x contains n or fewer one bits.
func SelectOne(x uint64, n int) int {
	// TODO(serverplumber) I used the sync.OnceFunc pattern because that's what the stdlib
	// authors used in crc32 where I picked up this pattern.
	// Considering this is a hot path of something we're tweaking to shave
	// nanoseconds, perhaps we should just init statically.
	selectOneInitOnce()
	return selectOne(x, n)
}
