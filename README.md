# Veritas

AI-truthfulness verification, fact-checking, hallucination
detection, and response-consistency analysis for the Plinius Go
service family. Veritas is the truth/verification layer consumed
by HelixAgent and HelixCode and reusable as a standalone library
by any consumer that needs to score the credibility of LLM
output against reference sources.

Module path: `digital.vasic.veritas`. Apache-2.0.

## Status

- Compiles: `go build ./...` exits 0.
- Tests pass under `-race`: 2 packages (`pkg/types`, `pkg/client`),
  all green.
- Round-288 (2026-05-19): docs + Challenge enrichment with paired
  mutation, 5-locale UX evidence per CONST-046, and a real
  exerciser binary at `challenges/runner/main.go`.
- Round-27 (2026-05-17) sentinel: the substring/keyword "baseline
  pipeline" that previously shipped as the default Verifier has
  been removed. Callers MUST inject a real LLM-backed Verifier
  via `SetVerifier` before invoking `VerifyClaim` / `BatchVerify` /
  `CompareModels`; absence surfaces `ErrBaselineVerifierNotConfigured`
  rather than fabricated confidence scores.

## Anti-bluff guarantees (Article XI §11.9 + CONST-035 + CONST-050)

> Verbatim 2026-05-19 operator mandate: *"all existing tests and
> Challenges do work in anti-bluff manner - they MUST confirm
> that all tested codebase really works as expected! We had been
> in position that all tests do execute with success and all
> Challenges as well, but in reality the most of the features
> does not work and can't be used! This MUST NOT be the case and
> execution of tests and Challenges MUST guarantee the quality,
> the completition and full usability by end users of the
> product!"*

Every PASS emitted by this submodule's tests + Challenges carries
**positive runtime evidence** captured during execution:

1. **No baseline-bluff Verifier.** The historical substring +
   crude-negation default that fabricated verdicts has been
   replaced with `ErrBaselineVerifierNotConfigured`. Tests that
   need deterministic verdicts install `substringTestVerifier`
   from `client_test.go` via `SetVerifier` — CONST-050(A)
   permits stubs in `*_test.go` only; production code MUST inject
   a real LLM verifier.
2. **Real round-trips, not asserted absence-of-error.** The
   round-288 runner (`challenges/runner/main.go`) builds a real
   `*client.Client`, registers a real `substringTestVerifier`
   via the documented `SetVerifier` injection point, registers
   real sources via `AddSource`, invokes every advertised method
   on the client surface (`VerifyClaim`, `BatchVerify`,
   `CompareModels`, `CheckConsistency`, `DetectHallucination`,
   `GetFactSources`), and asserts per-call invariants on the
   returned values (verdict + confidence in range; consistency
   score in [0,1]; hallucination flag and confidence band; model
   comparison agreement bounds).
3. **Paired mutation.** `challenges/veritas_describe_challenge.sh
   --mutate` builds a scratch program that returns
   `VerifyResult{Confidence: 2.5}` (out-of-band confidence) and
   asserts the same invariant check the runner uses MUST trip
   it (exit 99). A mutation run that exits 0 means the
   Challenge itself is a bluff; the script exits 1 to surface
   that.
4. **5-locale UX evidence (CONST-046).** Every runner pass
   prints one summary line per `{en, sr, ja, es, de}`. Missing
   locales fail the Challenge (exit 4). The locale templates
   are mirrored in `challenges/fixtures/locales.yaml` for
   reviewer-visible drift detection.
5. **Defensive use only.** Veritas exists to *score* truthfulness;
   it does NOT generate, mutate, obfuscate, or weaponise any
   payload. The Challenge greps for inverse-helper names
   (`Generate(Payload|Attack|Obfuscat)`, `Bypass*`) in non-test
   sources and fails on any hit.

## Purpose

