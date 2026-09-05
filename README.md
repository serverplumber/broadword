# broadword

Word-level `select` for Go: the position of the *n*-th set bit in a `uint64`.

```go
broadword.SelectOne(0b1010_0100, 1)  // 5 -- the second set bit
broadword.SelectOne(0, 0)            // 64 -- not found
```

## Why

`math/bits` covers most of what you want from a 64-bit word. `OnesCount64` compiles to `POPCNT`, `TrailingZeros64` to `TZCNT`. Select is the gap: there is no hardware instruction for it on most architectures and no standard library equivalent, so anything built on rank/select over bit vectors — succinct structures, Elias–Fano sequences, quotient filters — has to bring its own.

This package is that one function, extracted from work on a counting quotient filter where it sits on the hot path.

## API

```go
func SelectOne(x uint64, n int) int
```

Returns the position of the *n*-th set bit in `x`, zero-based, so `n = 0` gives the first. Returns `64` when `x` has `n` or fewer set bits, and for any out-of-range `n`, including negatives — no panic, no error, no second return value. Callers testing for absence compare against 64.

## Implementations

Two, selected once at first call.

**Generic** — Vigna's broadword algorithm ([Broadword Implementation of Rank/Select Queries](https://vigna.di.unimi.it/ftp/papers/Broadword.pdf), 2008). A SWAR reduction treating the word as eight parallel byte lanes: popcount per lane, prefix-sum all eight lanes in a single multiply, locate the lane holding the answer, then repeat the same trick one level down to find the bit within that lane. Pure Go, portable, no lookup table.

The absence of a table is deliberate. The usual implementations carry a 2 KB `selectInByte` array for the final step; this does it with a broadcast, a mask against the 8×8 diagonal, and a second prefix sum. No table means no cache line to miss and no third party's data vendored into the package.

**amd64** — Hand-written assembly using BMI2. Depositing a single bit at position *n* into `x` with `PDEP` scatters it to exactly the position of the *n*-th set bit; `TZCNT` reads that position off. Two instructions where the broadword reduction needs twenty-odd. Out-of-range `n` deposits nothing, and `TZCNT` of zero is 64, so the boundary case falls out of the instruction semantics rather than needing a branch.

Dispatch follows the standard library's `crc32` pattern: a package-level function variable resolved once through `sync.OnceFunc`, no build tags, one binary that works everywhere.

## The AMD wrinkle

Checking `cpu.X86.HasBMI2` is not sufficient to decide whether the assembly path is worth taking.

AMD's Zen, Zen+ and Zen 2 (family `0x17`) implement `PDEP` and `PEXT` in microcode. They are present, they report as available, and they are *slow* — cost scales with the population count of the mask instead of the roughly three-cycle single-µop form Intel has shipped since Haswell and AMD ships from Zen 3 onward. On a dense word, the "optimised" path loses to the portable one.

So the feature check is paired with a CPUID vendor and family test that excludes family `0x17` specifically, and those machines get the broadword path. This is the kind of thing that doesn't show up in a correctness test and doesn't show up in a benchmark run on one machine.

## Correctness

`naiveSelectOne` is a brute-force loop over the 64 bit positions, written to be obviously correct and to share none of the broadword tricks. It's the oracle:

- A table of hand-chosen edge cases — empty word, saturated word, negative *n*, *n* past the popcount, answers falling inside one byte, crossing a lane boundary, and skipping a run of zeroes — each checked against the oracle rather than against a hand-computed constant.
- `FuzzSelectOne` differentially tests the exported function against the oracle across arbitrary `(x, n)`.
- `BenchmarkSelectOne` measures both paths over 1024 precomputed pairs with words at 85% density, cycled so the branch predictor can't memorise the early-return outcome. The arch-specific sub-benchmark skips itself where the assembly path isn't selected.

## Status

Early. The API is one function and unlikely to change, but nothing here is tagged stable yet.

Known gaps:

- **No arm64 specialisation.** The generic path should do well there — AArch64's logical immediate encoding covers any pattern repeating with period 2/4/8/16/32/64, which is the shape of every SWAR mask, and shifted register operands can fold the reduction's `x - ((x & m) >> 1)` patterns into single ALU ops. Whether Go's backend actually emits the fused forms is unverified; the code carries a note to check with `-gcflags=-S` before anyone relies on it.
- **`SelectOne` is the only export.** The parallel byte comparisons the reduction is built from (`≤` and `≠0` across lanes) are unexported. They're generally useful and may come out later.
- **Dispatch overhead is unmeasured against the alternative.** Go's inliner doesn't cross into assembly, so the fast path is always a real call plus an indirect one through the function variable. For an operation this small that overhead is not obviously free, and static initialisation may be the better choice.

Requires `golang.org/x/sys/cpu` for the BMI2 feature bit.

## References

Sebastiano Vigna, *Broadword Implementation of Rank/Select Queries*, WEA 2008. The algorithms here are implemented from the paper; the correction to the 64-bit constant in Algorithm 1 and to line 2 of Algorithm 2 was found independently and matches the errata noted in Jesse Tov's Rust [`broadword`](https://docs.rs/broadword) crate.

## Licence

BSD 3-Clause. See `LICENSE`.
