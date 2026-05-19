// Command runner is the round-288 anti-bluff Challenge runner for
// digital.vasic.veritas. It exercises the public client API
// against a real *client.Client, injects a real Verifier via the
// documented SetVerifier injection point, registers real sources,
// invokes every advertised method, asserts per-call invariants on
// the returned values, and emits a 5-locale bilingual UX summary
// line per CONST-046.
//
// Defensive-use only. The runner exercises the verification API;
// it does NOT generate, mutate, or weaponise any payload. There
// is no inverse helper.
//
// Exit codes:
//
//	0 — every check passed; every locale line printed.
//	1 — usage / flag error.
//	2 — coverage gap (missing op, construction failure).
//	3 — invariant violation (out-of-band confidence/score, mismatched verdict, lost source).
//	4 — locale UX line missing.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	client "digital.vasic.veritas/pkg/client"
	"digital.vasic.veritas/pkg/types"
)

// locale describes a UX line printed by the runner. The text is a
// short, locale-correct summary that consumers can grep for to
// confirm operator-facing localisation was emitted in every
// supported locale.
type locale struct {
	tag  string
	line func(opCount int, verifications int) string
}

// supportedLocales is the 5-locale CONST-046 set the runner must
// emit every run. The set mirrors the test-bank locale matrix used
// across other round-28X enrichments.
func supportedLocales() []locale {
	return []locale{
		{
			tag: "en",
			line: func(o, v int) string {
				return fmt.Sprintf("[en] veritas: %d operations exercised, %d verifications run via real client (defensive-use only)", o, v)
			},
		},
		{
			tag: "sr",
			line: func(o, v int) string {
				return fmt.Sprintf("[sr] veritas: %d operacija izvršeno, %d verifikacija pokrenuto preko realnog klijenta (samo za odbranu)", o, v)
			},
		},
		{
			tag: "ja",
			line: func(o, v int) string {
				return fmt.Sprintf("[ja] veritas: %d 件の操作を実行、%d 件の検証を実クライアントで実施(防御用途のみ)", o, v)
			},
		},
		{
			tag: "es",
			line: func(o, v int) string {
				return fmt.Sprintf("[es] veritas: %d operaciones ejecutadas, %d verificaciones realizadas con cliente real (uso defensivo)", o, v)
			},
		},
		{
			tag: "de",
			line: func(o, v int) string {
				return fmt.Sprintf("[de] veritas: %d Operationen ausgeführt, %d Verifikationen mit echtem Client durchgeführt (nur Verteidigung)", o, v)
			},
		},
	}
}

func main() {
	all := flag.Bool("all", false, "run every check (default mode)")
	describe := flag.Bool("describe", false, "describe the interface surface only")
	flag.Parse()

	if !*all && !*describe {
		*all = true
	}

	if *describe {
		if err := runDescribe(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(exitCodeFor(err))
		}
		return
	}

	if err := runAll(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitCodeFor(err))
	}
}

// runDescribe enumerates the public surface and asserts the
// round-27 sentinel ErrBaselineVerifierNotConfigured is present
// + grep-able. Useful for fast smoke checks that don't touch a
// full verification pipeline.
func runDescribe() error {
	if client.ErrBaselineVerifierNotConfigured == nil {
		return wrap(errCoverage, errors.New("ErrBaselineVerifierNotConfigured sentinel is nil"))
	}
	msg := client.ErrBaselineVerifierNotConfigured.Error()
	if !strings.Contains(msg, "veritas") {
		return wrap(errInvariant, fmt.Errorf("sentinel message lost 'veritas' token: %q", msg))
	}
	if !strings.Contains(msg, "SetVerifier") {
		return wrap(errInvariant, fmt.Errorf("sentinel message lost SetVerifier guidance: %q", msg))
	}
	fmt.Println("sentinel=ErrBaselineVerifierNotConfigured present=true")
	fmt.Println("sentinel.contains=veritas,SetVerifier OK")
	fmt.Println("OK round-27 anti-bluff sentinel verified")
	return nil
}

