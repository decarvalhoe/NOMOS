package ruleexec

// VRC-42 (#578) — the substrate is borrowed, and NOMOS computes nothing.
//
// Doctrine §2.3: the proof is the failure. The load-bearing tests are the ones
// that withhold a substrate, or give an incoherent one, and prove no value
// appears anyway. That is what keeps anti-goal §10.3 — "no home-made rule
// engine" — a property rather than an intention.

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeSubstrate struct {
	resp Response
	err  error
	seen Request
	// raw is what the substrate "wrote". Empty means: marshal resp, so a test
	// that does not care about raw bytes still produces a coherent record.
	raw []byte
}

func (f *fakeSubstrate) Run(req Request) (Response, []byte, error) {
	f.seen = req
	if f.err != nil {
		return Response{}, nil, f.err
	}
	raw := f.raw
	if raw == nil {
		raw, _ = json.Marshal(f.resp)
	}
	return f.resp, raw, nil
}

func formulas() []Formula {
	return []Formula{
		{
			AtomID:     "A-1",
			Expression: "base + supplement",
			Trace:      SourceTrace{CanonicalRef: "art-7/code-5", File: "doc.md", StartLine: 5, EndLine: 7},
		},
		{
			AtomID:     "A-2",
			Expression: "si le taux dépasse cinq pour cent",
			Trace:      SourceTrace{CanonicalRef: "art-8/code-13", File: "doc.md", StartLine: 13, EndLine: 15},
		},
	}
}

func goodResponse() Response {
	return Response{
		SchemaVersion: ResponseSchema,
		Substrate:     "borrowed-engine/1.2",
		Results: []Result{
			{AtomID: "A-1", Status: StatusComputed, Value: json.RawMessage(`105`), Unit: "scalar"},
			{AtomID: "A-2", Status: StatusUnsupported, Reason: "not an arithmetic shape"},
		},
	}
}

func run(t *testing.T, resp Response, err error) (ExecutionRecord, error) {
	t.Helper()
	sub := &fakeSubstrate{resp: resp, err: err}
	return Execute(sub, []string{"borrowed"}, formulas())
}

// --- the anti-goal, made testable ---------------------------------------

func TestExecute_WithoutASubstrateNothingIsComputed(t *testing.T) {
	// The central property: NOMOS does not stand in for a rule engine. No
	// default, no zero, no "unknown but probably".
	record, err := Execute(nil, []string{"none"}, formulas())
	if err == nil {
		t.Fatal("execution without a substrate returned a record")
	}
	if !strings.Contains(err.Error(), "NOMOS computes nothing itself") {
		t.Fatalf("the refusal should say why: %v", err)
	}
	if len(record.Results) != 0 || record.ComputedCount != 0 {
		t.Fatalf("a record was produced anyway: %+v", record)
	}
}

func TestExecute_NomosNeverFillsInAValueTheSubstrateWithheld(t *testing.T) {
	// Every formula comes back unsupported. NOMOS must record exactly that.
	resp := Response{
		SchemaVersion: ResponseSchema,
		Substrate:     "borrowed-engine/1.2",
		Results: []Result{
			{AtomID: "A-1", Status: StatusUnsupported, Reason: "out of scope"},
			{AtomID: "A-2", Status: StatusUnsupported, Reason: "out of scope"},
		},
	}
	record, err := run(t, resp, nil)
	if err != nil {
		t.Fatalf("all-unsupported is a legitimate answer: %v", err)
	}
	if record.ComputedCount != 0 || record.UnsupportedNum != 2 {
		t.Fatalf("counts wrong: %+v", record)
	}
	for _, r := range record.Results {
		if len(r.Value) != 0 {
			t.Fatalf("a value appeared where the substrate gave none: %+v", r)
		}
	}
}

func TestExecute_SubstrateFailureProducesNoPartialRecord(t *testing.T) {
	// ADVERSARIAL: a crashed substrate must not leave usable-looking results.
	record, err := run(t, Response{}, errors.New("boom"))
	if err == nil {
		t.Fatal("a failing substrate produced a record")
	}
	if !strings.Contains(err.Error(), FindingSubstrateFailed) {
		t.Fatalf("the failure code should be named: %v", err)
	}
	if len(record.Results) != 0 {
		t.Fatalf("partial record leaked: %+v", record)
	}
}

// --- the exchange contract ----------------------------------------------

