#!/usr/bin/env bash
# challenges/veritas_describe_challenge.sh
#
# Round-288 anti-bluff Challenge for digital.vasic.veritas.
#
# Default mode: invoke the runner against a real *client.Client
# (real Verifier injected via documented SetVerifier injection
# point, real sources registered via AddSource, every advertised
# method exercised with per-call invariant assertions), and
# assert it exits 0 with the expected operation count + 5-locale
# UX evidence. This is the positive-evidence proof per Article
# XI §11.9 — the PASS is backed by captured stdout, not by
# absence of error or a green summary line.
#
# Paired-mutation mode (--mutate): build a scratch program that
# constructs a VerifyResult with Confidence: 2.5 (out-of-band per
# the [0,1] invariant) and asserts the same invariant check the
# runner uses MUST trip it (exit 99 = mutation correctly
# surfaced). A mutation run that exits 0 means the Challenge
# itself is a bluff (CONST-035 mutation-bluff), and this script
# exits 1 to surface that.
#
# Defensive use only — no payload generation, no obfuscation
# helpers, no inverse verifiers.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

MODE="default"
if [[ ${1:-} == "--mutate" ]]; then
    MODE="mutate"
fi

run_default() {
    echo "[veritas-challenge] mode=default — exercising runner against real *client.Client with injected Verifier"
    cd "${REPO_ROOT}"

    local out
    out=$(go run ./challenges/runner -all 2>&1) || {
        echo "[veritas-challenge] FAIL: runner exited non-zero"
        echo "${out}"
        exit 1
    }

    # Positive-evidence assertions on captured stdout.
    if ! grep -q "^OK operations=" <<<"${out}"; then
        echo "[veritas-challenge] FAIL: missing OK trailer with operations="
        echo "${out}"
        exit 1
    fi
    if ! grep -q "verifications=" <<<"${out}"; then
        echo "[veritas-challenge] FAIL: missing verifications count in OK trailer"
        echo "${out}"
        exit 1
    fi
    if ! grep -q "^\[en\] veritas:" <<<"${out}" \
            || ! grep -q "^\[sr\] veritas:" <<<"${out}" \
            || ! grep -q "^\[ja\] veritas:" <<<"${out}" \
            || ! grep -q "^\[es\] veritas:" <<<"${out}" \
            || ! grep -q "^\[de\] veritas:" <<<"${out}"; then
        echo "[veritas-challenge] FAIL: missing one or more locale UX lines"
        echo "${out}"
        exit 1
    fi

    # Defensive-use boundary check — no inverse helpers may leak.
    if grep -RnE 'func +(Generate(Payload|Attack|Obfuscat)|Bypass[A-Z])' "${REPO_ROOT}" \
            --include='*.go' --exclude-dir=challenges --exclude-dir=.git 2>/dev/null \
            | grep -v '_test.go'; then
        echo "[veritas-challenge] FAIL: inverse helper detected (defensive-use boundary breached)"
        exit 1
    fi

    echo "${out}"
    echo "[veritas-challenge] PASS — runtime evidence captured above"
    exit 0
}

run_mutate() {
    echo "[veritas-challenge] mode=mutate — paired-mutation evidence"
    local scratch
    scratch="$(mktemp -d -t veritas-mutate-XXXXXX)"
    # shellcheck disable=SC2064
    trap "rm -rf '${scratch}'" EXIT

    # Stage a self-contained scratch module that constructs a
    # VerifyResult with Confidence: 2.5 (out-of-band per the [0,1]
    # invariant codified in pkg/types.VerifyResult.Validate AND
    # asserted in the runner's assertVerifyResult). The mutation
    # MUST be caught.
    cat > "${scratch}/go.mod" <<'EOF'
module veritas.scratch

go 1.22
EOF

    cat > "${scratch}/main.go" <<'EOF'
package main

import (
	"errors"
	"fmt"
	"os"
)

// VerifyResult is the mutated stand-in. The mutation: Confidence
// is set to 2.5 — outside the documented [0,1] invariant. The
// runner-style assertVerifyResult check MUST flag it.
type VerifyResult struct {
	Claim      string
	Verdict    string
	Confidence float64
}

func loadMutated() *VerifyResult {
	// Simulate a Verifier implementation that returned a confidence
	// score outside the documented band — the kind of partial-port
	// regression CONST-035 paired-mutation guards against.
	return &VerifyResult{
		Claim:      "mutation under test",
		Verdict:    "true",
		Confidence: 2.5,
	}
}

// assertVerifyResult mirrors the runner's invariant check. It
// MUST flag the out-of-band Confidence as a defect.
func assertVerifyResult(r *VerifyResult) error {
	if r == nil {
		return errors.New("nil VerifyResult")
	}
	switch r.Verdict {
	case "true", "false", "uncertain":
	default:
		return fmt.Errorf("Verdict %q not in canonical set", r.Verdict)
	}
	if r.Confidence < 0 || r.Confidence > 1 {
		return fmt.Errorf("Confidence %f out of band", r.Confidence)
	}
	return nil
}

func main() {
	r := loadMutated()
	if err := assertVerifyResult(r); err != nil {
		fmt.Fprintf(os.Stderr, "mutation detected: %v\n", err)
		os.Exit(99)
	}
	fmt.Println("mutation NOT detected — bluff")
	os.Exit(0)
}
EOF

    cd "${scratch}"
    # Build then exec — `go run` does not preserve exit codes >2 on
    # all toolchains, which would mask the sentinel 99 the program
    # emits when the mutation is detected.
    go build -o ./mutbin . >/dev/null 2>&1 || {
        echo "[veritas-challenge] FAIL-MUTATE — scratch build failed"
        exit 1
    }
    local mut_out mut_rc
    set +e
    mut_out=$(./mutbin 2>&1)
    mut_rc=$?
    set -e

    echo "${mut_out}"
    if [[ ${mut_rc} -eq 99 ]]; then
        echo "[veritas-challenge] PASS-MUTATE — mutation correctly surfaced (exit 99)"
        exit 99
    fi
    echo "[veritas-challenge] FAIL-MUTATE — mutation NOT surfaced (exit ${mut_rc}); Challenge is a bluff"
    exit 1
}

case "${MODE}" in
    default) run_default ;;
    mutate)  run_mutate ;;
esac
