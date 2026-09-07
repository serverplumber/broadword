//go:build amd64 && !purego

// Copyright (C) Serverplumber. All Rights Reserved.

#include "textflag.h"

// func selectPDEP(x uint64, n int) int
//
// Requires BMI1 (TZCNT) and BMI2 (PDEP). Callers must call hasBMI() first.
// Without BMI1, TZCNT decodes as BSF, which leaves its destination unmodified
// on a zero source instead of returning 64. A failure rather than a fault.
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

// func microcodedPDEP() bool
//
// Callers must call hasBMI() first.
//
// Reports whether the running CPU implements PDEP/PEXT in microcode.
// On those chips cost scales with popcount(mask) instead of the ~3-cycle
// single-uop form everyone else gets, so selectPDEP can lose to the
// plain broadword path on a dense mask.
//
// Only AMD has shipped a microcoded PDEP, in two families: 0x15
// (Excavator, AMD's first BMI2 part) and 0x17 (Zen, Zen+, Zen 2). AMD
// fixed it in Zen 3 (family 0x19).
//
// Family 0x15 is excluded wholesale rather than by model number. The
// earlier cores in it -- Bulldozer, Piledriver, Steamroller -- predate
// BMI2 entirely, so they cannot reach this check: archAvailableSelectOne
// tests hasBMI() first. Every family 0x15 part that gets here is
// an Excavator, which saves decoding the split model field.
//
// Vendor comes from CPUID leaf 0 (EBX:EDX:ECX spell "AuthenticAMD").
// Family comes from CPUID leaf 1: bits 11:8 of EAX are the base family;
// when that field reads 0xF, the true family is 0xF plus the extended
// family in bits 27:20 (0xF+0x06 == 0x15, 0xF+0x08 == 0x17, 0xF+0x0A ==
// 0x19 for Zen3+).
TEXT ·microcodedPDEP(SB), NOSPLIT|NOFRAME, $0-1
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
	CMPL BX, $0x15 // Excavator
	JEQ  yes
	CMPL BX, $0x17 // Zen, Zen+, Zen 2
	JEQ  yes
no:
	MOVB $0, ret+0(FP)
	RET

yes:
	MOVB $1, ret+0(FP)
	RET

// func hasBMI() bool
// we guard after checking max < 7
// CPUID returns data either way and we would read gibberish if we did not guard
// check EBX from CPUID leaf 7, subleaf 0
// bit 3 is BMI1
// bit 8 is BMI2
// Both are needed, mask then test.
// To reach leaf 7, zero ECX then CPUID selects subleaf 0
// leaf 1 ignores ECX.
TEXT ·hasBMI(SB), NOSPLIT|NOFRAME, $0-1
	MOVL $0, AX
	CPUID
	CMPL AX, $7
	JCS no               // max leaf < 7: does not exist

	MOVL $7, AX
	XORL CX, CX          // subleaf 0
	CPUID

	ANDL $0x108, BX      // (1<<8)|(1<<3)
	CMPL BX, $0x108
	JEQ yes

no:
	MOVB $0, ret+0(FP)
	RET

yes:
	MOVB $1, ret+0(FP)
	RET
