package client

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"digital.vasic.veritas/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// substringTestVerifier is the substring + crude-negation stand-in
// previously installed as the package-level default. Round-27 §11.4
// audit (2026-05-17) moved it here: CONST-050(A) permits stub
// Verifiers in *_test.go only. Production callers MUST inject a real
// LLM-backed Verifier via SetVerifier.
func substringTestVerifier(_ context.Context, claim string, sources []string) (types.VerifyResult, error) {
	lower := strings.ToLower(strings.TrimSpace(claim))
	verdict := "unknown"
	confidence := 0.2
	evidence := []types.Evidence{}
	contradictions := []types.Contradiction{}
	supported := 0
	contradicted := 0
	for i, s := range sources {
		lowerS := strings.ToLower(s)
		if strings.Contains(lowerS, lower) {
			supported++
			evidence = append(evidence, types.Evidence{
				Source:    fmt.Sprintf("source-%d", i),
				Excerpt:   testTruncate(s, 120),
				Relevance: 0.9,
			})
		} else if testNegatedMatch(lower, lowerS) {
			contradicted++
			contradictions = append(contradictions, types.Contradiction{
				StatementA: claim,
				StatementB: testTruncate(s, 120),
				Severity:   0.7,
			})
		}
	}
	switch {
	case supported > contradicted && supported > 0:
		verdict = "supported"
		confidence = 0.7 + float64(supported)*0.05
		if confidence > 0.95 {
			confidence = 0.95
		}
	case contradicted > supported && contradicted > 0:
		verdict = "contradicted"
		confidence = 0.7 + float64(contradicted)*0.05
		if confidence > 0.95 {
			confidence = 0.95
		}
	case len(sources) == 0:
		verdict = "unknown"
		confidence = 0.1
	}
	return types.VerifyResult{
		Claim:          claim,
		Verdict:        verdict,
		Confidence:     confidence,
		Evidence:       evidence,
		Contradictions: contradictions,
	}, nil
}

// testNegatedMatch / testTruncate replicate the package-private helpers
// the original substring verifier used; kept local to *_test.go so
// production code cannot accidentally rely on them.
func testNegatedMatch(claim, source string) bool {
	if strings.Contains(claim, " not ") {
		stripped := strings.Replace(claim, " not ", " ", 1)
		return strings.Contains(source, stripped)
	}
	for _, prefix := range []string{"is ", "are ", "was ", "were "} {
		if idx := strings.Index(claim, prefix); idx >= 0 {
			neg := claim[:idx+len(prefix)] + "not " + claim[idx+len(prefix):]
			if strings.Contains(source, neg) {
				return true
			}
		}
	}
	return false
}

func testTruncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// newTestClient builds a Client with the substringTestVerifier
// installed so unit tests have deterministic substring-based
// verdicts without depending on a real LLM provider. Mirrors the
// AutoTemp newTestClient pattern (round-23 §11.4 audit).
func newTestClient(t *testing.T) *Client {
	t.Helper()
	c, err := New()
	require.NoError(t, err)
	c.SetVerifier(substringTestVerifier)
	return c
}

func TestNew(t *testing.T) {
	client, err := New()
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.NoError(t, client.Close())
}

func TestDoubleClose(t *testing.T) {
	client, err := New()
	require.NoError(t, err)
	assert.NoError(t, client.Close())
	assert.NoError(t, client.Close())
}

func TestConfig(t *testing.T) {
	client, err := New()
	require.NoError(t, err)
	defer client.Close()
	assert.NotNil(t, client.Config())
}

// TestVerifyClaimWithoutInjectedVerifier_ReturnsSentinel asserts the
// round-27 §11.4 audit fix: New()'s default Verifier returns
// ErrBaselineVerifierNotConfigured when SetVerifier has not been
// called, instead of the previous silent substring-match-with-
// fabricated-confidence behaviour.
func TestVerifyClaimWithoutInjectedVerifier_ReturnsSentinel(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()

	_, err = c.VerifyClaim(context.Background(), types.VerifyRequest{
		Claim:            "the sky is blue",
		ReferenceSources: []string{"the sky is blue today"},
	})
	require.Error(t, err, "VerifyClaim without injected Verifier MUST surface the sentinel, not fabricate a verdict")
	require.True(t, errors.Is(err, ErrBaselineVerifierNotConfigured),
		"expected errors.Is(err, ErrBaselineVerifierNotConfigured), got: %v", err)
}

