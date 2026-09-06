//go:build !amd64 || purego

// Copyright (C) Serverplumber. All Rights Reserved.

package broadword

// archAvailableSelectOne reports whether an arch-specific selectOne exists
// on this platform. There isn't one outside of amd64/selectPDEP yet, so
// selectOneInitOnce always falls back to the generic implementation here.
func archAvailableSelectOne() bool {
	return false
}

var archSelectOne func(uint64, int) int // nil
