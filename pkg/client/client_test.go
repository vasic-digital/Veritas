package client

import (
	"context"
	"testing"

	"digital.vasic.veritas/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestVerifyClaimSupported(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
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
	c, err := New()
	require.NoError(t, err)
	defer c.Close()

	r, err := c.VerifyClaim(context.Background(), types.VerifyRequest{
		Claim: "quantum entanglement defies causality",
	})
	require.NoError(t, err)
	assert.Equal(t, "unknown", r.Verdict)
}

func TestVerifyClaimInvalid(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()

	_, err = c.VerifyClaim(context.Background(), types.VerifyRequest{})
	assert.Error(t, err)
}

func TestCheckConsistency(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
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
	c, err := New()
	require.NoError(t, err)
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
	c, err := New()
	require.NoError(t, err)
	defer c.Close()
	c.AddSource("encyclopedia", "the earth is round")

	r, err := c.CompareModels(context.Background(), "the earth is round",
		[]string{"gpt-4", "claude-3"})
	require.NoError(t, err)
	assert.Equal(t, 2, len(r.ModelResults))
}

func TestGetFactSources(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
	defer c.Close()
	c.AddSource("wiki", "The Eiffel Tower is located in Paris.")
	c.AddSource("blog", "I like pizza.")

	ev, err := c.GetFactSources(context.Background(), "Eiffel Tower")
	require.NoError(t, err)
	assert.Len(t, ev, 1)
	assert.Equal(t, "wiki", ev[0].Source)
}

func TestBatchVerify(t *testing.T) {
	c, err := New()
	require.NoError(t, err)
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
