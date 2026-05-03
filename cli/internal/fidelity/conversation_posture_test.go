package fidelity

import (
	"testing"
)

func TestEvaluatePostureCiteHighConfidence(t *testing.T) {
	ctx := ChunkContext{
		ChunkID: "CHUNK-1", GovernanceStatus: "active",
		Confidence: "high", Domain: "insurance",
		InScope: true, HasSource: true,
	}
	d := EvaluatePosture(ctx, DefaultPostureContract())

	if d.Action != ActionCite {
		t.Fatalf("expected cite, got %s", d.Action)
	}
	if !d.MustCite {
		t.Fatal("expected must_cite=true")
	}
	if d.RuleID != "POSTURE-CITE-HIGH-CONFIDENCE" {
		t.Fatalf("expected POSTURE-CITE-HIGH-CONFIDENCE, got %s", d.RuleID)
	}
}

func TestEvaluatePostureParaphraseMedium(t *testing.T) {
	ctx := ChunkContext{
		ChunkID: "CHUNK-2", GovernanceStatus: "active",
		Confidence: "medium", Domain: "insurance",
		InScope: true, HasSource: true,
	}
	d := EvaluatePosture(ctx, DefaultPostureContract())

	if d.Action != ActionParaphrase {
		t.Fatalf("expected paraphrase, got %s", d.Action)
	}
	if !d.MustCite {
		t.Fatal("expected must_cite=true for paraphrase")
	}
}

func TestEvaluatePostureRefuseOutOfScope(t *testing.T) {
	ctx := ChunkContext{
		ChunkID: "CHUNK-3", GovernanceStatus: "active",
		Confidence: "high", InScope: false, HasSource: true,
	}
	d := EvaluatePosture(ctx, DefaultPostureContract())

	if d.Action != ActionRefuse {
		t.Fatalf("expected refuse for out-of-scope, got %s", d.Action)
	}
	if d.MustCite {
		t.Fatal("refuse should not require citation")
	}
}

func TestEvaluatePostureRefuseRepealed(t *testing.T) {
	ctx := ChunkContext{
		ChunkID: "CHUNK-4", GovernanceStatus: "repealed",
		Confidence: "high", InScope: true, HasSource: true,
		IsRepealed: true,
	}
	d := EvaluatePosture(ctx, DefaultPostureContract())

	if d.Action != ActionRefuse {
		t.Fatalf("expected refuse for repealed, got %s", d.Action)
	}
}

func TestEvaluatePostureDisclaimStale(t *testing.T) {
	ctx := ChunkContext{
		ChunkID: "CHUNK-5", GovernanceStatus: "amended",
		Confidence: "high", InScope: true, HasSource: true,
		IsStale: true,
	}
	d := EvaluatePosture(ctx, DefaultPostureContract())

	if d.Action != ActionDisclaim {
		t.Fatalf("expected disclaim for stale, got %s", d.Action)
	}
	if !d.MustCite {
		t.Fatal("disclaim should require citation")
	}
	if len(d.Caveats) == 0 {
		t.Fatal("expected caveats for stale chunk")
	}
}

func TestEvaluatePostureDisclaimLowConfidence(t *testing.T) {
	ctx := ChunkContext{
		ChunkID: "CHUNK-6", GovernanceStatus: "active",
		Confidence: "low", InScope: true, HasSource: true,
	}
	d := EvaluatePosture(ctx, DefaultPostureContract())

	if d.Action != ActionDisclaim {
		t.Fatalf("expected disclaim for low confidence, got %s", d.Action)
	}
}

func TestEvaluatePostureDeferNoSource(t *testing.T) {
	ctx := ChunkContext{
		ChunkID: "CHUNK-7", GovernanceStatus: "active",
		Confidence: "high", InScope: true, HasSource: false,
	}
	d := EvaluatePosture(ctx, DefaultPostureContract())

	if d.Action != ActionDefer {
		t.Fatalf("expected defer for no source, got %s", d.Action)
	}
}

func TestEvaluatePosturePriorityOrder(t *testing.T) {
	// Out-of-scope (priority 0) should win over stale (priority 2).
	ctx := ChunkContext{
		ChunkID: "CHUNK-8", Confidence: "high",
		InScope: false, IsStale: true, HasSource: true,
	}
	d := EvaluatePosture(ctx, DefaultPostureContract())

	if d.Action != ActionRefuse {
		t.Fatalf("expected refuse (higher priority) over disclaim, got %s", d.Action)
	}
}

func TestEvaluatePostureFallback(t *testing.T) {
	// Context that matches no rule.
	ctx := ChunkContext{
		ChunkID: "CHUNK-9", GovernanceStatus: "active",
		Confidence: "unknown", InScope: true, HasSource: true,
	}
	d := EvaluatePosture(ctx, DefaultPostureContract())

	if d.Action != ActionDisclaim {
		t.Fatalf("expected fallback disclaim, got %s", d.Action)
	}
	if d.RuleID != "POSTURE-FALLBACK" {
		t.Fatalf("expected fallback rule, got %s", d.RuleID)
	}
}

func TestValidateContractValid(t *testing.T) {
	errs := ValidateContract(DefaultPostureContract())
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidateContractEmpty(t *testing.T) {
	errs := ValidateContract(PostureContract{})
	if len(errs) < 2 {
		t.Fatalf("expected at least 2 errors for empty contract, got %v", errs)
	}
}

func TestValidateContractDuplicateRuleID(t *testing.T) {
	c := PostureContract{
		Domain: "test",
		Rules: []PostureRule{
			{ID: "RULE-1", Action: ActionCite, Condition: "x"},
			{ID: "RULE-1", Action: ActionRefuse, Condition: "y"},
		},
	}
	errs := ValidateContract(c)
	found := false
	for _, e := range errs {
		if len(e) > 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected duplicate ID error")
	}
}

func TestValidateContractInvalidAction(t *testing.T) {
	c := PostureContract{
		Domain: "test",
		Rules: []PostureRule{
			{ID: "RULE-1", Action: "invalid_action", Condition: "x"},
		},
	}
	errs := ValidateContract(c)
	if len(errs) == 0 {
		t.Fatal("expected error for invalid action")
	}
}

func TestAllActions(t *testing.T) {
	actions := AllActions()
	if len(actions) != 6 {
		t.Fatalf("expected 6 actions, got %d", len(actions))
	}
	for _, a := range actions {
		if !a.IsValid() {
			t.Fatalf("expected %s to be valid", a)
		}
	}
}

func TestPostureActionIsValid(t *testing.T) {
	if !ActionCite.IsValid() {
		t.Fatal("cite should be valid")
	}
	if PostureAction("invent").IsValid() {
		t.Fatal("invent should not be valid")
	}
}

func TestDefaultContractHasAllPriorityCases(t *testing.T) {
	c := DefaultPostureContract()
	actions := map[PostureAction]bool{}
	for _, r := range c.Rules {
		actions[r.Action] = true
	}
	// Should cover cite, paraphrase, refuse, disclaim, defer.
	expected := []PostureAction{ActionCite, ActionParaphrase, ActionRefuse, ActionDisclaim, ActionDefer}
	for _, a := range expected {
		if !actions[a] {
			t.Fatalf("default contract missing action %s", a)
		}
	}
}