func TestExecute_HappyPathCarriesTheSourceTrace(t *testing.T) {
	record, err := run(t, goodResponse(), nil)
	if err != nil {
		t.Fatalf("unexpected failure: %v", err)
	}
	if record.Substrate != "borrowed-engine/1.2" {
		t.Fatalf("the record must name who computed it: %q", record.Substrate)
	}
	if record.ComputedCount != 1 || record.UnsupportedNum != 1 {
		t.Fatalf("counts wrong: %+v", record)
	}
	if !strings.HasPrefix(record.RequestDigest, "sha256:") {
		t.Fatalf("the request digest is missing: %q", record.RequestDigest)
	}
	for _, r := range record.Results {
		if r.Trace.CanonicalRef == "" || r.Trace.File == "" || r.Trace.StartLine == 0 {
			t.Fatalf("a value without its source trace is just a number: %+v", r)
		}
		if r.Expression == "" {
			t.Fatalf("the formula must travel with its result: %+v", r)
		}
	}
	if err := VerifyRecord(record, formulas()); err != nil {
		t.Fatalf("the record the engine just built does not verify: %v", err)
	}
}

func TestExecute_ExpressionIsSentVerbatim(t *testing.T) {
	sub := &fakeSubstrate{resp: goodResponse()}
	if _, err := Execute(sub, []string{"borrowed"}, formulas()); err != nil {
		t.Fatal(err)
	}
	if sub.seen.SchemaVersion != RequestSchema {
		t.Fatalf("request schema is %q", sub.seen.SchemaVersion)
	}
	if sub.seen.Formulas[1].Expression != "si le taux dépasse cinq pour cent" {
		t.Fatalf("NOMOS rewrote the formula: %q", sub.seen.Formulas[1].Expression)
	}
}

func TestExecute_RefusesAnOffContractResponse(t *testing.T) {
	cases := map[string]func(*Response){
		"wrong schema version": func(r *Response) { r.SchemaVersion = "nomos-rule-substrate-response-v9" },
		"substrate unnamed":    func(r *Response) { r.Substrate = "  " },
		"answer nobody asked for": func(r *Response) {
			r.Results = append(r.Results, Result{AtomID: "A-99", Status: StatusComputed, Value: json.RawMessage(`1`)})
		},
		"answered twice": func(r *Response) {
			r.Results = append(r.Results, Result{AtomID: "A-1", Status: StatusComputed, Value: json.RawMessage(`7`)})
		},

		"computed with no value": func(r *Response) { r.Results[0].Value = nil },
		"unsupported, no reason": func(r *Response) { r.Results[1].Reason = "" },
		"unsupported with value": func(r *Response) { r.Results[1].Value = json.RawMessage(`3`) },
		"unknown status":         func(r *Response) { r.Results[0].Status = "maybe" },
		"empty status":           func(r *Response) { r.Results[0].Status = "" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			resp := goodResponse()
			mutate(&resp)
			record, err := run(t, resp, nil)
			if err == nil {
				t.Fatalf("an off-contract response was accepted: %+v", record)
			}
			if !strings.Contains(err.Error(), FindingSubstrateFailed) {
				t.Fatalf("the failure code should be named: %v", err)
			}
			if len(record.Results) != 0 {
				t.Fatalf("a record leaked: %+v", record)
			}
		})
	}
}

func TestExecute_MissingAnswerIsDiagnosedAsSuch(t *testing.T) {
	// A missing answer would fail anyway — a zero-value result has no status —
	// but "unknown status \"\"" tells a reader far less than naming the atom the
	// substrate skipped. The explicit check earns its place through the message,
	// so the test asserts the message.
	resp := goodResponse()
	resp.Results = resp.Results[:1]
	record, err := run(t, resp, nil)
	if err == nil {
		t.Fatalf("a missing answer was accepted: %+v", record)
	}
	if !strings.Contains(err.Error(), "returned no answer for") {
		t.Fatalf("the diagnosis should name the skipped atom, got: %v", err)
	}
	if !strings.Contains(err.Error(), "A-2") {
		t.Fatalf("the diagnosis should name which atom: %v", err)
	}
	if len(record.Results) != 0 {
		t.Fatalf("a record leaked: %+v", record)
	}
}

func TestExecute_RefusesIncoherentInput(t *testing.T) {
	sub := &fakeSubstrate{resp: goodResponse()}
	if _, err := Execute(sub, []string{"borrowed"}, nil); err == nil {
		t.Fatal("an empty formula list was accepted")
	}
	dup := []Formula{{AtomID: "A-1", Expression: "x"}, {AtomID: "A-1", Expression: "y"}}
	if _, err := Execute(sub, []string{"borrowed"}, dup); err == nil {
		t.Fatal("duplicate atom ids were accepted")
	}
	noID := []Formula{{AtomID: " ", Expression: "x"}}
	if _, err := Execute(sub, []string{"borrowed"}, noID); err == nil {
		t.Fatal("a formula with no atom id was accepted")
	}
}

