package ast

// Per-symbol confidence scoring (#34 Phase 1 — substrate, zero behavior change).
//
// Today every symbol from a given extractor gets that extractor's per-language
// constant (1.0 / 0.85 / 0.70). Phase 1 introduces the composition machinery
// without changing any output: kindBaseline is empty, pathPatterns is empty,
// every signal contributes 0. Compose() reduces to the per-extractor constant
// for every symbol, byte-identical to today. The snapshot diff for this PR is
// zero — that is the gate.
//
// Phase 2 (separate PR) populates the lookup tables and wires the per-symbol
// signals; that PR's snapshot diff is the rationale.
//
// Phase 4 (separate PR) flips default min_confidence from 0.0 → 0.7 in the
// search/query/trace tools. Until then, scores are informational, not gating.
//
// See design/per-symbol-confidence.md for the full rationale.

// Signals carries the per-symbol score components. Each field is a pure
// function of (extractor output, file path, kind, source). Compose() reduces
// them to a single confidence in [0, 1]. Phase 1 leaves every contributor
// equal to the no-op value, so Compose() == BaseExtractor for every symbol.
type Signals struct {
	// BaseExtractor is the existing per-language constant from Extractor.Confidence().
	// 1.0 for AST-backed extractors, 0.85 for stable regex, 0.70 for approximate.
	BaseExtractor float64

	// KindBaseline reflects the symbol kind's structural informativeness.
	// Set by computeSignals from the kindBaseline lookup, or falls back to
	// BaseExtractor when the kind isn't in the table. Phase 1 always falls
	// back, so KindBaseline == BaseExtractor for every symbol.
	KindBaseline float64

	// PathPenalty is a negative contribution from the file path (lockfile,
	// vendor/, generated dist/, README, etc.). Phase 1 leaves this 0; Phase 2
	// populates pathPatterns. Always non-positive.
	PathPenalty float64

	// BreadthPenalty fires when a symbol's parent has unusually high fan-out
	// (lockfile-shape detection: hundreds of sibling Settings under one key).
	// Phase 1 leaves this 0; needs structural info wired through extractors.
	// Always non-positive.
	BreadthPenalty float64

	// LeafPenalty is a small negative for scalar-leaf settings (less
	// structurally informative than a parent mapping). Phase 1 leaves this 0;
	// needs structural info wired through extractors. Always non-positive.
	LeafPenalty float64

	// IdentBonus is +0.05 for clean identifiers, -0.10 for empty/whitespace
	// names. Phase 1 leaves this 0.
	IdentBonus float64

	// GeneratedPen fires on `// Code generated` markers in the file head.
	// Phase 1 leaves this 0.
	GeneratedPen float64
}

// Compose reduces the signals to a single confidence score.
//
// Composition: average(BaseExtractor, KindBaseline) + sum-of-deltas, then clamp
// to [0, 1]. Averaging the two baselines (rather than summing) keeps the
// expected range bounded regardless of how the lookups evolve. Penalties and
// bonuses then push the score within [0, 1].
//
// Order-independent by construction (commutative addition of the deltas).
// Pure function — same inputs produce the same output, byte-identical, on
// any platform. These properties are pinned by tests in confidence_test.go.
func (s Signals) Compose() float64 {
	base := (s.BaseExtractor + s.KindBaseline) / 2.0
	score := base + s.PathPenalty + s.BreadthPenalty + s.LeafPenalty +
		s.IdentBonus + s.GeneratedPen
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

// kindBaseline maps symbol kinds to their structural-informativeness
// baseline. EMPTY in Phase 1 — every kind falls back to BaseExtractor in
// computeSignals, preserving today's per-language constant. Phase 2 (separate
// PR) populates this with values like Function:1.0, Setting:0.95, Section:0.80.
var kindBaseline = map[string]float64{}

// pathPatterns lists path-shape penalties (lockfile, vendor/, README, etc.).
// EMPTY in Phase 1 — no path produces a penalty. Phase 2 (separate PR)
// populates this with the curated list from the design doc.
var pathPatterns []pathPattern

// pathPattern is a single path-shape rule. Phase 2 will populate the global
// list; defining the type now lets composition tests construct synthetic
// signal sets.
type pathPattern struct {
	// Glob matches against either the file basename (default) or a directory
	// component (when IsDir is true). Behaves like filepath.Match.
	Glob    string
	Penalty float64
	IsDir   bool
	// Reason is surfaced on diagnostics so a snapshot diff that adds a penalty
	// can be reviewed by intent.
	Reason string
}

// computeSignals builds the Signals struct for one symbol. In Phase 1 every
// signal except BaseExtractor and KindBaseline (== BaseExtractor by fallback)
// is zero, so the composed score equals BaseExtractor for every symbol.
//
// Phase 2 populates the lookup tables; this function's signature stays
// the same so the wiring in extractor.go doesn't change between phases.
func computeSignals(sym *ExtractedSymbol, baseExtractor float64, relPath string, source []byte) Signals {
	s := Signals{BaseExtractor: baseExtractor}

	if k, ok := kindBaseline[sym.Kind]; ok {
		s.KindBaseline = k
	} else {
		s.KindBaseline = baseExtractor
	}

	// Phase 1: pathPatterns is empty, so this loop is a no-op. The structure
	// is here so Phase 2 can populate the table without touching wiring.
	for _, p := range pathPatterns {
		penalty := matchPathPattern(relPath, p)
		if penalty != 0 {
			s.PathPenalty = penalty
			break
		}
	}

	// IdentBonus / GeneratedPen / BreadthPenalty / LeafPenalty are all
	// intentionally not computed in Phase 1. Adding them is the Phase 2 PR;
	// adding them here would change the snapshot output.

	return s
}

// matchPathPattern returns the pattern's Penalty when relPath matches, else 0.
// Pulled out so per-pattern matching is unit-testable in isolation.
//
// Phase 1: invoked from a no-op loop (pathPatterns is empty). Phase 2 turns
// this into the real per-symbol path check.
func matchPathPattern(relPath string, p pathPattern) float64 {
	// Phase 2 wires real glob matching here. Phase 1 keeps the function as a
	// stub that always returns 0, so even synthetic signal construction in
	// tests can't accidentally short-circuit through.
	return 0
}
