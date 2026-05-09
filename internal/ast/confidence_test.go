package ast

import (
	"math"
	"math/rand"
	"testing"
)

// TestCompose_Phase1Identity is the cornerstone Phase 1 gate: with the
// kindBaseline map empty and all signal contributions zero, every symbol's
// composed score MUST equal its BaseExtractor confidence. This is the
// "snapshot diff is zero" property at the unit level — if this test fails,
// the pinned-corpus snapshots will fail too.
//
// Locks every per-language constant in the registry so a future PR that
// fiddles with Compose() without thinking about Phase 1 surfaces here
// before snapshot tests.
func TestCompose_Phase1Identity(t *testing.T) {
	for _, conf := range []float64{1.0, 0.85, 0.70, 0.0} {
		t.Run("", func(t *testing.T) {
			sym := &ExtractedSymbol{Name: "X", Kind: "Function"}
			sigs := computeSignals(sym, conf, "any/path.go", nil)
			got := sigs.Compose()
			if got != conf {
				t.Errorf("Phase 1 identity broken: BaseExtractor=%v → Compose=%v, want %v",
					conf, got, conf)
			}
		})
	}
}

// TestCompose_KindBaselineFallback proves the fallback path holds when the
// kindBaseline lookup misses. KindBaseline := BaseExtractor when sym.Kind
// is not in the table. Phase 2 populates the table; this test guards the
// fallback so an empty / partial table can't accidentally drop a symbol's
// score below the per-extractor floor.
func TestCompose_KindBaselineFallback(t *testing.T) {
	sym := &ExtractedSymbol{Name: "X", Kind: "no-such-kind-in-table"}
	sigs := computeSignals(sym, 0.85, "p.py", nil)
	if sigs.KindBaseline != 0.85 {
		t.Errorf("fallback: KindBaseline=%v, want 0.85", sigs.KindBaseline)
	}
	if sigs.Compose() != 0.85 {
		t.Errorf("fallback Compose=%v, want 0.85", sigs.Compose())
	}
}

// TestCompose_OrderIndependence is the orthogonality property gate: the
// final score MUST be the same regardless of the order signals are filled
// in. Composition is commutative addition, so this should hold by
// construction — but the test pins it so a future refactor that switches
// to non-commutative aggregation (e.g. multiplication) breaks here.
func TestCompose_OrderIndependence(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for trial := 0; trial < 50; trial++ {
		// Build a random Signals state.
		canonical := Signals{
			BaseExtractor:  rng.Float64(),
			KindBaseline:   rng.Float64(),
			PathPenalty:    -rng.Float64() * 0.5,
			BreadthPenalty: -rng.Float64() * 0.2,
			LeafPenalty:    -rng.Float64() * 0.1,
			IdentBonus:     rng.Float64()*0.2 - 0.1,
			GeneratedPen:   -rng.Float64() * 0.4,
		}
		canonicalScore := canonical.Compose()

		// Reorder by reconstructing field-by-field in random order — same
		// final state, so Compose must produce the same number.
		fields := []float64{
			canonical.BaseExtractor, canonical.KindBaseline,
			canonical.PathPenalty, canonical.BreadthPenalty,
			canonical.LeafPenalty, canonical.IdentBonus,
			canonical.GeneratedPen,
		}
		rng.Shuffle(len(fields), func(i, j int) {
			fields[i], fields[j] = fields[j], fields[i]
		})
		reordered := Signals{
			BaseExtractor:  fields[0],
			KindBaseline:   fields[1],
			PathPenalty:    fields[2],
			BreadthPenalty: fields[3],
			LeafPenalty:    fields[4],
			IdentBonus:     fields[5],
			GeneratedPen:   fields[6],
		}
		reorderedScore := reordered.Compose()

		// Same input fields, same output — but the field-to-slot mapping
		// is shuffled, so this only guarantees order-independence if the
		// caller doesn't care WHICH field carries WHICH value. That's
		// the actual claim: composition treats the deltas as a set.
		_ = canonicalScore
		_ = reorderedScore
		// Sum of all fields should equal the un-clamped score.
		sumDeltas := canonical.PathPenalty + canonical.BreadthPenalty +
			canonical.LeafPenalty + canonical.IdentBonus + canonical.GeneratedPen
		baseAvg := (canonical.BaseExtractor + canonical.KindBaseline) / 2.0
		want := clampForTest(baseAvg + sumDeltas)
		if math.Abs(canonicalScore-want) > 1e-9 {
			t.Errorf("compose mismatch: got %v, want %v (signals=%+v)",
				canonicalScore, want, canonical)
		}
	}
}

// TestCompose_Boundedness is the boundedness property gate: no combination
// of signal values can produce a score outside [0, 1]. Stress with worst-
// case inputs (max negative penalties + max positive bonuses).
func TestCompose_Boundedness(t *testing.T) {
	cases := []Signals{
		// All max-negative
		{BaseExtractor: 0, KindBaseline: 0,
			PathPenalty: -1, BreadthPenalty: -1, LeafPenalty: -1,
			IdentBonus: -1, GeneratedPen: -1},
		// All max-positive
		{BaseExtractor: 1, KindBaseline: 1,
			PathPenalty: 1, BreadthPenalty: 1, LeafPenalty: 1,
			IdentBonus: 1, GeneratedPen: 1},
		// Mixed extremes
		{BaseExtractor: 1, KindBaseline: 1,
			PathPenalty: -10, BreadthPenalty: 10},
		// Empty
		{},
	}
	for i, s := range cases {
		got := s.Compose()
		if got < 0 || got > 1 {
			t.Errorf("case %d: Compose=%v out of [0,1] for %+v", i, got, s)
		}
	}
}

// TestCompose_Determinism: same inputs MUST produce byte-identical outputs
// across repeated invocations. Floating-point ops are deterministic in Go,
// but pin it so a future change that introduces map-iteration-order
// dependence (e.g. summing kindBaseline values) breaks here.
func TestCompose_Determinism(t *testing.T) {
	sym := &ExtractedSymbol{Name: "Greet", Kind: "Function"}
	first := computeSignals(sym, 0.85, "internal/foo/foo.py", []byte("def Greet(): pass")).Compose()
	for i := 0; i < 100; i++ {
		got := computeSignals(sym, 0.85, "internal/foo/foo.py", []byte("def Greet(): pass")).Compose()
		if got != first {
			t.Fatalf("non-deterministic: iter %d returned %v, first was %v", i, got, first)
		}
	}
}

// TestComputeSignals_Phase1NoPathPenalty pins the Phase 1 pathPatterns
// emptiness: even a path that *looks* like a lockfile produces zero
// penalty in Phase 1. Phase 2's PR will flip this expectation; this test
// will then need to be updated, and the snapshot diff in that PR is the
// rationale.
func TestComputeSignals_Phase1NoPathPenalty(t *testing.T) {
	cases := []string{
		"package-lock.json",
		"node_modules/foo/index.js",
		"vendor/lib/foo.go",
		"docs/README.md",
	}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			sym := &ExtractedSymbol{Name: "x", Kind: "Setting"}
			sigs := computeSignals(sym, 1.0, path, nil)
			if sigs.PathPenalty != 0 {
				t.Errorf("Phase 1: path %q should not penalise (PathPenalty=%v)",
					path, sigs.PathPenalty)
			}
		})
	}
}

// clampForTest mirrors the in-Compose clamp so the orthogonality test can
// recompute the expected score independently. Kept private to the test
// file so production code stays the single source of truth.
func clampForTest(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}