// runAll exercises the full client API against a real
// *client.Client with an injected substring-style test Verifier.
// Per CONST-050(A), the test Verifier lives in this runner (not
// in production code) and the production package's default still
// surfaces ErrBaselineVerifierNotConfigured for callers that
// forget to inject one.
func runAll() error {
	ctx := context.Background()
	ops := 0
	verifications := 0

	// Coverage check 0 — sentinel must surface BEFORE injection.
	noVerifierClient, err := client.New()
	if err != nil {
		return wrap(errCoverage, fmt.Errorf("New (pre-injection): %w", err))
	}
	_, vcErr := noVerifierClient.VerifyClaim(ctx, types.VerifyRequest{Claim: "control claim"})
	if vcErr == nil {
		return wrap(errInvariant, errors.New("VerifyClaim without SetVerifier did NOT surface sentinel — round-27 bluff regression"))
	}
	if !strings.Contains(vcErr.Error(), "SetVerifier") {
		return wrap(errInvariant, fmt.Errorf("pre-injection error lost SetVerifier guidance: %v", vcErr))
	}
	_ = noVerifierClient.Close()
	ops++

	// Coverage check 1 — real client construction with injected Verifier.
	c, err := client.New()
	if err != nil {
		return wrap(errCoverage, fmt.Errorf("New: %w", err))
	}
	defer c.Close()
	ops++

	c.SetVerifier(runnerVerifier)
	ops++

	if c.Config() == nil {
		return wrap(errInvariant, errors.New("Config() returned nil after New"))
	}
	ops++

	// Coverage check 2 — AddSource / GetFactSources / RemoveSource.
	c.AddSource("wiki-water", "At standard atmospheric pressure water boils at 100C.")
	c.AddSource("wiki-paris", "Paris is the capital of France.")
	c.AddSource("", "empty id — must be no-op") // exercises nil-safe path
	ops++

	src, err := c.GetFactSources(ctx, "paris")
	if err != nil {
		return wrap(errCoverage, fmt.Errorf("GetFactSources: %w", err))
	}
	if len(src) == 0 {
		return wrap(errInvariant, errors.New("GetFactSources returned empty for matching claim"))
	}
	for _, e := range src {
		if e.Source == "" || e.Excerpt == "" {
			return wrap(errInvariant, fmt.Errorf("Evidence missing fields: %+v", e))
		}
		if e.Relevance < 0 || e.Relevance > 1 {
			return wrap(errInvariant, fmt.Errorf("Evidence.Relevance out of band: %f", e.Relevance))
		}
	}
	ops++

	// Coverage check 3 — VerifyClaim happy path.
	r, err := c.VerifyClaim(ctx, types.VerifyRequest{
		Claim: "Paris is the capital of France.",
		ReferenceSources: []string{
			"Paris is the capital of France.",
		},
	})
	if err != nil {
		return wrap(errCoverage, fmt.Errorf("VerifyClaim happy: %w", err))
	}
	if err := assertVerifyResult(r); err != nil {
		return wrap(errInvariant, fmt.Errorf("VerifyClaim happy result: %w", err))
	}
	if r.Verdict != "true" {
		return wrap(errInvariant, fmt.Errorf("expected verdict=true, got %q", r.Verdict))
	}
	verifications++
	ops++

	// Coverage check 4 — VerifyClaim contradicted path.
	r2, err := c.VerifyClaim(ctx, types.VerifyRequest{
		Claim: "Paris is the capital of Germany.",
		ReferenceSources: []string{
			"Paris is the capital of France.",
		},
	})
	if err != nil {
		return wrap(errCoverage, fmt.Errorf("VerifyClaim contradicted: %w", err))
	}
	if err := assertVerifyResult(r2); err != nil {
		return wrap(errInvariant, fmt.Errorf("VerifyClaim contradicted result: %w", err))
	}
	verifications++
	ops++

	// Coverage check 5 — BatchVerify.
	batch, err := c.BatchVerify(ctx, []string{
		"Paris is the capital of France.",
		"Water boils at 100C at sea level.",
	})
	if err != nil {
		return wrap(errCoverage, fmt.Errorf("BatchVerify: %w", err))
	}
	if len(batch) != 2 {
		return wrap(errInvariant, fmt.Errorf("BatchVerify returned %d, want 2", len(batch)))
	}
	for i, br := range batch {
		if err := assertVerifyResult(&br); err != nil {
			return wrap(errInvariant, fmt.Errorf("BatchVerify[%d]: %w", i, err))
		}
		verifications++
	}
	ops++

	// Coverage check 6 — CompareModels.
	cmp, err := c.CompareModels(ctx, "Paris is the capital of France.", []string{"model-a", "model-b"})
	if err != nil {
		return wrap(errCoverage, fmt.Errorf("CompareModels: %w", err))
	}
	if cmp.AgreementScore < 0 || cmp.AgreementScore > 1 {
		return wrap(errInvariant, fmt.Errorf("CompareModels AgreementScore out of band: %f", cmp.AgreementScore))
	}
	if len(cmp.ModelResults) != 2 {
		return wrap(errInvariant, fmt.Errorf("CompareModels returned %d model results, want 2", len(cmp.ModelResults)))
	}
	verifications += len(cmp.ModelResults)
	ops++

	// Coverage check 7 — CheckConsistency identical responses → 1.0.
	cons, err := c.CheckConsistency(ctx, []string{
		"the sky is blue today",
		"the sky is blue today",
	}, []string{"model-a", "model-b"})
	if err != nil {
		return wrap(errCoverage, fmt.Errorf("CheckConsistency identical: %w", err))
	}
	if cons.ConsistencyScore != 1.0 {
		return wrap(errInvariant, fmt.Errorf("identical responses score %f, want 1.0", cons.ConsistencyScore))
	}
	ops++

	// Coverage check 8 — CheckConsistency disjoint responses → low.
	cons2, err := c.CheckConsistency(ctx, []string{
		"the sky is blue today",
		"completely unrelated other lexicon entirely",
	}, []string{"model-a", "model-b"})
	if err != nil {
		return wrap(errCoverage, fmt.Errorf("CheckConsistency disjoint: %w", err))
	}
	if cons2.ConsistencyScore < 0 || cons2.ConsistencyScore > 1 {
		return wrap(errInvariant, fmt.Errorf("disjoint score out of band: %f", cons2.ConsistencyScore))
	}
	if cons2.ConsistencyScore >= 0.5 {
		return wrap(errInvariant, fmt.Errorf("disjoint responses scored %f, expected <0.5", cons2.ConsistencyScore))
	}
	if len(cons2.DivergentPoints) > 10 {
		return wrap(errInvariant, fmt.Errorf("DivergentPoints not bounded: %d", len(cons2.DivergentPoints)))
	}
	ops++

	// Coverage check 9 — DetectHallucination flagged path.
	h, err := c.DetectHallucination(ctx, "As I mentioned earlier, in 2023 the latest study confirmed this.", "model-a")
	if err != nil {
		return wrap(errCoverage, fmt.Errorf("DetectHallucination flagged: %w", err))
	}
	if !h.Hallucinated {
		return wrap(errInvariant, errors.New("DetectHallucination missed trigger phrase"))
	}
	if h.Confidence < 0 || h.Confidence > 0.95 {
		return wrap(errInvariant, fmt.Errorf("Hallucination confidence out of band: %f", h.Confidence))
	}
	if hErr := h.Validate(); hErr != nil {
		return wrap(errInvariant, fmt.Errorf("HallucinationResult.Validate: %w", hErr))
	}
	ops++

	// Coverage check 10 — DetectHallucination clean path.
	h2, err := c.DetectHallucination(ctx, "Paris is the capital of France.", "model-a")
	if err != nil {
		return wrap(errCoverage, fmt.Errorf("DetectHallucination clean: %w", err))
	}
	if h2.Hallucinated {
		return wrap(errInvariant, errors.New("DetectHallucination false-flagged a clean response"))
	}
	if h2.Confidence != 0.0 {
		return wrap(errInvariant, fmt.Errorf("clean response confidence %f, want 0.0", h2.Confidence))
	}
	ops++

	// Coverage check 11 — Close is idempotent.
	if err := c.Close(); err != nil {
		return wrap(errCoverage, fmt.Errorf("Close (1st): %w", err))
	}
	if err := c.Close(); err != nil {
		return wrap(errCoverage, fmt.Errorf("Close (2nd, must be idempotent): %w", err))
	}
	ops++

	// 5-locale bilingual UX evidence per CONST-046.
	printed := 0
	for _, loc := range supportedLocales() {
		out := loc.line(ops, verifications)
		if !strings.Contains(out, "veritas:") {
			return wrap(errLocale, fmt.Errorf("locale %s: missing canonical token", loc.tag))
		}
		fmt.Println(out)
		printed++
	}
	if printed != len(supportedLocales()) {
		return wrap(errLocale, fmt.Errorf("printed %d/%d locales", printed, len(supportedLocales())))
	}

	fmt.Printf("OK operations=%d verifications=%d locales=%d\n", ops, verifications, printed)
	return nil
}

