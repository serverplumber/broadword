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

func oldZen() bool

func archAvailableSelectOne() bool {
	return cpu.X86.HasBMI1 && cpu.X86.HasBMI2 && !oldZen()
}

var archSelectOne = selectPDEP
