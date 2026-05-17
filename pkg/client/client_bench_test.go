package client

import (
	"context"
	"testing"

	"digital.vasic.veritas/pkg/types"
)

// BenchmarkVerifyClaim measures the cost of the substring test
// verifier (CONST-050(A) permits stubs in *_test.go only — the
// production default returns ErrBaselineVerifierNotConfigured per
// round-27 §11.4 audit). The benchmark therefore wires
// substringTestVerifier explicitly to keep the measurement
// meaningful.
func BenchmarkVerifyClaim(b *testing.B) {
	c, err := New()
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close()
	c.SetVerifier(substringTestVerifier)
	ctx := context.Background()
	req := types.VerifyRequest{
		Claim:            "water boils at 100",
		ReferenceSources: []string{"at sea level water boils at 100", "water boils at 100 celsius"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := c.VerifyClaim(ctx, req); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCheckConsistency(b *testing.B) {
	c, err := New()
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close()
	ctx := context.Background()
	responses := []string{
		"the sky is blue during the day",
		"the sky appears blue due to rayleigh scattering",
		"a clear sky is blue",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := c.CheckConsistency(ctx, responses, nil); err != nil {
			b.Fatal(err)
		}
	}
}
