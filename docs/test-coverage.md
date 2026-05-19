# test-coverage.md — `digital.vasic.veritas`

Round-288 symbol → test / Challenge ledger. Every exported symbol
of `digital.vasic.veritas` MUST appear here with the test(s) and
Challenge(s) that exercise it AND the anti-bluff dimension each
proves. Adding an exported symbol without updating this ledger
is a CONST-048 violation. Per Article XI §11.9, every PASS row
MUST carry positive runtime evidence — the "Evidence" column
documents what to capture during a release-gate sweep.

## Exported symbols — type layer (`pkg/types`)

| Symbol                  | Kind     | Unit test(s)                     | Challenge(s)                                      | Anti-bluff dimension                                                       | Evidence (runtime)                                                              |
|-------------------------|----------|----------------------------------|---------------------------------------------------|----------------------------------------------------------------------------|---------------------------------------------------------------------------------|
| `VerifyRequest`         | struct   | `pkg/types/types_test.go`        | `runner -all`                                     | `Validate()` rejects empty Claim; round-trips through client.              | Runner builds a real VerifyRequest, passes through client, asserts non-nil result. |
| `(*VerifyRequest).Validate` | method | `pkg/types/types_test.go`     | `runner -all`                                     | Empty-claim rejection is honest, not stubbed.                              | Unit test asserts error path; runner asserts happy path.                         |
| `VerifyResult`          | struct   | `pkg/types/types_test.go`        | `runner -all`, `veritas_describe_challenge.sh --mutate` | Confidence ∈ [0,1] invariant honoured; Verdict in canonical set.       | Default: runner asserts every per-call confidence in band. Mutate: planted `Confidence: 2.5` MUST trip the check → exit 99. |
| `(*VerifyResult).Validate` | method | `pkg/types/types_test.go`     | `veritas_describe_challenge.sh --mutate`         | Same as above; mutation invariant.                                         | Mutation Challenge uses this exact check on a planted invalid result.            |
| `Evidence`              | struct   | `pkg/types/types_test.go`        | `runner -all`                                     | Relevance honest (not always 1.0); excerpt truncated to budget.            | Runner registers source, calls GetFactSources, asserts non-empty Evidence list.  |
| `Contradiction`         | struct   | `pkg/types/types_test.go`        | `runner -all`                                     | Severity-banded; Models attribution preserved.                             | Runner asserts shape after CompareModels.                                        |
| `ConsistencyCheck`      | struct   | `pkg/types/types_test.go`        | `runner -all`                                     | ConsistencyScore in [0,1]; DivergentPoints sorted + bounded.               | Runner asserts identical responses → 1.0; disjoint responses → low; ≤10 divergent. |
| `HallucinationResult`   | struct   | `pkg/types/types_test.go`        | `runner -all`                                     | Confidence ∈ [0,1]; Hallucinated flag truthful.                            | Runner asserts triggers fire → flagged; clean response → not flagged.            |
| `(*HallucinationResult).Validate` | method | `pkg/types/types_test.go` | `runner -all`                              | Same as above.                                                              | Runner round-trips through Validate after every DetectHallucination call.         |
| `FactCheck`             | struct   | `pkg/types/types_test.go`        | `runner -all`                                     | Confidence ∈ [0,1]; Verified flag truthful.                                 | Runner inspects HallucinationResult.FactualChecks shape.                          |
| `(*FactCheck).Validate` | method   | `pkg/types/types_test.go`        | `runner -all`                                     | Same as above.                                                              | Unit test exhaustive; runner spot-checks.                                         |
| `ModelComparison`       | struct   | `pkg/types/types_test.go`        | `runner -all`                                     | AgreementScore in [0,1]; Most/Least-Accurate non-empty when models present.| Runner asserts after CompareModels with ≥2 models.                                |

## Exported symbols — client layer (`pkg/client`)

