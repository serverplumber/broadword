//go:build !amd64

// Copyright (C) Serverplumber. All Rights Reserved.

package broadword

// archAvailableSelectOne reports whether an arch-specific selectOne exists
// on this platform. There isn't one outside of amd64/selectPDEP yet, so
// selectOneInitOnce always falls back to the generic implementation here.
func archAvailableSelectOne() bool {
	return false
}

// archUpdateSelectOne is never called: archAvailableSelectOne always
// returns false on this platform, so selectOneInitOnce never selects it.
func archUpdateSelectOne(x uint64, n int) int {
	panic("selectOne: no arch-specific implementation on this platform")
}