// TestBatchVerifyWithoutInjectedVerifier_ReturnsSentinel — BatchVerify
// composes on VerifyClaim so the sentinel propagates.
func TestBatchVerifyWithoutInjectedVerifier_ReturnsSentinel(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()

	_, err = c.BatchVerify(context.Background(), []string{"claim 1"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrBaselineVerifierNotConfigured),
		"expected errors.Is(err, ErrBaselineVerifierNotConfigured), got: %v", err)
}

// TestCompareModelsWithoutInjectedVerifier_ReturnsSentinel — same.
func TestCompareModelsWithoutInjectedVerifier_ReturnsSentinel(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()

	_, err = c.CompareModels(context.Background(), "claim", []string{"gpt-4"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrBaselineVerifierNotConfigured),
		"expected errors.Is(err, ErrBaselineVerifierNotConfigured), got: %v", err)
}

func TestVerifyClaimSupported(t *testing.T) {
	c := newTestClient(t)
	defer c.Close()

	r, err := c.VerifyClaim(context.Background(), types.VerifyRequest{
		Claim: "the sky is blue",
		ReferenceSources: []string{
			"During the day, the sky is blue because of Rayleigh scattering.",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "supported", r.Verdict)
	assert.Greater(t, r.Confidence, 0.5)
	assert.NotEmpty(t, r.Evidence)
}

func TestVerifyClaimUnknown(t *testing.T) {
	c := newTestClient(t)
	defer c.Close()

	r, err := c.VerifyClaim(context.Background(), types.VerifyRequest{
		Claim: "quantum entanglement defies causality",
	})
	require.NoError(t, err)
	assert.Equal(t, "unknown", r.Verdict)
}

func TestVerifyClaimInvalid(t *testing.T) {
	c := newTestClient(t)
	defer c.Close()

	_, err := c.VerifyClaim(context.Background(), types.VerifyRequest{})
	assert.Error(t, err)
}

func TestCheckConsistency(t *testing.T) {
	c := newTestClient(t)
	defer c.Close()

	r, err := c.CheckConsistency(context.Background(),
		[]string{"the cat sat on the mat", "the cat sat on the mat"},
		[]string{"m1", "m2"})
	require.NoError(t, err)
	assert.InDelta(t, 1.0, r.ConsistencyScore, 1e-9)

	r2, err := c.CheckConsistency(context.Background(),
		[]string{"red apples are sweet", "green apples are sour"},
		[]string{"m1", "m2"})
	require.NoError(t, err)
	assert.Less(t, r2.ConsistencyScore, 1.0)
}

func TestDetectHallucination(t *testing.T) {
	c := newTestClient(t)
	defer c.Close()

	r, err := c.DetectHallucination(context.Background(),
		"As I mentioned earlier, in 2023 scientists proved this.", "gpt-4")
	require.NoError(t, err)
	assert.True(t, r.Hallucinated)
	assert.NotEmpty(t, r.SuspiciousSegments)

	r2, err := c.DetectHallucination(context.Background(),
		"Water boils at 100 degrees Celsius at sea level.", "gpt-4")
	require.NoError(t, err)
	assert.False(t, r2.Hallucinated)
}

func TestCompareModels(t *testing.T) {
	c := newTestClient(t)
	defer c.Close()
	c.AddSource("encyclopedia", "the earth is round")

	r, err := c.CompareModels(context.Background(), "the earth is round",
		[]string{"gpt-4", "claude-3"})
	require.NoError(t, err)
	assert.Equal(t, 2, len(r.ModelResults))
}

func TestGetFactSources(t *testing.T) {
	c := newTestClient(t)
	defer c.Close()
	c.AddSource("wiki", "The Eiffel Tower is located in Paris.")
	c.AddSource("blog", "I like pizza.")

	ev, err := c.GetFactSources(context.Background(), "Eiffel Tower")
	require.NoError(t, err)
	assert.Len(t, ev, 1)
	assert.Equal(t, "wiki", ev[0].Source)
}

func TestBatchVerify(t *testing.T) {
	c := newTestClient(t)
	defer c.Close()

	rs, err := c.BatchVerify(context.Background(),
		[]string{"claim 1", "claim 2"})
	require.NoError(t, err)
	assert.Len(t, rs, 2)
}

func TestSetVerifier(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()

	c.SetVerifier(func(_ context.Context, claim string, _ []string) (types.VerifyResult, error) {
		return types.VerifyResult{Claim: claim, Verdict: "forced", Confidence: 0.42}, nil
	})
	r, err := c.VerifyClaim(context.Background(), types.VerifyRequest{Claim: "anything"})
	require.NoError(t, err)
	assert.Equal(t, "forced", r.Verdict)
}
