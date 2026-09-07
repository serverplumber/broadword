// Copyright (C) Serverplumber. All Rights Reserved.
//go:build amd64 && !purego

package broadword

import (
	"golang.org/x/sys/cpu"
)

// selectPDEP is defined in selectOne_amd64.s.
//
//go:noescape
func selectPDEP(x uint64, n int) int

// microcodedPDEP is defined in selectOne_amd64.s. The HasBMI2 test below
// must be evaluated before it: the family check leans on BMI2 having
// already ruled out the pre-Excavator parts of AMD family 0x15.
func microcodedPDEP() bool

func hasBMI() bool { return cpu.X86.HasBMI1 && cpu.X86.HasBMI2 }

func archAvailableSelectOne() bool {
	return hasBMI() && !microcodedPDEP()
}

var useArch = archAvailableSelectOne()

func selectOne(x uint64, n int) int {
	if useArch {
		return selectPDEP(x, n)
	}
	return genericSelectOne(x, n)
}
