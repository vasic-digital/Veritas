// Package client provides the Go client for the Veritas library.
//
// Veritas performs AI-truthfulness verification, simple fact-checking,
// hallucination detection, and response-consistency analysis.
//
// IMPORTANT — round-27 §11.4 audit (2026-05-17): the substring-and-
// keyword "baseline pipeline" that previously shipped as the default
// Verifier on New() / NewFromConfig() was a PASS-bluff at the
// library-default layer. It fabricated supported/contradicted/unknown
// verdicts with hardcoded confidence scores (0.2 / 0.7+0.05× / 0.95)
// derived purely from substring matches against the supplied sources,
// while presenting itself as a real fact-verification engine. Callers
// who forgot to inject a real LLM-backed Verifier via SetVerifier
// received fabricated confidence numbers and trusted them.
//
// The bluff has been removed. baselineVerifier now returns
// ErrBaselineVerifierNotConfigured. Callers MUST inject a real
// Verifier via SetVerifier before invoking VerifyClaim /
// BatchVerify / CompareModels.
//
// Basic usage:
//
//	import veritas "digital.vasic.veritas/pkg/client"
//
//	c, err := veritas.New()
//	if err != nil { log.Fatal(err) }
//	defer c.Close()
//
//	// REQUIRED — wire a real Verifier (LLM-backed or rule-based)
//	// before invoking VerifyClaim. Without this, VerifyClaim
//	// surfaces ErrBaselineVerifierNotConfigured.
//	c.SetVerifier(myRealLLMVerifier)
package client

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"digital.vasic.pliniuscommon/pkg/config"
	"digital.vasic.pliniuscommon/pkg/errors"

	. "digital.vasic.veritas/pkg/types"
)

// ErrBaselineVerifierNotConfigured is returned by VerifyClaim (and the
// helpers that compose on it — BatchVerify, CompareModels) when no
// Verifier has been injected via SetVerifier. Prior to round-27 §11.4
// audit (2026-05-17), the package's default Verifier was a substring-
// matching "baseline" that emitted fabricated verdicts with hardcoded
// confidence scores while pretending to be a real verification
// pipeline. That bluff has been replaced with this sentinel so
// callers cannot mistake the absence of a real Verifier for working
// verification.
//
// Tests that need deterministic verdicts for assertion purposes
// install a unit-test helper via SetVerifier — see
// substringTestVerifier in client_test.go (CONST-050(A) permits
// stubs in *_test.go only).
//
// Constitutional anchors: CONST-035 (anti-bluff), CONST-050(A)
// (no-fakes-beyond-unit-tests), Article XI §11.9 (forensic anchor).
var ErrBaselineVerifierNotConfigured = fmt.Errorf("veritas: baseline Verifier has not been replaced — call client.SetVerifier(...) with a real LLM-backed verifier before invoking Verify (the previous baseline default produced fabricated keyword-match verdicts with hardcoded confidence scores; §11.4 PASS-bluff removed)")

// Verifier evaluates a claim against reference sources.
type Verifier func(ctx context.Context, claim string, sources []string) (VerifyResult, error)

// Client is the Go client for Veritas.
type Client struct {
	cfg    *config.Config
	mu     sync.RWMutex
	closed bool

	verifier Verifier
	sources  map[string]string // keyed by source id -> corpus text
}

// New creates a new Veritas client with the baseline verifier seeded.
func New(opts ...config.Option) (*Client, error) {
	cfg := config.New("veritas", opts...)
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "veritas",
			"invalid configuration", err)
	}
	return &Client{
		cfg:      cfg,
		verifier: baselineVerifier,
		sources:  make(map[string]string),
	}, nil
}

// NewFromConfig creates a client from a config object.
func NewFromConfig(cfg *config.Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "veritas",
			"invalid configuration", err)
	}
	return &Client{
		cfg:      cfg,
		verifier: baselineVerifier,
		sources:  make(map[string]string),
	}, nil
}

// Close gracefully closes the client.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return nil
}

// Config returns the client configuration.
func (c *Client) Config() *config.Config { return c.cfg }

// SetVerifier swaps the verification implementation.
func (c *Client) SetVerifier(v Verifier) {
	if v == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.verifier = v
}

// AddSource registers a reference-source corpus by id.
func (c *Client) AddSource(id, text string) {
	if id == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sources[id] = text
}

// VerifyClaim runs the configured verifier.
func (c *Client) VerifyClaim(ctx context.Context, req VerifyRequest) (*VerifyResult, error) {
	if err := req.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "veritas",
			"invalid parameters", err)
	}
	sources := req.ReferenceSources
	if len(sources) == 0 {
		// fall back to known registered sources
		c.mu.RLock()
		for _, txt := range c.sources {
			sources = append(sources, txt)
		}
		c.mu.RUnlock()
	}
	c.mu.RLock()
	v := c.verifier
	c.mu.RUnlock()
	r, err := v(ctx, req.Claim, sources)
	if err != nil {
		return nil, errors.Wrap(errors.ErrCodeUnavailable, "veritas",
			"verifier failed", err)
	}
	out := r
	return &out, nil
}

