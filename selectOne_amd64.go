// Copyright (C) Serverplumber. All Rights Reserved.
//go:build amd64 && !purego

package broadword

// selectPDEP is defined in selectOne_amd64.s.
//
//go:noescape
func selectPDEP(x uint64, n int) int

// microcodedPDEP is defined in selectOne_amd64.s. The HasBMI2 test below
// must be evaluated before it: the family check leans on BMI2 having
// already ruled out the pre-Excavator parts of AMD family 0x15.
func microcodedPDEP() bool

// hasBMI checks for BMI1 and BMI2 instruction set availability
func hasBMI() bool

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