func TestExecute_IsDeterministicUnderResponseOrder(t *testing.T) {
	resp := goodResponse()
	first, err := run(t, resp, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Results[0], resp.Results[1] = resp.Results[1], resp.Results[0]
	second, err := run(t, resp, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Results) != len(second.Results) {
		t.Fatal("result count varies with response order")
	}
	for i := range first.Results {
		if first.Results[i].AtomID != second.Results[i].AtomID {
			t.Fatalf("result order varies with response order: %v vs %v",
				first.Results[i].AtomID, second.Results[i].AtomID)
		}
	}
}

// --- the record must survive re-checking --------------------------------

func TestVerifyRecord_RefusesATamperedRecord(t *testing.T) {
	base, err := run(t, goodResponse(), nil)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*ExecutionRecord){
		"expression rewritten": func(r *ExecutionRecord) { r.Results[0].Expression = "something else" },
		"trace moved":          func(r *ExecutionRecord) { r.Results[0].Trace.StartLine = 999 },
		"substrate erased":     func(r *ExecutionRecord) { r.Substrate = "" },
		"counts inflated":      func(r *ExecutionRecord) { r.ComputedCount = 5 },
		"schema changed":       func(r *ExecutionRecord) { r.SchemaVersion = "v9" },
		"reason dropped": func(r *ExecutionRecord) {
			for i := range r.Results {
				if r.Results[i].Status == StatusUnsupported {
					r.Results[i].Reason = ""
				}
			}
		},
		"answers an unknown atom": func(r *ExecutionRecord) { r.Results[0].AtomID = "A-99" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			forged := base
			forged.Results = append([]TracedResult{}, base.Results...)
			mutate(&forged)
			if err := VerifyRecord(forged, formulas()); err == nil {
				t.Fatal("a tampered record verified")
			}
		})
	}
}

// --- the process boundary ------------------------------------------------

func TestExternal_EmptyCommandIsAFailureNotADefault(t *testing.T) {
	if _, _, err := (External{}).Run(Request{SchemaVersion: RequestSchema}); err == nil {
		t.Fatal("an empty command was accepted")
	}
}

func TestExternal_NonZeroExitIsAFailure(t *testing.T) {
	sub := External{Command: []string{"false"}, Timeout: 10 * time.Second}
	if _, _, err := sub.Run(Request{SchemaVersion: RequestSchema}); err == nil {
		t.Fatal("a failing process was accepted")
	}
}

func TestExternal_NonJSONOutputIsAFailure(t *testing.T) {
	sub := External{Command: []string{"echo", "not json"}, Timeout: 10 * time.Second}
	if _, _, err := sub.Run(Request{SchemaVersion: RequestSchema}); err == nil {
		t.Fatal("non-JSON output was accepted")
	}
}

// --- #642: record integrity, byte by byte -------------------------------
//
// The digests exist so that a record edited after emission stops agreeing with
// itself. Each case below flips one thing and proves VerifyRecord notices.

func TestVerifyRecord_RecomputesTheRequestDigest(t *testing.T) {
	record, err := run(t, goodResponse(), nil)
	if err != nil {
		t.Fatal(err)
	}
	// The record verifies against the formulas it was built from…
	if err := VerifyRecord(record, formulas()); err != nil {
		t.Fatalf("fresh record does not verify: %v", err)
	}
	// …and stops verifying when the question changes by one byte.
	edited := formulas()
	edited[0].Expression = "base + supplements"
	if err := VerifyRecord(record, edited); err == nil {
		t.Fatal("a record verified against a formula it never answered")
	}
	// A hand-written digest is refused too.
	forged := record
	forged.RequestDigest = "sha256:" + strings.Repeat("0", 64)
	if err := VerifyRecord(forged, formulas()); err == nil {
		t.Fatal("a forged request_digest verified")
	}
	if err := VerifyRecord(ExecutionRecord{
		SchemaVersion: RecordSchema, Substrate: "x", Results: record.Results,
		ComputedCount: record.ComputedCount, UnsupportedNum: record.UnsupportedNum,
	}, formulas()); err == nil {
		t.Fatal("a record with no request_digest verified")
	}
}

func TestVerifyRecord_ADigestlessRecordIsRefused(t *testing.T) {
	record, err := run(t, goodResponse(), nil)
	if err != nil {
		t.Fatal(err)
	}
	// The expected diagnosis matters as much as the refusal: an absent digest
	// and a malformed one are different defects, and the message says which.
	for name, tc := range map[string]struct {
		strip func(*ExecutionRecord)
		want  string
	}{
		"no results_digest": {
			func(r *ExecutionRecord) { r.ResultsDigest = "" },
			"carries no results_digest",
		},
		"no response_digest": {
			func(r *ExecutionRecord) { r.ResponseDigest = "" },
			"the substrate's own output is unbound",
		},
		"malformed response_digest": {
			func(r *ExecutionRecord) { r.ResponseDigest = "md5:whatever" },
			"is not a sha256 digest",
		},
	} {
		t.Run(name, func(t *testing.T) {
			forged := record
			tc.strip(&forged)
			err := VerifyRecord(forged, formulas())
			if err == nil {
				t.Fatal("an unbound record verified")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected the diagnosis to say %q, got: %v", tc.want, err)
			}
		})
	}
}

