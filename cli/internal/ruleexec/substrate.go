// Package ruleexec runs computable atoms through an EXTERNAL substrate.
//
// VRC-42 (#578, doc 45 §3 B3) — "substrat d'exécution emprunté, jamais
// construit". Anti-goal §10.3 of the plan is explicit: NOMOS does not build a
// rule engine. What was missing was not an engine; it was the boundary through
// which a borrowed one can be reached without NOMOS becoming one.
//
// So this package computes nothing. It defines a versioned JSON protocol,
// spawns a process, hands it the formulas verbatim, and validates what comes
// back. Every value in an execution record was produced by the substrate; if no
// substrate answers, the record has no values, and that is a refusal rather
// than a fallback.
//
// Two properties follow, both load-bearing:
//
//  1. NOTHING IS COMPUTED HERE. There is no evaluator, no arithmetic, no
//     expression parser in this package. With no substrate configured, nothing
//     is produced at all — never a default, never a zero, never "unknown but
//     probably". A test pins the absence.
//  2. THE BOUNDARY IS A PROCESS. The substrate is reached by exec, over stdin
//     and stdout. Nothing is imported, linked, or vendored. That is what keeps
//     a copyleft substrate at arm's length: the licence register
//     (docs/regulated/ip-governance/license-register.yaml) sets the policy —
//     `process_api_boundary` — names the candidate engines, and this package is
//     its mechanism. This file names no substrate at all, and a wiring-matrix
//     must_be_absent probe keeps it that way: the register names them, the
//     engine does not.
package ruleexec

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// Protocol versions. Any change to the exchange shape MUST bump these: the
// engine refuses a response in another version rather than guessing.
const (
	RequestSchema  = "nomos-rule-substrate-request-v1"
	ResponseSchema = "nomos-rule-substrate-response-v1"
	RecordSchema   = "nomos-rule-execution-record-v1"
)

// Result statuses. `unsupported` is a first-class answer: a substrate that
// cannot evaluate a formula must say so, and saying so is not a failure.
const (
	StatusComputed    = "computed"
	StatusUnsupported = "unsupported"
)

// FindingSubstrateFailed is the single failure code. Any substrate problem —
// absent, crashed, timed out, off-contract, incoherent — lands here, and the
// execution produces no values.
const FindingSubstrateFailed = "RULE_SUBSTRATE_FAILED"

// DefaultTimeout bounds one substrate invocation.
const DefaultTimeout = 2 * time.Minute

const claimBoundary = "Values in this record were computed by the named external substrate, " +
	"not by NOMOS. NOMOS carried the formula and its source trace, and validated the " +
	"exchange. It asserts nothing about the substrate's correctness."

// SourceTrace ties a computed value back to the text it came from. Without it a
// number is just a number.
type SourceTrace struct {
	CanonicalRef string `json:"canonical_ref,omitempty"`
	File         string `json:"file,omitempty"`
	StartLine    int    `json:"start_line,omitempty"`
	EndLine      int    `json:"end_line,omitempty"`
}

// Formula is one computable atom handed to the substrate. Expression is the
// atom's text VERBATIM: NOMOS does not rewrite, normalise or interpret it.
type Formula struct {
	AtomID     string            `json:"atom_id"`
	Expression string            `json:"expression"`
	Parameters map[string]string `json:"parameters,omitempty"`
	Trace      SourceTrace       `json:"source_trace"`
}

// Request is what the substrate reads on stdin.
type Request struct {
	SchemaVersion string    `json:"schema_version"`
	Formulas      []Formula `json:"formulas"`
}

// Response is what the substrate writes on stdout. Substrate identifies the
// engine that ran, so a record names who computed it.
type Response struct {
	SchemaVersion string   `json:"schema_version"`
	Substrate     string   `json:"substrate"`
	Results       []Result `json:"results"`
}

// Result is one formula's outcome as the substrate reports it.
type Result struct {
	AtomID string          `json:"atom_id"`
	Status string          `json:"status"`
	Value  json.RawMessage `json:"value,omitempty"`
	Unit   string          `json:"unit,omitempty"`
	Reason string          `json:"reason,omitempty"`
}

// TracedResult is what NOMOS records: the substrate's answer, welded to the
// formula and the source it came from.
type TracedResult struct {
	AtomID     string          `json:"atom_id"`
	Expression string          `json:"expression"`
	Trace      SourceTrace     `json:"source_trace"`
	Status     string          `json:"status"`
	Value      json.RawMessage `json:"value,omitempty"`
	Unit       string          `json:"unit,omitempty"`
	Reason     string          `json:"reason,omitempty"`
}

