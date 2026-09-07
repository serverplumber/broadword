# broadword

Word-level `select` for Go: the position of the *n*-th set bit in a `uint64`.

```go
broadword.SelectOne(0b1010_0100, 1)  // 5 -- the second set bit
broadword.SelectOne(0, 0)            // 64 -- not found
```

## Why

`math/bits` covers most of what you want from a 64-bit word. `OnesCount64` compiles to `POPCNT`, `TrailingZeros64` to `TZCNT`. Select is the gap: there is no hardware instruction for it on most architectures and no standard library equivalent, so anything built on rank/select over bit vectors — succinct structures, Elias–Fano sequences, quotient filters — has to bring its own.

This package is that one function, written while porting a counting quotient filter, where select sits on the hot path. 

The comments here are denser than a package this size normally warrants. I did this because this
repo doubles as a record of how bit-level work is done in `Go`. Where the inliner gives up, what
the assembler will and will not let us assume, what the CPU feature bits actually mean.

## API

```go
func SelectOne(x uint64, n int) int
```

Returns the position of the *n*-th set bit in `x`, zero-based, so `n = 0` gives the first. Returns `64` when `x` has `n` or fewer set bits, and for any out-of-range `n`, including negatives — no panic, no error, no second return value. Callers testing for absence compare against 64.

## Implementations

Two, selected once at package initialisation.

**Generic** — Vigna's broadword algorithm ([Broadword Implementation of Rank/Select Queries](https://vigna.di.unimi.it/ftp/papers/Broadword.pdf), 2008). A SWAR reduction treating the word as eight parallel byte lanes: popcount per lane, prefix-sum all eight lanes in a single multiply, locate the lane holding the answer, then repeat the same trick one level down to find the bit within that lane. Pure Go, portable, no lookup table.

The absence of a table is deliberate. The usual implementations carry a 2 KB `selectInByte` array for the final step; this does it with a broadcast, a mask against the 8×8 diagonal, and a second prefix sum. No table means no cache line to miss and no third party's data vendored into the package.

**amd64** — Hand-written assembly using BMI2. Depositing a single bit at position *n* into `x` with `PDEP` scatters it to exactly the position of the *n*-th set bit; `TZCNT` reads that position off. The whole block reduces to a `PDEP` and a `TZCNT`. Out-of-range `n` deposits nothing, and `TZCNT` of zero is 64, so the boundary case falls out of the instruction semantics rather than needing a branch.

Dispatch resolves once, at package initialisation. A single `bool` records whether the assembly is worth using, and `selectOne` branches on it between two ordinary direct calls — no function variable, no indirect call, and the branch is perfectly predicted after the first iteration. One binary still works everywhere; the CPU is probed at startup, not at build time.

That branch lives in a build-tagged file, which is forced rather than chosen. `selectPDEP` only exists under `amd64 && !purego`, so a call site naming it directly cannot appear in portable code. `selectOne.go` holds the documented exported function; each architecture supplies its own unexported `selectOne` underneath it.

## The AMD wrinkle

Checking `cpu.X86.HasBMI2` is not sufficient to decide whether the assembly path is worth taking.

Some AMD parts implement `PDEP` and `PEXT` in microcode. They are present, they report as available, and they are *slow* — cost scales with the population count of the mask instead of the roughly three-cycle single-µop form Intel has shipped since Haswell and AMD ships from Zen 3 onward. On a dense word, the "optimised" path loses to the portable one.