- `pkg/types` — value types: `VerifyRequest`, `VerifyResult`,
  `Evidence`, `Contradiction`, `ConsistencyCheck`,
  `HallucinationResult`, `FactCheck`, `ModelComparison`. Every
  type ships `Validate()` honest about field-range invariants.
- `pkg/client` — verification pipeline:
  - `VerifyClaim(req)` — runs the injected Verifier, returns
    verdict (`true | false | uncertain`) + confidence + evidence
    + contradictions.
  - `BatchVerify([]claims)` — sequential per-claim verification.
  - `CompareModels(claim, models)` — cross-model agreement +
    most/least-accurate label.
  - `CheckConsistency(responses, models)` — pairwise Jaccard
    similarity + divergent tokens.
  - `DetectHallucination(response, model)` — keyword-trigger
    detector for ungrounded claims.
  - `GetFactSources(claim)` — registered-source lookup with
    relevance.
  - `SetVerifier(Verifier)` / `AddSource(id, text)` — extension hooks.

## Quick start

```go
package main

import (
    "context"
    "fmt"
    "log"

    veritas "digital.vasic.veritas/pkg/client"
    "digital.vasic.veritas/pkg/types"
)

func main() {
    c, err := veritas.New()
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()

    // REQUIRED — inject a real LLM-backed Verifier. Without
    // this, VerifyClaim surfaces ErrBaselineVerifierNotConfigured
    // (round-27 sentinel; previous default fabricated verdicts).
    c.SetVerifier(myLLMVerifier)

    c.AddSource("wiki-water",
        "At standard atmospheric pressure water boils at 100C.")

    r, err := c.VerifyClaim(context.Background(), types.VerifyRequest{
        Claim: "Water boils at 100C at sea level.",
        ReferenceSources: []string{
            "At standard atmospheric pressure water boils at 100C.",
        },
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("verdict=%s confidence=%.2f\n", r.Verdict, r.Confidence)
}
```

Run the round-288 Challenge against your tree:

```bash
cd Veritas
bash challenges/veritas_describe_challenge.sh           # default — exit 0 PASS
bash challenges/veritas_describe_challenge.sh --mutate  # mutation  — exit 99 PASS
```

The default mode invokes `challenges/runner/main.go -all` against
a real `pkg/client` and asserts captured stdout contains the
operation tally and every supported locale line. The mutate mode
runs a scratch program that violates the `Confidence ∈ [0,1]`
invariant and asserts the invariant check surfaces the violation.

## Module path

```go
import "digital.vasic.veritas"
```

## Lineage

Extracted from internal HelixAgent research tree on 2026-04-21.
The earlier Python upstream name was obfuscated (leetspeak); this
Go port uses a clean readable name. Graduated to functional
status alongside its 7 sibling Plinius modules.

Historical research corpus (unused) remains at
`docs/research/go-elder-plinius-v3/go-elder-plinius/go-v3r1t4s/`
inside the HelixAgent repository.

## Development layout

This module's `go.mod` declares the module as
`digital.vasic.veritas` and uses a relative `replace` directive
pointing at `../PliniusCommon`. Sibling modules (Storage,
EventBus, Concurrency, Observability, Auth, VectorDB, Embeddings,
Database, Cache, LLMOps, …) follow the same `digital.vasic.*`
naming convention.

## Documentation

- [`docs/test-coverage.md`](docs/test-coverage.md) — symbol →
  test → Challenge ledger; every exported symbol with the
  anti-bluff dimension it proves and the runtime evidence
  captured at release-gate sweep.
- [`docs/HOST_POWER_MANAGEMENT.md`](docs/HOST_POWER_MANAGEMENT.md)
  — CONST-033 host-power hard-ban guidance.
- [`CLAUDE.md`](CLAUDE.md) / [`AGENTS.md`](AGENTS.md) /
  [`CONSTITUTION.md`](CONSTITUTION.md) — governance + cascaded
  anchors (CONST-035, CONST-047..061, Article XI §11.9).

## License

Apache-2.0