| Symbol                          | Kind   | Unit test(s)                                      | Challenge(s)                                              | Anti-bluff dimension                                                                                            | Evidence (runtime)                                                                 |
|---------------------------------|--------|---------------------------------------------------|-----------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------|
| `ErrBaselineVerifierNotConfigured` | var | `pkg/client/client_test.go`                       | `runner -describe`                                        | Surfaces when no real Verifier injected — replaces round-27 substring-bluff default.                            | Describe-mode runner asserts the sentinel string is non-empty + grep-able.        |
| `Verifier`                      | type   | `pkg/client/client_test.go`                       | `runner -all`                                             | Function-type contract honoured; runner installs `substringTestVerifier` via documented `SetVerifier` injection.| Runner exercises after injection; uninjected path surfaces sentinel.              |
| `Client`                        | struct | `pkg/client/client_test.go`                       | `runner -all`, `veritas_describe_challenge.sh`            | mu-protected; close-idempotent.                                                                                  | Runner calls Close twice, asserts second no-op.                                   |
| `New`                           | func   | `pkg/client/client_test.go`                       | `runner -all`                                             | Returns wired client with baseline verifier seeded; cfg validated.                                              | Runner constructs + asserts non-nil cfg.                                          |
| `NewFromConfig`                 | func   | `pkg/client/client_test.go`                       | `runner -all`                                             | Same as above; explicit cfg path.                                                                                | Runner constructs from config.New + asserts.                                      |
| `(*Client).Close`               | method | `pkg/client/client_test.go`                       | `runner -all`                                             | Idempotent; no resources leaked.                                                                                 | Runner double-close.                                                              |
| `(*Client).Config`              | method | `pkg/client/client_test.go`                       | `runner -all`                                             | Returns the same cfg passed at construction.                                                                     | Runner asserts identity.                                                          |
| `(*Client).SetVerifier`         | method | `pkg/client/client_test.go`                       | `runner -all`                                             | Nil-safe (no-op on nil); mu-protected swap.                                                                      | Runner installs test verifier; sets nil → previous still active.                  |
| `(*Client).AddSource`           | method | `pkg/client/client_test.go`                       | `runner -all`                                             | Empty-id rejected (no-op); mu-protected.                                                                          | Runner adds non-empty id + asserts via GetFactSources.                            |
| `(*Client).VerifyClaim`         | method | `pkg/client/client_test.go`                       | `runner -all`, `veritas_describe_challenge.sh --mutate`   | Real Verifier dispatch; without injection surfaces sentinel; result Validate-checked.                            | Runner asserts verdict + confidence band; mutate plants invalid confidence → exit 99. |
| `(*Client).CheckConsistency`    | method | `pkg/client/client_test.go`                       | `runner -all`                                             | Pairwise Jaccard correct; identical → 1.0; disjoint → low; ≤10 divergent.                                       | Runner asserts identical-response score 1.0; disjoint score <0.5.                 |
| `(*Client).DetectHallucination` | method | `pkg/client/client_test.go`                       | `runner -all`                                             | Trigger list fires; clean response → not flagged; confidence capped 0.95.                                       | Runner asserts both branches; checks Confidence ≤ 0.95.                           |
| `(*Client).CompareModels`       | method | `pkg/client/client_test.go`                       | `runner -all`                                             | Per-model VerifyClaim composition; AgreementScore in [0,1].                                                      | Runner with 2+ models asserts bounded agreement.                                  |
| `(*Client).GetFactSources`      | method | `pkg/client/client_test.go`                       | `runner -all`                                             | Substring-match on registered sources; relevance set.                                                            | Runner asserts non-empty list after AddSource + matching claim.                   |
| `(*Client).BatchVerify`         | method | `pkg/client/client_test.go`                       | `runner -all`                                             | Sequential per-claim VerifyClaim composition; error propagation.                                                 | Runner asserts batch length = input length; surfaces first-error.                 |

## Anti-bluff dimensions covered

| Dimension                                                          | Where proved                                                                                   |
|--------------------------------------------------------------------|------------------------------------------------------------------------------------------------|
| No baseline-Verifier bluff (round-27 sentinel)                     | `pkg/client/client.go` `ErrBaselineVerifierNotConfigured`; `runner -describe` asserts.         |
| Real client construction + lifecycle                               | `pkg/client/client_test.go` + runner build-call-close.                                          |
| Real Verifier injection per documented `SetVerifier` contract      | Runner installs `substringTestVerifier` via `SetVerifier`; CONST-050(A) keeps stub in `_test.go`. |
| Real per-call result invariants                                    | Runner asserts verdict/confidence/score bands on every call.                                    |
| Paired-mutation surfaces invariant violations                      | `challenges/veritas_describe_challenge.sh --mutate` plants `Confidence: 2.5` → exit 99.         |
| 5-locale operator UX evidence (CONST-046)                          | `challenges/runner/main.go` + `challenges/fixtures/locales.yaml`.                              |
| Defensive-use boundary (no payload generators, no obfuscators)     | Challenge greps for `Generate(Payload|Attack|Obfuscat)` in non-test sources; must be empty.    |
| `.gitignore` discipline (CONST-053)                                | `.gitignore` audit + `git ls-files --error-unmatch` checks.                                    |

## Maintenance

When you add an exported symbol:

1. Add a row to the matching table above with a real test + Challenge entry.
2. Update `challenges/runner/main.go` if the symbol participates in the
   runtime invariant assertions.
3. Re-run `bash challenges/veritas_describe_challenge.sh` (default
   AND `--mutate`) and capture the output.
4. Bump the round-N tag in commit message + this header.