// CheckConsistency scores the pairwise consistency of a set of responses
// using a shingled Jaccard similarity. models is optional metadata.
func (c *Client) CheckConsistency(ctx context.Context, responses []string, models []string) (*ConsistencyCheck, error) {
	if len(responses) < 2 {
		return &ConsistencyCheck{
			Responses:        responses,
			Models:           models,
			ConsistencyScore: 1.0,
		}, nil
	}
	pairs := 0
	total := 0.0
	for i := 0; i < len(responses); i++ {
		for j := i + 1; j < len(responses); j++ {
			total += jaccard(tokens(responses[i]), tokens(responses[j]))
			pairs++
		}
	}
	score := total / float64(pairs)
	// Divergent points: tokens appearing in some responses but not others.
	counts := map[string]int{}
	for _, r := range responses {
		for tok := range toSet(tokens(r)) {
			counts[tok]++
		}
	}
	divergent := []string{}
	for tok, cnt := range counts {
		if cnt < len(responses) && cnt > 0 {
			divergent = append(divergent, tok)
		}
	}
	sort.Strings(divergent)
	if len(divergent) > 10 {
		divergent = divergent[:10]
	}
	return &ConsistencyCheck{
		Responses:        responses,
		ConsistencyScore: score,
		Models:           models,
		DivergentPoints:  divergent,
	}, nil
}

// DetectHallucination flags suspicious segments via a small trigger list.
func (c *Client) DetectHallucination(ctx context.Context, response string, model string) (*HallucinationResult, error) {
	triggers := []string{
		"as i mentioned earlier",
		"according to the latest study",
		"it is widely known",
		"in 20", // ungrounded year claim
	}
	lower := strings.ToLower(response)
	hall := false
	suspicious := []string{}
	for _, t := range triggers {
		if strings.Contains(lower, t) {
			hall = true
			suspicious = append(suspicious, t)
		}
	}
	conf := 0.0
	if hall {
		conf = 0.6 + float64(len(suspicious))*0.05
		if conf > 0.95 {
			conf = 0.95
		}
	}
	return &HallucinationResult{
		SuspiciousSegments: suspicious,
		Confidence:         conf,
		Hallucinated:       hall,
	}, nil
}

// CompareModels runs VerifyClaim per model (treating model name as source).
func (c *Client) CompareModels(ctx context.Context, claim string, models []string) (*ModelComparison, error) {
	if claim == "" {
		return nil, errors.New(errors.ErrCodeInvalidArgument, "veritas", "claim is required")
	}
	results := make(map[string]VerifyResult, len(models))
	var bestModel, worstModel string
	bestScore := -1.0
	worstScore := 2.0
	for _, m := range models {
		vr, err := c.VerifyClaim(ctx, VerifyRequest{
			Claim:       claim,
			SourceModel: m,
		})
		if err != nil {
			return nil, err
		}
		results[m] = *vr
		if vr.Confidence > bestScore {
			bestScore = vr.Confidence
			bestModel = m
		}
		if vr.Confidence < worstScore {
			worstScore = vr.Confidence
			worstModel = m
		}
	}
	agreement := 0.0
	if bestScore >= 0 && worstScore <= 1 {
		agreement = 1 - (bestScore - worstScore)
	}
	return &ModelComparison{
		Claim:          claim,
		ModelResults:   results,
		AgreementScore: agreement,
		MostAccurate:   bestModel,
		LeastAccurate:  worstModel,
	}, nil
}

// GetFactSources returns registered sources that reference the claim terms.
func (c *Client) GetFactSources(ctx context.Context, claim string) ([]Evidence, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	lower := strings.ToLower(claim)
	out := []Evidence{}
	for id, txt := range c.sources {
		if strings.Contains(strings.ToLower(txt), lower) {
			out = append(out, Evidence{
				Source:    id,
				Excerpt:   truncate(txt, 200),
				Relevance: 1.0,
			})
		}
	}
	return out, nil
}

// BatchVerify calls VerifyClaim for each claim.
func (c *Client) BatchVerify(ctx context.Context, claims []string) ([]VerifyResult, error) {
	out := make([]VerifyResult, 0, len(claims))
	for _, claim := range claims {
		r, err := c.VerifyClaim(ctx, VerifyRequest{Claim: claim})
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, nil
}

// --- helpers ---

func tokens(s string) []string {
	f := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r == '_' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})
	return f
}

func toSet(xs []string) map[string]struct{} {
	m := make(map[string]struct{}, len(xs))
	for _, x := range xs {
		m[x] = struct{}{}
	}
	return m
}

func jaccard(a, b []string) float64 {
	sa, sb := toSet(a), toSet(b)
	if len(sa) == 0 && len(sb) == 0 {
		return 1.0
	}
	inter := 0
	for k := range sa {
		if _, ok := sb[k]; ok {
			inter++
		}
	}
	union := len(sa) + len(sb) - inter
	if union == 0 {
		return 1.0
	}
	return float64(inter) / float64(union)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// baselineVerifier previously implemented a substring + crude-negation
// "verdict" with hardcoded confidence scores (0.2 / 0.7+0.05× / 0.95).
// Per round-27 §11.4 audit (2026-05-17), any caller that forgot to
// call SetVerifier before invoking VerifyClaim received fabricated
// confidence numbers with no error surfaced — CRITICAL PASS-bluff at
// the library-default layer.
//
// Fix: baselineVerifier now returns ErrBaselineVerifierNotConfigured.
// New() / NewFromConfig() still seed this function as the default so
// client construction stays cheap, but the first call to VerifyClaim
// (or BatchVerify / CompareModels which compose on it) without an
// injected real Verifier surfaces the sentinel error explaining what
// to do (call SetVerifier).
//
// The previous substring-matching logic has been preserved in
// substringTestVerifier inside client_test.go where CONST-050(A)
// permits stub Verifiers; tests install it via SetVerifier.
func baselineVerifier(_ context.Context, _ string, _ []string) (VerifyResult, error) {
	return VerifyResult{}, ErrBaselineVerifierNotConfigured
}
