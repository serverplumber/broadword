//go:build amd64 && !purego

// Copyright (C) Serverplumber. All Rights Reserved.

#include "textflag.h"

// func selectPDEP(x uint64, n int) int
//
// Requires BMI1 (TZCNT) and BMI2 (PDEP). Callers must check
// cpu.X86.HasBMI2 -- on older hardware this SIGILLs.
//
// Depositing a single bit at position n into x scatters that bit to the
// position of the n'th set bit of x; TZCNT then reads it off. PDEP of an
// out-of-range bit yields 0, and TZCNT of 0 is 64, so n >= popcount(x)
// returns 64 without any explicit test.
TEXT ·selectPDEP(SB), NOSPLIT|NOFRAME, $0-24
	MOVQ   x+0(FP), AX
	MOVQ   n+8(FP), CX
	MOVQ   $1, DX
	SHLQ   CX, DX     // garbage if n out of range; fixed up below
	XORQ   BX, BX
	CMPQ   CX, $64    // must come after SHLQ and XORQ -- both clobber flags
	CMOVQCC BX, DX    // n >= 64 unsigned: DX = 0, so PDEP gives 0, TZCNT 64
	PDEPQ  AX, DX, DX
	TZCNTQ DX, AX
	MOVQ   AX, ret+16(FP)
	RET

// func notZen3() bool
//
// Reports whether the running CPU is AMD family 0x17 -- Zen, Zen+, or
// Zen 2. On those chips PDEP/PEXT are implemented in microcode: cost
// scales with popcount(mask) instead of the ~3-cycle single-uop form
// everyone else gets, so selectPDEP can lose to the plain broadword
// path on a dense mask. AMD fixed this starting with Zen 3 (family
// 0x19), hence the name.
//
// Vendor comes from CPUID leaf 0 (EBX:EDX:ECX spell "AuthenticAMD").
// Family comes from CPUID leaf 1: bits 11:8 of EAX are the base family;
// when that field reads 0xF, the true family is 0xF plus the extended
// family in bits 27:20 (0xF+0x08 == 0x17 for Zen/Zen+/Zen2, 0xF+0x0A ==
// 0x19 for Zen3+).
TEXT ·oldZen(SB), NOSPLIT|NOFRAME, $0-1
	MOVL $0, AX
	CPUID
	CMPL BX, $0x68747541 // "Auth"
	JNE  no
	CMPL DX, $0x69746e65 // "enti"
	JNE  no
	CMPL CX, $0x444d4163 // "cAMD"
	JNE  no

	MOVL $1, AX
	CPUID
	MOVL AX, BX
	SHRL $8, BX
	ANDL $0xF, BX // base family
	CMPL BX, $0xF
	JNE  checkFamily
	MOVL AX, CX
	SHRL $20, CX
	ANDL $0xFF, CX // extended family
	ADDL CX, BX    // actual family = base + extended

checkFamily:
	CMPL BX, $0x17
	JNE  no
	MOVB $1, ret+0(FP)
	RET

no:
	MOVB $0, ret+0(FP)
	RET

