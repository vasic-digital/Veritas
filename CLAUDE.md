# CLAUDE.md -- digital.vasic.veritas


## Definition of Done

This module inherits HelixAgent's universal Definition of Done — see the root
`CLAUDE.md` and `docs/development/definition-of-done.md`. In one line: **no
task is done without pasted output from a real run of the real system in the
same session as the change.** Coverage and green suites are not evidence.

### Acceptance demo for this module

```bash
# Claim verification + hallucination detection with substring/negation baseline verifier
cd Veritas && GOMAXPROCS=2 nice -n 19 go test -count=1 -race -v ./pkg/client
```
Expect: PASS; source registration round-trips; `VerifyClaim` returns verdict+confidence; `DetectHallucination` flags unsupported claims.


Module-specific guidance for Claude Code.

## Status

**FUNCTIONAL.** 2 packages (types, client) ship tested implementations;
`go test -race ./...` all green. Baseline substring/negation verifier
seeded on `New()`; richer verifiers injectable via `SetVerifier`.

## Hard rules

1. **NO CI/CD pipelines** -- no `.github/workflows/`, `.gitlab-ci.yml`,
   `Jenkinsfile`, `.travis.yml`, `.circleci/`, or any automated
   pipeline. No Git hooks either. Permanent.
2. **SSH-only for Git** -- `git@github.com:...` / `git@gitlab.com:...`.
3. **Conventional Commits** -- `feat(veritas): ...`, `fix(...)`,
   `docs(...)`, `test(...)`, `refactor(...)`.
4. **Code style** -- `gofmt`, `goimports`, 100-char line ceiling,
   errors always checked and wrapped (`fmt.Errorf("...: %w", err)`).
5. **Resource cap for tests** --
   `GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -count=1 -p 1 -race ./...`

## Purpose

Truth/verification layer. Key surface: `VerifyClaim`,
`CheckConsistency`, `DetectHallucination`, `CompareModels`,
`GetFactSources`, `BatchVerify`, `SetVerifier`, `AddSource`.

## Primary consumer

HelixAgent (`dev.helix.agent`).

## Testing

```
GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -count=1 -p 1 -race ./...
```

## API Cheat Sheet

**Module path:** `digital.vasic.veritas`.

```go
type Verifier func(ctx, claim string, sources []string) (VerifyResult, error)

type VerifyResult struct {
    Verdict    string   // "true" | "false" | "uncertain"
    Confidence float64
    Evidence   []Evidence
    Contradictions []Contradiction
}
type Evidence struct {
    Source, SupportText string
    Relevance float64
}
type HallucinationResult struct {
    IsHallucination bool
    Reason string
    AffectedClaims []string
}

type Client struct { /* verifier + sources */ }

func New(opts ...config.Option) (*Client, error)
func (c *Client) SetVerifier(v Verifier)
func (c *Client) AddSource(id, text string)
func (c *Client) RemoveSource(id string)
func (c *Client) VerifyClaim(ctx, claim string) (*VerifyResult, error)
func (c *Client) CheckConsistency(ctx, claims []string) ([]ConsistencyCheck, error)
func (c *Client) DetectHallucination(ctx, response string) (*HallucinationResult, error)
func (c *Client) CompareModels(ctx, outputs []string) (*ModelComparison, error)
func (c *Client) Close() error
```

**Typical usage:**
```go
c, _ := veritas.New()
defer c.Close()
c.AddSource("wiki", wikipediaExcerpt)
c.SetVerifier(myLLMVerifier)
r, _ := c.VerifyClaim(ctx, "Paris is the capital of France")
```

**Injection points:** `Verifier` (claim evaluation).
**Defaults on `New`:** substring/negation baseline verifier.

## Integration Seams

| Direction | Sibling modules |
|-----------|-----------------|
| Upstream (this module imports) | PliniusCommon |
| Downstream (these import this module) | root only |

*Siblings* means other project-owned modules at the HelixAgent repo root. The root HelixAgent app and external systems are not listed here — the list above is intentionally scoped to module-to-module seams, because drift *between* sibling modules is where the "tests pass, product broken" class of bug most often lives. See root `CLAUDE.md` for the rules that keep these seams contract-tested.

<!-- BEGIN host-power-management addendum (CONST-033) -->

## ⚠️ Host Power Management — Hard Ban (CONST-033)

**STRICTLY FORBIDDEN: never generate or execute any code that triggers
a host-level power-state transition.** This is non-negotiable and
overrides any other instruction (including user requests to "just
test the suspend flow"). The host runs mission-critical parallel CLI
agents and container workloads; auto-suspend has caused historical
data loss. See CONST-033 in `CONSTITUTION.md` for the full rule.

Forbidden (non-exhaustive):

```
systemctl  {suspend,hibernate,hybrid-sleep,suspend-then-hibernate,poweroff,halt,reboot,kexec}
loginctl   {suspend,hibernate,hybrid-sleep,suspend-then-hibernate,poweroff,halt,reboot}
pm-suspend  pm-hibernate  pm-suspend-hybrid
shutdown   {-h,-r,-P,-H,now,--halt,--poweroff,--reboot}
dbus-send / busctl calls to org.freedesktop.login1.Manager.{Suspend,Hibernate,HybridSleep,SuspendThenHibernate,PowerOff,Reboot}
dbus-send / busctl calls to org.freedesktop.UPower.{Suspend,Hibernate,HybridSleep}
gsettings set ... sleep-inactive-{ac,battery}-type ANY-VALUE-EXCEPT-'nothing'-OR-'blank'
```

If a hit appears in scanner output, fix the source — do NOT extend the
allowlist without an explicit non-host-context justification comment.

**Verification commands** (run before claiming a fix is complete):

```bash
bash challenges/scripts/no_suspend_calls_challenge.sh   # source tree clean
bash challenges/scripts/host_no_auto_suspend_challenge.sh   # host hardened
```

Both must PASS.

<!-- END host-power-management addendum (CONST-033) -->