Two families are affected: `0x15` (Excavator, AMD's first part with BMI2 at all) and `0x17` (Zen, Zen+, Zen 2). So the feature check is paired with a CPUID vendor and family test that excludes both, and those machines get the broadword path.

Family `0x15` is excluded wholesale rather than by model number, which is worth a word since the family also contains Bulldozer, Piledriver and Steamroller. Those cores predate BMI2, so `cpu.X86.HasBMI2` has already rejected them by the time the family test runs — every `0x15` part that reaches it is an Excavator. Ordering the checks that way buys the accuracy of a model-number test without decoding CPUID's split model field.

This is the kind of thing that doesn't show up in a correctness test and doesn't show up in a benchmark run on one machine.

## Correctness

`naiveSelectOne` is a brute-force loop over the 64 bit positions, written to be obviously correct and to share none of the broadword tricks. It is the oracle for the fuzz targets.

- `selectOneCases` is a table of hand-chosen edge cases — empty word, saturated word, negative *n*, *n* past the popcount, answers falling inside one byte, crossing a lane boundary, and skipping a run of zeroes. Its expected values are written out by hand rather than taken from the oracle, deliberately: an oracle shared by every assertion in the suite cannot catch a misconception baked into the oracle itself. The table lives in the untagged file so there is exactly one copy, and both `TestSelectOne` and `TestSelectPDEP` read from it.
- `FuzzGenericSelectOne` differentially tests the portable path against the oracle; `FuzzSelectPDEP` does the same for the assembly.
- `TestSelectOne` exercises `SelectOne` on every platform, unconditionally. That matters more than it looks: on a non-amd64 build it is the only coverage the exported API gets, and putting it behind an architecture check is exactly how it went untested for several commits.

Two different predicates guard the amd64 tests, and conflating them is easy:

| question | predicate | guards |
|---|---|---|
| does the symbol compile? | `//go:build amd64 && !purego` | the file |
| can this CPU execute it? | `hasBMI()` | correctness tests |
| should we dispatch to it? | `archAvailableSelectOne()` | benchmarks |

Correctness tests use `hasBMI()`, not the dispatch predicate. On Zen 2 and Excavator `PDEP` executes perfectly — it is merely slow — so guarding the tests with the dispatch check would leave the assembly entirely untested on precisely the machines where the dispatch decision is doing something non-obvious.

## Benchmarks

`BenchmarkSelectOne` covers the portable implementation and the exported entry point; `BenchmarkSelectPDEP`, in a build-tagged file, covers the assembly through a direct call. They are separate for the same reason the dispatch branch is: portable code cannot name `selectPDEP`, so reaching it from `BenchmarkSelectOne` would mean going through a function value and putting an indirection on one side of the comparison and not the other.

Both report a `throughput` and a `latency` variant. The throughput loops issue independent calls that the CPU overlaps, which is what a bulk scan over a bit vector sees. The latency loops feed each result into the next index so the calls serialise, which is closer to a walk down a rank/select structure. Each has a `baseline` sub-benchmark measuring the empty loop; subtract it before quoting a ratio, because at these magnitudes it is a large fraction of the total.

Net of baseline, on a 13th-gen Intel laptop part. Everything is relative to `selectPDEP` at `1.00×`, so lower is better:

| | generic | `selectPDEP` | `SelectOne` |
|---|---|---|---|
| throughput | 3.53× | 1.00× | 1.31× |
| latency | 3.40× | 1.00× | 1.13× |

The assembly is worth roughly 3.5× on throughput and 3.4× on latency. Reaching it through `SelectOne` rather than calling it directly costs 31% and 13% — that residue is a call frame rather than a branch, and Known gaps explains why it is the floor in pure Go.

Absolute nanoseconds are deliberately not quoted. The machine these came from is a power-capped laptop whose clock wanders between 400 MHz and 2 GHz under sustained load, and it has both P- and E-cores, so an unpinned run measures core placement as much as it measures code. The ratios above are stable to within a few percent; the raw times move by 3× depending on nothing but thermal state. If you want numbers you can act on, pin to one core, interleave the variants rather than running all of A then all of B, and put the result through `benchstat`:

```
go test -c -o bench.test .
for i in $(seq 1 20); do
  for b in 'SelectOne/throughput/generic$' 'SelectPDEP/throughput$'; do
    GOMAXPROCS=1 taskset -c 4 ./bench.test -test.run='^$' \
      -test.bench="$b" -test.benchtime=4000000x
  done
done
```

Inputs are 1024 precomputed pairs at 85% word density, generated from an explicitly seeded PRNG so that two runs compare the same work. The count is a power of two because the loops index with `i & 1023`: `len()` of a slice is not a compile-time constant, so `i % len(pairs)` compiles to a hardware `DIVQ` that costs more than either implementation and sits inside the timed region.

## Status

Early. The API is one function and unlikely to change, but nothing here is tagged stable yet.

Known gaps:

- **No arm64 specialisation.** The generic path should do well there — AArch64's logical immediate encoding covers any pattern repeating with period 2/4/8/16/32/64, which is the shape of every SWAR mask, and shifted register operands can fold the reduction's `x - ((x & m) >> 1)` patterns into single ALU ops. Whether Go's backend actually emits the fused forms is unverified; the code carries a note to check with `-gcflags=-S` before anyone relies on it.
- **`SelectOne` is the only export.** The parallel byte comparisons the reduction is built from (`≤` and `≠0` across lanes) are unexported. They're generally useful and may come out later.
- **Dispatch costs a call frame, and in pure Go that is the floor.** `SelectOne` runs 31% slower on throughput and 13% on latency than calling `selectPDEP` directly. That cost is not the branch, and it is no longer an indirect call: an earlier version dispatched through a function variable, and replacing it with a `bool` and a branch moved the measured overhead from 33% to 31%, which is within noise. The indirect call had a single target for the life of the process, and the branch predictor absorbed it completely.

  What remains is an extra frame. Go's inliner charges 57 per call site against a budget of 80, so any dispatcher containing two calls is over budget and cannot be inlined — a caller gets `SelectOne` inlined, then a call to `selectOne`, then a call to `selectPDEP`. Every way of arranging the branch has two call sites, so there is nothing to rearrange.

  `GOAMD64=v3` would remove it. The `amd64.v3` build constraint guarantees BMI1 and BMI2, so `selectOne` could be an unconditional `return selectPDEP(x, n)` — one call site, under budget, inlined away entirely. Measured on that variant, `SelectOne` costs exactly what a direct call costs: 1.00× instead of 1.31×.

  It is not implemented, because `GOAMD64=v3` can tell you `PDEP` *exists* and can never tell you it is *fast*. Zen 2 satisfies v3 in full and carries the microcoded `PDEP` that "The AMD wrinkle" exists to route around, so a v3 build that dropped the runtime probe would silently regress those machines — and putting the probe back restores the second call site and the entire cost. A silent regression on part of a fleet is worse than a loud one. Worth revisiting if a profile ever shows the frame mattering.

Requires `golang.org/x/sys/cpu` for the BMI1 and BMI2 feature bits.

## References

Sebastiano Vigna, *Broadword Implementation of Rank/Select Queries*, WEA 2008. The algorithms here are implemented from the paper; the correction to the 64-bit constant in Algorithm 1 and to line 2 of Algorithm 2 was found independently and matches the errata noted in Jesse Tov's Rust [`broadword`](https://docs.rs/broadword) crate.

## Licence

BSD 3-Clause. See `LICENSE`.