func TestVerifyRecord_OneEditedByteInAnyRecordedFieldIsCaught(t *testing.T) {
	// The requirement of #642, field by field. Each mutation leaves the record
	// internally plausible — the expression still matches the formula, the
	// status is still legal — and is caught only because the digest is
	// recomputed.
	base, err := run(t, goodResponse(), nil)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*ExecutionRecord){
		"value edited": func(r *ExecutionRecord) {
			for i := range r.Results {
				if r.Results[i].Status == StatusComputed {
					r.Results[i].Value = json.RawMessage(`106`)
				}
			}
		},
		"unit edited": func(r *ExecutionRecord) { r.Results[0].Unit = "CHF" },
		"reason edited": func(r *ExecutionRecord) {
			for i := range r.Results {
				if r.Results[i].Status == StatusUnsupported {
					r.Results[i].Reason = "a different reason"
				}
			}
		},
		"status swapped": func(r *ExecutionRecord) {
			// computed -> unsupported, kept legal by supplying a reason and
			// dropping the value: only the digest can catch this one.
			for i := range r.Results {
				if r.Results[i].Status == StatusComputed {
					r.Results[i].Status = StatusUnsupported
					r.Results[i].Reason = "reclassified after the fact"
					r.Results[i].Value = nil
					r.ComputedCount--
					r.UnsupportedNum++
				}
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			forged := base
			forged.Results = append([]TracedResult{}, base.Results...)
			mutate(&forged)
			if err := VerifyRecord(forged, formulas()); err == nil {
				t.Fatalf("%s survived verification", name)
			}
		})
	}
}

func TestComputeResultsDigest_IsOrderIndependentAndValueSensitive(t *testing.T) {
	base, err := run(t, goodResponse(), nil)
	if err != nil {
		t.Fatal(err)
	}
	forward, err := ComputeResultsDigest(base.Results)
	if err != nil {
		t.Fatal(err)
	}
	reversed := append([]TracedResult{}, base.Results...)
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	shuffled, err := ComputeResultsDigest(reversed)
	if err != nil {
		t.Fatal(err)
	}
	if forward != shuffled {
		t.Fatal("the digest depends on result order; it must not")
	}

	changed := append([]TracedResult{}, base.Results...)
	changed[0].Value = json.RawMessage(`999`)
	altered, err := ComputeResultsDigest(changed)
	if err != nil {
		t.Fatal(err)
	}
	if altered == forward {
		t.Fatal("the digest ignored a changed value")
	}
}

func TestVerifyResponseBytes_BindsTheSubstratesOwnOutput(t *testing.T) {
	// The one check the record cannot make alone: against retained raw output.
	raw := []byte(`{"schema_version":"nomos-rule-substrate-response-v1","substrate":"borrowed-engine/1.2","results":[]}`)
	sub := &fakeSubstrate{resp: goodResponse(), raw: raw}
	record, err := Execute(sub, []string{"borrowed"}, formulas())
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyResponseBytes(record, raw); err != nil {
		t.Fatalf("the retained output should match: %v", err)
	}
	if err := VerifyResponseBytes(record, append(raw, ' ')); err != nil {
		t.Fatalf("trailing whitespace should be tolerated: %v", err)
	}
	tampered := []byte(strings.Replace(string(raw), "1.2", "9.9", 1))
	if err := VerifyResponseBytes(record, tampered); err == nil {
		t.Fatal("tampered raw output verified against the record")
	}
	if err := VerifyResponseBytes(ExecutionRecord{}, raw); err == nil {
		t.Fatal("a record with no response_digest verified")
	}
}

func TestExecute_RecordCarriesAllThreeDigests(t *testing.T) {
	record, err := run(t, goodResponse(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, got := range map[string]string{
		"request_digest":  record.RequestDigest,
		"results_digest":  record.ResultsDigest,
		"response_digest": record.ResponseDigest,
	} {
		if !strings.HasPrefix(got, "sha256:") || len(got) != len("sha256:")+64 {
			t.Fatalf("%s is not a sha256 digest: %q", name, got)
		}
	}
	if record.RequestDigest == record.ResultsDigest {
		t.Fatal("the request and results digests collide; they cover different things")
	}
}