// ExecutionRecord is the emitted evidence.
type ExecutionRecord struct {
	SchemaVersion  string         `json:"schema_version"`
	ClaimBoundary  string         `json:"claim_boundary"`
	Substrate      string         `json:"substrate"`
	SubstrateCmd   string         `json:"substrate_cmd"`
	RequestDigest  string         `json:"request_digest"`
	FormulaCount   int            `json:"formula_count"`
	ComputedCount  int            `json:"computed_count"`
	UnsupportedNum int            `json:"unsupported_count"`
	Results        []TracedResult `json:"results"`
}

// Substrate is the borrowed engine. The interface exists so tests can drive the
// protocol without a process; the only shipped implementation is the external
// one below.
type Substrate interface {
	Run(req Request) (Response, error)
}

// External runs a command per batch: request JSON on stdin, response JSON on
// stdout. A non-zero exit, a timeout, or silence is a failure — never a result.
type External struct {
	Command []string
	Timeout time.Duration
	Env     []string
}

// Run implements Substrate.
func (e External) Run(req Request) (Response, error) {
	if len(e.Command) == 0 || strings.TrimSpace(e.Command[0]) == "" {
		return Response{}, errors.New("rule substrate: empty command")
	}
	name := e.Command[0]

	payload, err := json.Marshal(req)
	if err != nil {
		return Response{}, fmt.Errorf("rule substrate %q: encode request: %w", name, err)
	}

	timeout := e.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, e.Command[1:]...)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), e.Env...)

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return Response{}, fmt.Errorf("rule substrate %q timed out after %s", name, timeout)
		}
		return Response{}, fmt.Errorf("rule substrate %q failed: %v%s", name, err, stderrTail(stderr.String()))
	}

	var resp Response
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &resp); err != nil {
		return Response{}, fmt.Errorf("rule substrate %q: response is not %s JSON: %v%s",
			name, ResponseSchema, err, stderrTail(stderr.String()))
	}
	return resp, nil
}

func stderrTail(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if len(s) > 300 {
		s = "…" + s[len(s)-300:]
	}
	return " — stderr: " + s
}

