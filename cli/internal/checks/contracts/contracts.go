package contracts

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	upperIDPattern   = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]*$`)
	unitTypes        = []string{"rule", "catalog_entry", "exception", "formula", "term", "workflow", "scenario", "legacy_behavior", "ambiguity", "decision"}
	criticalityLevels = []string{"low", "medium", "high", "critical"}
	contractStatuses = []string{"planned", "present", "deprecated"}
	unitStatuses     = []string{"covered", "partial", "missing", "not_applicable", "deprecated"}
)

type canonicalMatrix struct {
	SchemaVersion string `yaml:"schema_version"`
	Units         []unit `yaml:"units"`
}

type unit struct {
	UnitID            string            `yaml:"unit_id"`
	UnitType          string            `yaml:"unit_type"`
	Name              string            `yaml:"name"`
	Domain            string            `yaml:"domain"`
	Criticality       string            `yaml:"criticality"`
	SourceRefs        []sourceRef       `yaml:"source_refs"`
	BusinessRule      string            `yaml:"business_rule"`
	CanonicalContract *canonicalContract `yaml:"canonical_contract"`
	SchemaRefs        []string          `yaml:"schema_refs"`
	DBRefs            []dbRef           `yaml:"db_refs"`
	VectorRefs        []vectorRef       `yaml:"vector_refs"`
	CoreRefs          []codeRef         `yaml:"core_refs"`
	APIRefs           []apiRef          `yaml:"api_refs"`
	UIRefs            []uiRef           `yaml:"ui_refs"`
	TestRefs          []string          `yaml:"test_refs"`
	DecisionRefs      []string          `yaml:"decision_refs"`
	Gaps              []string          `yaml:"gaps"`
	Status            string            `yaml:"status"`
}

type sourceRef struct {
	SourceID string `yaml:"source_id"`
	Locator  string `yaml:"locator"`
	Hash     string `yaml:"hash"`
}

type canonicalContract struct {
	Path     string `yaml:"path"`
	ObjectID string `yaml:"object_id"`
	Status   string `yaml:"status"`
}

type dbRef struct {
	Table string `yaml:"table"`
	Key   string `yaml:"key"`
}

type vectorRef struct {
	Collection string `yaml:"collection"`
	Filter     string `yaml:"filter"`
}

type codeRef struct {
	Package string `yaml:"package"`
	Module  string `yaml:"module"`
	Symbol  string `yaml:"symbol"`
}

type apiRef struct {
	Method string `yaml:"method"`
	Path   string `yaml:"path"`
}

type uiRef struct {
	App  string `yaml:"app"`
	Path string `yaml:"path"`
}

type CheckResult struct {
	Valid bool        `json:"valid"`
	Units []UnitCheck `json:"units"`
}

type UnitCheck struct {
	UnitID string       `json:"unit_id"`
	Valid  bool         `json:"valid"`
	Errors []CheckError `json:"errors,omitempty"`
}

type CheckError struct {
	UnitID  string `json:"unit_id"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func CheckContracts(manifestPath string, baseDir string) (CheckResult, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return CheckResult{}, fmt.Errorf("reading manifest: %w", err)
	}
	return CheckContractsFromBytes(data, baseDir)
}

func CheckContractsFromBytes(data []byte, baseDir string) (CheckResult, error) {
	var matrix canonicalMatrix
	if err := yaml.Unmarshal(data, &matrix); err != nil {
		return CheckResult{}, fmt.Errorf("parsing manifest: %w", err)
	}

	result := CheckResult{Valid: true}
	for _, u := range matrix.Units {
		uc := checkUnit(u, baseDir)
		if !uc.Valid {
			result.Valid = false
		}
		result.Units = append(result.Units, uc)
	}
	return result, nil
}

func checkUnit(u unit, baseDir string) UnitCheck {
	uc := UnitCheck{
		UnitID: u.UnitID,
		Valid:  true,
	}

	checkUnitID(&uc, u)
	checkUnitType(&uc, u)
	checkUnitName(&uc, u)
	checkDomain(&uc, u)
	checkCriticality(&uc, u)
	checkBusinessRule(&uc, u)
	checkSourceRefs(&uc, u)
	checkCanonicalContract(&uc, u, baseDir)
	checkUnitStatus(&uc, u)

	uc.Valid = len(uc.Errors) == 0
	return uc
}

func checkUnitID(uc *UnitCheck, u unit) {
	if strings.TrimSpace(u.UnitID) == "" {
		addError(uc, u.UnitID, "MISSING_UNIT_ID", "unit_id is required")
		return
	}
	if !upperIDPattern.MatchString(u.UnitID) {
		addError(uc, u.UnitID, "INVALID_UNIT_ID",
			fmt.Sprintf("unit_id %q must match %s", u.UnitID, upperIDPattern.String()))
	}
}

