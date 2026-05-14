package ast

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Python AST extractor. Shells out to `python3` running an embedded helper
// script that uses CPython's `ast` module to parse the file and emit a JSON
// summary. The Go side unmarshals that into the standard FileResult shape.
//
// Default-on when python3 is on PATH (mirrors the JS-AST convention since
// v0.20.0). Falls back to the regex extractor on parse failure, missing
// python3, or `PINCHER_DISABLE_PY_AST=1`.

//go:embed python_extract.py
var pythonExtractScript string

// pyASTTimeout caps the wall-clock spent on a single file's Python AST run.
// CPython's ast.parse is fast; this only guards against pathological inputs
// or a hung subprocess. The regex fallback runs on timeout.
const pyASTTimeout = 10 * time.Second

// pyASTEnabled reads env vars on every call so tests can flip the flag with
// t.Setenv without re-registering the extractor.
//
// Resolution order:
//  1. PINCHER_DISABLE_PY_AST=1 → false (explicit opt-out wins)
//  2. python3 not on PATH      → false (transparent fallback to regex)
//  3. otherwise                → true  (default-on)
func pyASTEnabled() bool {
	if os.Getenv("PINCHER_DISABLE_PY_AST") == "1" {
		return false
	}
	return python3OnPATH()
}

var (
	pyLookupOnce sync.Once
	pyAvailable  bool
)

// python3OnPATH caches exec.LookPath so per-file extraction stays a
// no-syscall fast path after the first call in a process lifetime.
func python3OnPATH() bool {
	pyLookupOnce.Do(func() {
		if _, err := exec.LookPath("python3"); err == nil {
			pyAvailable = true
		}
	})
	return pyAvailable
}

// pythonSymbolJSON / pythonEdgeJSON / pythonResponse mirror the JSON shape
// emitted by python_extract.py. Keep them in sync with that script.
type pythonSymbolJSON struct {
	Name          string `json:"name"`
	QualifiedName string `json:"qualified_name"`
	Kind          string `json:"kind"`
	Parent        string `json:"parent"`
	Signature     string `json:"signature"`
	Docstring     string `json:"docstring"`
	IsExported    bool   `json:"is_exported"`
	IsTest        bool   `json:"is_test"`
	StartByte     int    `json:"start_byte"`
	EndByte       int    `json:"end_byte"`
	StartLine     int    `json:"start_line"`
	EndLine       int    `json:"end_line"`
}

type pythonEdgeJSON struct {
	FromQN     string  `json:"from_qn"`
	ToName     string  `json:"to_name"`
	Kind       string  `json:"kind"`
	Confidence float64 `json:"confidence"`
}

type pythonResponse struct {
	Symbols []pythonSymbolJSON `json:"symbols"`
	Edges   []pythonEdgeJSON   `json:"edges"`
	Module  string             `json:"module"`
	// Error is set by the script when ast.parse raises SyntaxError. Non-empty
	// means Go falls back to the regex extractor.
	Error string `json:"error,omitempty"`
}

func (r *pythonResponse) toFileResult() *FileResult {
	syms := make([]ExtractedSymbol, len(r.Symbols))
	for i, s := range r.Symbols {
		syms[i] = ExtractedSymbol{
			Name:          s.Name,
			QualifiedName: s.QualifiedName,
			Kind:          s.Kind,
			StartByte:     s.StartByte,
			EndByte:       s.EndByte,
			StartLine:     s.StartLine,
			EndLine:       s.EndLine,
			Signature:     s.Signature,
			Docstring:     s.Docstring,
			Parent:        s.Parent,
			IsExported:    s.IsExported,
			IsTest:        s.IsTest,
		}
	}
	edges := make([]ExtractedEdge, len(r.Edges))
	for i, e := range r.Edges {
		edges[i] = ExtractedEdge{
			FromQN:     e.FromQN,
			ToName:     e.ToName,
			Kind:       e.Kind,
			Confidence: e.Confidence,
		}
	}
	return &FileResult{Symbols: syms, Edges: edges, Module: r.Module}
}

// extractPythonAST shells out to python3 with the embedded helper script.
// Returns (result, true) on success, (nil, false) on any failure — timeout,
// non-zero exit, JSON parse error, or Python SyntaxError in the source.
// The dispatch fn falls back to extractPython on false.
func extractPythonAST(src []byte, relPath string) (*FileResult, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), pyASTTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", "-c", pythonExtractScript, relPath)
	cmd.Stdin = bytes.NewReader(src)
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}

	var resp pythonResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, false
	}
	if resp.Error != "" {
		return nil, false
	}
	return resp.toFileResult(), true
}