// Execute runs the formulas through the substrate and returns the record.
//
// Fail-closed throughout. Any problem returns an error and NO record: a partial
// record would invite a reader to use the results that did come back, and a
// substrate that answered incoherently has not earned that trust.
func Execute(sub Substrate, cmd []string, formulas []Formula) (ExecutionRecord, error) {
	if sub == nil {
		return ExecutionRecord{}, fmt.Errorf("%s: no substrate configured — NOMOS computes nothing itself",
			FindingSubstrateFailed)
	}
	if len(formulas) == 0 {
		return ExecutionRecord{}, fmt.Errorf("%s: no formula atom to execute", FindingSubstrateFailed)
	}

	seen := map[string]bool{}
	for _, f := range formulas {
		if strings.TrimSpace(f.AtomID) == "" {
			return ExecutionRecord{}, fmt.Errorf("%s: a formula has no atom id", FindingSubstrateFailed)
		}
		if seen[f.AtomID] {
			return ExecutionRecord{}, fmt.Errorf("%s: duplicate formula atom id %q", FindingSubstrateFailed, f.AtomID)
		}
		seen[f.AtomID] = true
	}

	req := Request{SchemaVersion: RequestSchema, Formulas: formulas}
	payload, err := json.Marshal(req)
	if err != nil {
		return ExecutionRecord{}, fmt.Errorf("%s: encode request: %v", FindingSubstrateFailed, err)
	}
	digest := sha256.Sum256(payload)

	resp, err := sub.Run(req)
	if err != nil {
		return ExecutionRecord{}, fmt.Errorf("%s: %v", FindingSubstrateFailed, err)
	}
	if resp.SchemaVersion != ResponseSchema {
		return ExecutionRecord{}, fmt.Errorf("%s: response schema_version is %q, expected %q",
			FindingSubstrateFailed, resp.SchemaVersion, ResponseSchema)
	}
	if strings.TrimSpace(resp.Substrate) == "" {
		return ExecutionRecord{}, fmt.Errorf("%s: response does not name the substrate that ran it",
			FindingSubstrateFailed)
	}

	byAtom := make(map[string]Result, len(resp.Results))
	for _, r := range resp.Results {
		if _, dup := byAtom[r.AtomID]; dup {
			return ExecutionRecord{}, fmt.Errorf("%s: substrate answered twice for %q",
				FindingSubstrateFailed, r.AtomID)
		}
		if !seen[r.AtomID] {
			return ExecutionRecord{}, fmt.Errorf("%s: substrate answered for %q, which was never asked",
				FindingSubstrateFailed, r.AtomID)
		}
		byAtom[r.AtomID] = r
	}

	record := ExecutionRecord{
		SchemaVersion: RecordSchema,
		ClaimBoundary: claimBoundary,
		Substrate:     resp.Substrate,
		SubstrateCmd:  strings.Join(cmd, " "),
		RequestDigest: "sha256:" + hex.EncodeToString(digest[:]),
		FormulaCount:  len(formulas),
		Results:       make([]TracedResult, 0, len(formulas)),
	}

	for _, f := range formulas {
		r, ok := byAtom[f.AtomID]
		if !ok {
			return ExecutionRecord{}, fmt.Errorf("%s: substrate returned no answer for %q",
				FindingSubstrateFailed, f.AtomID)
		}
		switch r.Status {
		case StatusComputed:
			if len(bytes.TrimSpace(r.Value)) == 0 {
				return ExecutionRecord{}, fmt.Errorf("%s: %q is computed but carries no value",
					FindingSubstrateFailed, f.AtomID)
			}
			record.ComputedCount++
		case StatusUnsupported:
			if strings.TrimSpace(r.Reason) == "" {
				return ExecutionRecord{}, fmt.Errorf("%s: %q is unsupported with no reason",
					FindingSubstrateFailed, f.AtomID)
			}
			if len(bytes.TrimSpace(r.Value)) != 0 {
				return ExecutionRecord{}, fmt.Errorf("%s: %q is unsupported yet carries a value",
					FindingSubstrateFailed, f.AtomID)
			}
			record.UnsupportedNum++
		default:
			return ExecutionRecord{}, fmt.Errorf("%s: %q has unknown status %q",
				FindingSubstrateFailed, f.AtomID, r.Status)
		}

		record.Results = append(record.Results, TracedResult{
			AtomID:     f.AtomID,
			Expression: f.Expression,
			Trace:      f.Trace,
			Status:     r.Status,
			Value:      r.Value,
			Unit:       r.Unit,
			Reason:     r.Reason,
		})
	}

	sort.SliceStable(record.Results, func(i, j int) bool {
		return record.Results[i].AtomID < record.Results[j].AtomID
	})
	return record, nil
}

// VerifyRecord re-checks an execution record against the formulas it claims to
// answer. It is the adversarial counterpart of Execute: a record whose values
// drifted from the formulas, or that answers atoms nobody asked about, does not
// survive.
func VerifyRecord(record ExecutionRecord, formulas []Formula) error {
	if record.SchemaVersion != RecordSchema {
		return fmt.Errorf("record schema_version is %q, expected %q", record.SchemaVersion, RecordSchema)
	}
	if strings.TrimSpace(record.Substrate) == "" {
		return errors.New("record does not name the substrate that produced it")
	}
	byAtom := make(map[string]Formula, len(formulas))
	for _, f := range formulas {
		byAtom[f.AtomID] = f
	}
	if len(record.Results) != len(formulas) {
		return fmt.Errorf("record holds %d result(s) for %d formula(s)", len(record.Results), len(formulas))
	}
	computed, unsupported := 0, 0
	for _, r := range record.Results {
		f, ok := byAtom[r.AtomID]
		if !ok {
			return fmt.Errorf("record answers %q, which is not among the formulas", r.AtomID)
		}
		if r.Expression != f.Expression {
			return fmt.Errorf("record's expression for %q is not the formula that was sent", r.AtomID)
		}
		if r.Trace != f.Trace {
			return fmt.Errorf("record's source trace for %q does not match the atom", r.AtomID)
		}
		switch r.Status {
		case StatusComputed:
			computed++
		case StatusUnsupported:
			if strings.TrimSpace(r.Reason) == "" {
				return fmt.Errorf("%q is unsupported with no reason", r.AtomID)
			}
			unsupported++
		default:
			return fmt.Errorf("%q has unknown status %q", r.AtomID, r.Status)
		}
	}
	if computed != record.ComputedCount || unsupported != record.UnsupportedNum {
		return fmt.Errorf("record counts (%d computed, %d unsupported) disagree with its results (%d, %d)",
			record.ComputedCount, record.UnsupportedNum, computed, unsupported)
	}
	return nil
}