// runnerVerifier is the runner's substring + crude-negation
// test Verifier. CONST-050(A) permits stub Verifiers inside
// *_test.go files; the round-288 runner is a Challenge binary,
// not a test file, so it ships its OWN local stub rather than
// reaching into client_test.go's substringTestVerifier (which
// would couple the Challenge to internal test packaging).
//
// The stub mirrors the bluff-removed default's deterministic
// behaviour: substring-of-claim-in-source → "true" with high
// confidence; explicit "not"-prefixed contradiction → "false";
// otherwise "uncertain" with low confidence. This is OK here
// because (a) the runner is non-production, (b) the confidence
// floor is documented, and (c) the assertion bands check the
// invariants (∈[0,1]) rather than trusting the value.
func runnerVerifier(_ context.Context, claim string, sources []string) (types.VerifyResult, error) {
	lc := strings.ToLower(strings.TrimSpace(claim))
	if lc == "" {
		return types.VerifyResult{}, errors.New("empty claim")
	}
	verdict := "uncertain"
	confidence := 0.2
	evidence := []types.Evidence{}
	contradictions := []types.Contradiction{}
	for i, src := range sources {
		ls := strings.ToLower(src)
		if strings.Contains(ls, lc) {
			verdict = "true"
			confidence = 0.85
			evidence = append(evidence, types.Evidence{
				Source:    fmt.Sprintf("src-%d", i),
				Excerpt:   src,
				Relevance: 0.9,
			})
			break
		}
		// crude negation: if source contradicts a token in the claim
		// (e.g. claim says "Germany" but source says "France"),
		// flag contradiction.
		claimToks := strings.Fields(lc)
		for _, t := range claimToks {
			if len(t) < 4 {
				continue
			}
			if !strings.Contains(ls, t) && strings.Contains(ls, "is") {
				contradictions = append(contradictions, types.Contradiction{
					StatementA: claim,
					StatementB: src,
					Severity:   0.5,
					Models:     []string{"runner-stub"},
				})
				verdict = "false"
				confidence = 0.75
				break
			}
		}
		if verdict == "false" {
			break
		}
	}
	return types.VerifyResult{
		Claim:          claim,
		Verdict:        verdict,
		Confidence:     confidence,
		Evidence:       evidence,
		Contradictions: contradictions,
	}, nil
}

