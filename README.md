# Veritas

AI truthfulness verification: simple fact-checking, hallucination
detection, and response-consistency analysis across multiple AI models.
Part of the Plinius Go service family used by HelixAgent.

## Status

- Compiles: `go build ./...` exits 0.
- Tests pass under `-race`: 2 packages (types, client), all green.
- Baseline substring/negation verifier ships by default so the client
  is immediately usable; richer verifiers can be injected via
  `SetVerifier`.
- Integration-ready: consumable Go library for the HelixAgent ensemble.

## Purpose

- `pkg/types` — value types: `VerifyRequest`, `VerifyResult`,
  `Evidence`, `Contradiction`, `ConsistencyCheck`,
  `HallucinationResult`, `FactCheck`, `ModelComparison`.
- `pkg/client` — verification pipeline:
  - `VerifyClaim(req)` — supported / contradicted / unknown verdict
  - `CheckConsistency(responses, models)` — pairwise Jaccard + divergent
    tokens
  - `DetectHallucination(response, model)` — keyword-trigger detector
  - `CompareModels(claim, models)` — cross-model agreement
  - `GetFactSources(claim)` — registered-source lookup
  - `BatchVerify([]claims)`
  - `SetVerifier(Verifier)` / `AddSource(id, text)` — extension hooks

## Usage

```go
import (
    "context"
    "log"

    veritas "digital.vasic.veritas/pkg/client"
    "digital.vasic.veritas/pkg/types"
)

c, err := veritas.New()
if err != nil { log.Fatal(err) }
defer c.Close()

r, err := c.VerifyClaim(context.Background(), types.VerifyRequest{
    Claim: "Water boils at 100C at sea level.",
    ReferenceSources: []string{
        "At standard atmospheric pressure water boils at 100C.",
    },
})
if err != nil { log.Fatal(err) }
log.Printf("verdict=%s confidence=%.2f", r.Verdict, r.Confidence)
```

## Module path

```go
import "digital.vasic.veritas"
```

## Lineage

Extracted from internal HelixAgent research tree on 2026-04-21. The
earlier Python upstream name was obfuscated (leetspeak); this Go port
uses a clean readable name. Graduated to functional status alongside
its 7 sibling Plinius modules.

Historical research corpus (unused) remains at
`docs/research/go-elder-plinius-v3/go-elder-plinius/go-v3r1t4s/` inside
the HelixAgent repository.

## Development layout

This module's `go.mod` declares the module as `digital.vasic.veritas`
and uses a relative `replace` directive pointing at `../PliniusCommon`.

## License

Apache-2.0