func checkUnitType(uc *UnitCheck, u unit) {
	if strings.TrimSpace(u.UnitType) == "" {
		addError(uc, u.UnitID, "MISSING_UNIT_TYPE", "unit_type is required")
		return
	}
	if !slices.Contains(unitTypes, u.UnitType) {
		addError(uc, u.UnitID, "INVALID_UNIT_TYPE",
			fmt.Sprintf("unit_type %q must be one of: %s", u.UnitType, strings.Join(unitTypes, ", ")))
	}
}

func checkUnitName(uc *UnitCheck, u unit) {
	if strings.TrimSpace(u.Name) == "" {
		addError(uc, u.UnitID, "MISSING_NAME", "name is required")
	}
}

func checkDomain(uc *UnitCheck, u unit) {
	if strings.TrimSpace(u.Domain) == "" {
		addError(uc, u.UnitID, "MISSING_DOMAIN", "domain is required")
	}
}

func checkCriticality(uc *UnitCheck, u unit) {
	if strings.TrimSpace(u.Criticality) == "" {
		addError(uc, u.UnitID, "MISSING_CRITICALITY", "criticality is required")
		return
	}
	if !slices.Contains(criticalityLevels, u.Criticality) {
		addError(uc, u.UnitID, "INVALID_CRITICALITY",
			fmt.Sprintf("criticality %q must be one of: %s", u.Criticality, strings.Join(criticalityLevels, ", ")))
	}
}

func checkBusinessRule(uc *UnitCheck, u unit) {
	if strings.TrimSpace(u.BusinessRule) == "" {
		addError(uc, u.UnitID, "MISSING_BUSINESS_RULE", "business_rule is required")
	}
}

func checkSourceRefs(uc *UnitCheck, u unit) {
	if len(u.SourceRefs) == 0 {
		addError(uc, u.UnitID, "NO_SOURCE_REFS", "at least one source_ref is required")
		return
	}
	for i, ref := range u.SourceRefs {
		if strings.TrimSpace(ref.SourceID) == "" {
			addError(uc, u.UnitID, "MISSING_SOURCE_REF_ID",
				fmt.Sprintf("source_refs[%d].source_id is required", i))
		}
		if strings.TrimSpace(ref.Locator) == "" {
			addError(uc, u.UnitID, "MISSING_SOURCE_REF_LOCATOR",
				fmt.Sprintf("source_refs[%d].locator is required", i))
		}
	}
}

func checkCanonicalContract(uc *UnitCheck, u unit, baseDir string) {
	cc := u.CanonicalContract
	if cc == nil {
		addError(uc, u.UnitID, "NO_CONTRACT", "canonical_contract is required")
		return
	}
	if strings.TrimSpace(cc.Path) == "" {
		addError(uc, u.UnitID, "MISSING_CONTRACT_PATH", "canonical_contract.path is required")
	} else if baseDir != "" {
		resolved := filepath.Join(baseDir, cc.Path)
		if _, err := os.Stat(resolved); err != nil {
			addError(uc, u.UnitID, "CONTRACT_FILE_NOT_FOUND",
				fmt.Sprintf("canonical_contract file not found: %s", resolved))
		}
	}
	if strings.TrimSpace(cc.ObjectID) == "" {
		addError(uc, u.UnitID, "MISSING_CONTRACT_OBJECT_ID", "canonical_contract.object_id is required")
	}
	if strings.TrimSpace(cc.Status) == "" {
		addError(uc, u.UnitID, "MISSING_CONTRACT_STATUS", "canonical_contract.status is required")
	} else if !slices.Contains(contractStatuses, cc.Status) {
		addError(uc, u.UnitID, "INVALID_CONTRACT_STATUS",
			fmt.Sprintf("canonical_contract.status %q must be one of: %s", cc.Status, strings.Join(contractStatuses, ", ")))
	}
}

func checkUnitStatus(uc *UnitCheck, u unit) {
	if strings.TrimSpace(u.Status) == "" {
		addError(uc, u.UnitID, "MISSING_STATUS", "status is required")
		return
	}
	if !slices.Contains(unitStatuses, u.Status) {
		addError(uc, u.UnitID, "INVALID_STATUS",
			fmt.Sprintf("status %q must be one of: %s", u.Status, strings.Join(unitStatuses, ", ")))
	}
}

func addError(uc *UnitCheck, unitID string, code string, message string) {
	uc.Errors = append(uc.Errors, CheckError{
		UnitID:  unitID,
		Code:    code,
		Message: message,
	})
}