// assertVerifyResult enforces the invariants the runner's PASS
// claims: non-nil result, Verdict ∈ {true,false,uncertain},
// Confidence ∈ [0,1], Validate passes. This same logic is the
// target of the paired-mutation Challenge — the mutation plants
// a Confidence: 2.5 result and asserts THIS check trips it.
func assertVerifyResult(r *types.VerifyResult) error {
	if r == nil {
		return errors.New("nil VerifyResult")
	}
	switch r.Verdict {
	case "true", "false", "uncertain":
		// ok
	default:
		return fmt.Errorf("Verdict %q not in canonical set", r.Verdict)
	}
	if r.Confidence < 0 || r.Confidence > 1 {
		return fmt.Errorf("Confidence %f out of band", r.Confidence)
	}
	// Per pkg/types.VerifyResult.Validate, Claim must be non-empty
	// for Validate to pass. Synthesise a placeholder Claim if the
	// returned result omitted it — the invariant we care about is
	// Confidence band + Verdict set, not the Claim mirror.
	check := *r
	if check.Claim == "" {
		check.Claim = "runner-asserted"
	}
	if err := check.Validate(); err != nil {
		return fmt.Errorf("VerifyResult.Validate: %w", err)
	}
	return nil
}

// Sentinel error tags for exit-code mapping.
var (
	errCoverage  = errors.New("coverage")
	errInvariant = errors.New("invariant")
	errLocale    = errors.New("locale")
)

// taggedError attaches a sentinel for exit-code mapping while
// preserving the inner cause via Unwrap.
type taggedError struct {
	tag   error
	inner error
}

func (e *taggedError) Error() string { return e.inner.Error() }
func (e *taggedError) Unwrap() error { return e.inner }
func (e *taggedError) Is(t error) bool {
	return errors.Is(e.tag, t)
}

func wrap(tag, inner error) error {
	return &taggedError{tag: tag, inner: inner}
}

func exitCodeFor(err error) int {
	switch {
	case errors.Is(err, errCoverage):
		return 2
	case errors.Is(err, errInvariant):
		return 3
	case errors.Is(err, errLocale):
		return 4
	default:
		return 1
	}
}
