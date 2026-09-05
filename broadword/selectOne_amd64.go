// Copyright (C) Serverplumber. All Rights Reserved.

package broadword

import (
	"golang.org/x/sys/cpu"
)

// selectPDEP is defined in selectOne_amd64.s.
//
//go:noescape
func selectPDEP(x uint64, n int) int

func oldZen() bool

func archAvailableSelectOne() bool {
	return cpu.X86.HasBMI2 && !oldZen()
}

func archUpdateSelectOne(x uint64, n int) int {
	if !cpu.X86.HasBMI2 {
		panic("bmi2 not supported")
	}
	return selectPDEP(x, n)
}
