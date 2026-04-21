package client

import (
	"context"
	stderrors "errors"
	"testing"

	"digital.vasic.veritas/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConsistencyIdenticalResponses — identical answers score 1.0.
func TestConsistencyIdenticalResponses(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()

	res, err := c.CheckConsistency(context.Background(),
		[]string{"the sky is blue", "the sky is blue", "the sky is blue"}, nil)
	require.NoError(t, err)
	assert.InDelta(t, 1.0, res.ConsistencyScore, 1e-9)
	assert.Empty(t, res.DivergentPoints)
}

// TestConsistencyDivergentResponses — mixed answers should have <1.0 score and divergent tokens.
func TestConsistencyDivergentResponses(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()

	res, err := c.CheckConsistency(context.Background(),
		[]string{"apple orange", "banana orange", "apple cherry"}, nil)
	require.NoError(t, err)
	assert.Less(t, res.ConsistencyScore, 1.0)
	assert.NotEmpty(t, res.DivergentPoints)
}

// TestConsistencySinglePassthrough — 1 response short-circuits to 1.0.
func TestConsistencySinglePassthrough(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()

	res, err := c.CheckConsistency(context.Background(), []string{"only"}, nil)
	require.NoError(t, err)
	assert.InDelta(t, 1.0, res.ConsistencyScore, 1e-9)
}

// TestBatchVerifyEmpty — empty claim list returns empty, no error.
func TestBatchVerifyEmpty(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()
	out, err := c.BatchVerify(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, out)
}

// TestBatchVerifyPropagatesError — one bad claim aborts the batch.
func TestBatchVerifyPropagatesError(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()
	// empty claim fails Validate.
	_, err = c.BatchVerify(context.Background(), []string{"valid", ""})
	assert.Error(t, err)
}

// TestDetectHallucinationAsciiBenign — plain ASCII without triggers returns clean.
func TestDetectHallucinationAsciiBenign(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()
	res, err := c.DetectHallucination(context.Background(), "a short clean response", "m")
	require.NoError(t, err)
	assert.False(t, res.Hallucinated)
	assert.InDelta(t, 0.0, res.Confidence, 1e-9)
}

// TestDetectHallucinationTrigger — trigger phrase flagged.
func TestDetectHallucinationTrigger(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()
	res, err := c.DetectHallucination(context.Background(),
		"as I mentioned earlier, according to the latest study, this is so", "m")
	require.NoError(t, err)
	assert.True(t, res.Hallucinated)
	assert.Greater(t, res.Confidence, 0.5)
	assert.NotEmpty(t, res.SuspiciousSegments)
}

// TestVerifyClaimContradicted — a source asserting the negation flips verdict.
func TestVerifyClaimContradicted(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()
	r, err := c.VerifyClaim(context.Background(), types.VerifyRequest{
		Claim:            "the ball is red",
		ReferenceSources: []string{"the ball is not red according to the spec"},
	})
	require.NoError(t, err)
	assert.Equal(t, "contradicted", r.Verdict)
	assert.Greater(t, r.Confidence, 0.5)
}

// TestVerifyClaimSupportedBySource — source containing the claim supports.
func TestVerifyClaimSupportedBySource(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()
	r, err := c.VerifyClaim(context.Background(), types.VerifyRequest{
		Claim:            "water boils at 100 celsius",
		ReferenceSources: []string{"at sea level, water boils at 100 celsius"},
	})
	require.NoError(t, err)
	assert.Equal(t, "supported", r.Verdict)
}

// TestVerifyClaimInjectedVerifierError — SetVerifier with erroring impl surfaces wrapped error.
func TestVerifyClaimInjectedVerifierError(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()
	c.SetVerifier(func(_ context.Context, _ string, _ []string) (types.VerifyResult, error) {
		return types.VerifyResult{}, stderrors.New("backend down")
	})
	_, err = c.VerifyClaim(context.Background(), types.VerifyRequest{Claim: "anything"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "verifier failed")
}

// TestAddSourceAndGetFactSources — registered sources indexed by substring match.
func TestAddSourceAndGetFactSources(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()
	c.AddSource("doc1", "The Eiffel Tower is in Paris.")
	c.AddSource("doc2", "London has Big Ben.")
	evs, err := c.GetFactSources(context.Background(), "Eiffel")
	require.NoError(t, err)
	require.Len(t, evs, 1)
	assert.Equal(t, "doc1", evs[0].Source)
}

// TestAddSourceEmptyIgnored.
func TestAddSourceEmptyIgnored(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()
	c.AddSource("", "x")
	evs, err := c.GetFactSources(context.Background(), "x")
	require.NoError(t, err)
	assert.Empty(t, evs)
}

// TestCompareModelsEmptyClaim.
func TestCompareModelsEmptyClaim(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()
	_, err = c.CompareModels(context.Background(), "", []string{"m1"})
	assert.Error(t, err)
}

// TestVerifyClaimInvalidRequest.
func TestVerifyClaimInvalidRequest(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()
	_, err = c.VerifyClaim(context.Background(), types.VerifyRequest{Claim: ""})
	assert.Error(t, err)
}
