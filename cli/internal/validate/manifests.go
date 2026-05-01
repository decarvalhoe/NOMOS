package validate

import "fmt"

var (
	scopeVerdicts     = []string{"in_scope", "partial", "blocked", "out_of_scope"}
	confidenceLevels  = []string{"low", "medium", "high"}
	lifecycleValues   = []string{"greenfield", "brownfield"}
	riskLevels        = []string{"low", "medium", "high", "critical"}
	surfaceTypes      = []string{"api", "ui", "worker", "data", "infra", "docs", "event", "cli", "batch"}
	severities        = []string{"low", "medium", "high", "critical"}
	reportTypes       = []string{"nomos-report.json", "coverage-report.md", "attestation", "sbom", "provenance"}
	attestationLevels = []string{"none", "basic", "signed"}
	dataSensitivity   = []string{"public", "internal", "restricted", "secret"}
	sourceTypes       = []string{"markdown", "pdf", "html", "php", "source_code", "csv", "database_export", "spreadsheet", "image", "audio", "decision", "api_export"}
	sourcePriorities  = []string{"primary", "secondary", "legacy", "derived", "reference"}
	sourceStatuses    = []string{"active", "superseded", "duplicate", "out_of_scope", "needs_review", "blocked"}
	allowedUses       = []string{"structured_contract", "vector_index", "citation_internal", "citation_external", "golden_case", "human_review_only"}
	redactionPolicies = []string{"none", "partial", "full"}
	unitTypes         = []string{"rule", "catalog_entry", "exception", "formula", "term", "workflow", "scenario", "legacy_behavior", "ambiguity", "decision"}
	criticalityLevels = []string{"low", "medium", "high", "critical"}
	unitStatuses      = []string{"covered", "partial", "missing", "not_applicable", "deprecated"}
	contractStatuses  = []string{"planned", "present", "deprecated"}

	adapterLifecycleStatuses    = []string{"experimental", "supported", "deprecated", "retired"}
	adapterCapabilityStatuses   = []string{"experimental", "stable", "deprecated"}
	adapterCapabilityCategories = []string{"detection", "extraction", "validation", "evidence", "integration"}
	adapterCapabilityIDs        = []string{
		"repo_metadata_detection",
		"language_detection",
		"surface_detection",
		"route_detection",
		"service_detection",
		"data_model_detection",
		"config_detection",
		"dependency_detection",
		"fixture_detection",
		"mock_detection",
		"hardcoded_catalog_detection",
		"forbidden_pattern_detection",
		"provenance_extraction",
		"test_surface_detection",
		"ci_detection",
		"adapter_self_check",
	}
	adapterPackageTypes = []string{"oci", "npm", "pypi", "maven", "nuget", "binary", "source"}
	adapterLanguages    = []string{
		"javascript",
		"typescript",
		"python",
		"java",
		"kotlin",
		"scala",
		"go",
		"csharp",
		"sql",
		"terraform",
		"yaml",
		"unknown",
	}
	adapterInputKinds = []string{
		"filesystem",
		"package_manifest",
		"lockfile",
		"source_file",
		"ast",
		"config_file",
		"ci_file",
		"schema_file",
		"nomos_project",
	}
	adapterOutputKinds = []string{
		"surface_inventory",
		"capability_result",
		"forbidden_pattern_finding",
		"provenance_link",
		"adapter_diagnostic",
		"command_result",
	}
	adapterEvidenceKinds = []string{
		"file_path",
		"line_range",
		"symbol",
		"route",
		"package_metadata",
		"ast_node",
		"config_key",
		"command_exit_code",
	}
	adapterRequirementTypes = []string{"tool", "runtime", "grammar", "schema", "policy", "environment"}
	adapterCommandIDs       = []string{"detect", "analyze", "verify", "self-check"}
	adapterInputTransports  = []string{"argv", "stdin-json", "file-json"}
	adapterOutputTransports = []string{"stdout-json", "file-json", "exit-code"}
	adapterCommandCWDs      = []string{"repo_root", "adapter_root"}
	adapterRequiredChecks   = []string{
		"manifest-validates",
		"capabilities-declared",
		"version-compatibility-declared",
		"commands-smoke-tested",
		"fixtures-pass",
		"limitations-declared",
	}
	adapterCompatibilityStatuses = []string{"supported", "deprecated", "blocked"}
	schemaSupportModes           = []string{"read", "write", "read_write"}
)

type projectManifest struct {
	SchemaVersion string `yaml:"schema_version"`
	Project       struct {
		ID          string  `yaml:"id"`
		Name        string  `yaml:"name"`
		Description string  `yaml:"description"`
		Repository  string  `yaml:"repository"`
		Domain      string  `yaml:"domain"`
		Lifecycle   string  `yaml:"lifecycle"`
		RiskLevel   string  `yaml:"risk_level"`
		Owners      []owner `yaml:"owners"`
	} `yaml:"project"`
	Scope struct {
		Verdict         string    `yaml:"verdict"`
		Confidence      string    `yaml:"confidence"`
		InScope         []string  `yaml:"in_scope"`
		OutOfScope      []string  `yaml:"out_of_scope"`
		Assumptions     []string  `yaml:"assumptions"`
		BoundedContexts []string  `yaml:"bounded_contexts"`
		Blockers        []blocker `yaml:"blockers"`
	} `yaml:"scope"`
	Surfaces   []surfaceDecl `yaml:"surfaces"`
	Toolchain  toolchain     `yaml:"toolchain"`
	Compliance compliance    `yaml:"compliance"`
	Evidence   evidence      `yaml:"evidence"`
	Notes      string        `yaml:"notes"`
}

type owner struct {
	Name  string `yaml:"name"`
	Role  string `yaml:"role"`
	Email string `yaml:"email"`
}

type blocker struct {
	ID          string `yaml:"id"`
	Severity    string `yaml:"severity"`
	Description string `yaml:"description"`
	Remediation string `yaml:"remediation"`
}

type surfaceDecl struct {
	Name        string   `yaml:"name"`
	Type        string   `yaml:"type"`
	Path        string   `yaml:"path"`
	Stack       string   `yaml:"stack"`
	Critical    bool     `yaml:"critical"`
	Entrypoints []string `yaml:"entrypoints"`
}

type toolchain struct {
	Build           []string `yaml:"build"`
	Test            []string `yaml:"test"`
	Lint            []string `yaml:"lint"`
	Typecheck       []string `yaml:"typecheck"`
	PackageManagers []string `yaml:"package_managers"`
	CISystems       []string `yaml:"ci_systems"`
}

type compliance struct {
	Regulated         bool     `yaml:"regulated"`
	Standards         []string `yaml:"standards"`
	DataSensitivity   string   `yaml:"data_sensitivity"`
	ExceptionsAllowed bool     `yaml:"exceptions_allowed"`
}

type evidence struct {
	RequiredReports  []string `yaml:"required_reports"`
	AttestationLevel string   `yaml:"attestation_level"`
}

type sourceManifest struct {
	SchemaVersion string   `yaml:"schema_version"`
	Sources       []source `yaml:"sources"`
}

type source struct {
	ID              string   `yaml:"id"`
	Path            string   `yaml:"path"`
	Type            string   `yaml:"type"`
	Domain          string   `yaml:"domain"`
	Priority        string   `yaml:"priority"`
	Status          string   `yaml:"status"`
	Hash            string   `yaml:"hash"`
	Version         string   `yaml:"version"`
	Owner           string   `yaml:"owner"`
	License         string   `yaml:"license"`
	Confidentiality string   `yaml:"confidentiality"`
	AllowedUses     []string `yaml:"allowed_uses"`
	RedactionPolicy string   `yaml:"redaction_policy"`
	Notes           string   `yaml:"notes"`
}

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
	CanonicalContract canonicalContract `yaml:"canonical_contract"`
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

type adapterManifest struct {
	SchemaVersion string                  `yaml:"schema_version"`
	Adapter       adapterIdentity         `yaml:"adapter"`
	Compatibility adapterCompatibility    `yaml:"compatibility"`
	StackSupport  []adapterStackSupport   `yaml:"stack_support"`
	Capabilities  adapterCapabilities     `yaml:"capabilities"`
	Commands      []adapterCommand        `yaml:"commands"`
	Limitations   []adapterLimitation     `yaml:"limitations"`
	TestContract  adapterTestContract     `yaml:"test_contract"`
	Metadata      adapterManifestMetadata `yaml:"metadata"`
}

type adapterIdentity struct {
	ID      string          `yaml:"id"`
	Name    string          `yaml:"name"`
	Version string          `yaml:"version"`
	Status  string          `yaml:"status"`
	Owners  []adapterOwner  `yaml:"owners"`
	License string          `yaml:"license"`
	Package *adapterPackage `yaml:"package"`
}

type adapterOwner struct {
	Name  string `yaml:"name"`
	Role  string `yaml:"role"`
	Email string `yaml:"email"`
}

type adapterPackage struct {
	Type     string `yaml:"type"`
	Name     string `yaml:"name"`
	Version  string `yaml:"version"`
	URL      string `yaml:"url"`
	Checksum string `yaml:"checksum"`
}

type adapterCompatibility struct {
	NomosCore        adapterNomosCoreCompatibility `yaml:"nomos_core"`
	ManifestContract struct {
		Version string `yaml:"version"`
	} `yaml:"manifest_contract"`
	Schemas    adapterSchemas              `yaml:"schemas"`
	Deprecates []adapterDeprecatedContract `yaml:"deprecates"`
}

type adapterNomosCoreCompatibility struct {
	MinVersion     string   `yaml:"min_version"`
	MaxVersion     string   `yaml:"max_version"`
	TestedVersions []string `yaml:"tested_versions"`
}

type adapterSchemas struct {
	NomosProject    *schemaVersionSupport `yaml:"nomos_project"`
	SourceManifest  *schemaVersionSupport `yaml:"source_manifest"`
	CanonicalMatrix *schemaVersionSupport `yaml:"canonical_matrix"`
	AdapterManifest *schemaVersionSupport `yaml:"adapter_manifest"`
}

type schemaVersionSupport struct {
	MinVersion string `yaml:"min_version"`
	MaxVersion string `yaml:"max_version"`
	Mode       string `yaml:"mode"`
}

type adapterDeprecatedContract struct {
	Field       string `yaml:"field"`
	Since       string `yaml:"since"`
	RemoveAfter string `yaml:"remove_after"`
	Replacement string `yaml:"replacement"`
	Reason      string `yaml:"reason"`
}

type adapterStackSupport struct {
	Language        string   `yaml:"language"`
	Runtime         string   `yaml:"runtime"`
	Frameworks      []string `yaml:"frameworks"`
	PackageManagers []string `yaml:"package_managers"`
	FileGlobs       []string `yaml:"file_globs"`
	ExcludeGlobs    []string `yaml:"exclude_globs"`
	Surfaces        []string `yaml:"surfaces"`
}

type adapterCapabilities struct {
	ContractVersion string               `yaml:"contract_version"`
	Provides        []adapterCapability  `yaml:"provides"`
	Requires        []adapterRequirement `yaml:"requires"`
}

type adapterCapability struct {
	ID         string   `yaml:"id"`
	Category   string   `yaml:"category"`
	Status     string   `yaml:"status"`
	Surfaces   []string `yaml:"surfaces"`
	Languages  []string `yaml:"languages"`
	Frameworks []string `yaml:"frameworks"`
	Inputs     []string `yaml:"inputs"`
	Outputs    []string `yaml:"outputs"`
	Evidence   []string `yaml:"evidence"`
	Confidence string   `yaml:"confidence"`
	Notes      string   `yaml:"notes"`
}

type adapterRequirement struct {
	ID       string `yaml:"id"`
	Type     string `yaml:"type"`
	Version  string `yaml:"version"`
	Optional bool   `yaml:"optional"`
	Reason   string `yaml:"reason"`
}

type adapterCommand struct {
	ID    string   `yaml:"id"`
	Argv  []string `yaml:"argv"`
	CWD   string   `yaml:"cwd"`
	Input struct {
		Transport string `yaml:"transport"`
		RepoArg   string `yaml:"repo_arg"`
		SchemaRef string `yaml:"schema_ref"`
	} `yaml:"input"`
	Output struct {
		Transport string `yaml:"transport"`
		SchemaRef string `yaml:"schema_ref"`
	} `yaml:"output"`
	TimeoutSeconds *int `yaml:"timeout_seconds"`
	Required       bool `yaml:"required"`
}

type adapterLimitation struct {
	ID          string `yaml:"id"`
	Severity    string `yaml:"severity"`
	Description string `yaml:"description"`
	Mitigation  string `yaml:"mitigation"`
}

type adapterTestContract struct {
	Fixtures            []adapterFixture           `yaml:"fixtures"`
	RequiredChecks      []string                   `yaml:"required_checks"`
	CompatibilityMatrix []adapterCompatibilityCase `yaml:"compatibility_matrix"`
}

type adapterFixture struct {
	ID                   string   `yaml:"id"`
	Path                 string   `yaml:"path"`
	Purpose              string   `yaml:"purpose"`
	ExpectedCapabilities []string `yaml:"expected_capabilities"`
}

type adapterCompatibilityCase struct {
	NomosCore string `yaml:"nomos_core"`
	Adapter   string `yaml:"adapter"`
	Status    string `yaml:"status"`
	Reason    string `yaml:"reason"`
}

type adapterManifestMetadata struct {
	Homepage      string `yaml:"homepage"`
	Repository    string `yaml:"repository"`
	Documentation string `yaml:"documentation"`
	Notes         string `yaml:"notes"`
}

func validateProject(file string, manifest projectManifest) []ValidationError {
	var errors []ValidationError

	addRequired(&errors, file, "project.id", manifest.Project.ID)
	addPattern(&errors, file, "project.id", manifest.Project.ID, lowerIDPattern)
	addRequired(&errors, file, "project.name", manifest.Project.Name)
	addRequired(&errors, file, "project.domain", manifest.Project.Domain)
	addRequired(&errors, file, "project.lifecycle", manifest.Project.Lifecycle)
	addEnum(&errors, file, "project.lifecycle", manifest.Project.Lifecycle, lifecycleValues)
	addRequired(&errors, file, "project.risk_level", manifest.Project.RiskLevel)
	addEnum(&errors, file, "project.risk_level", manifest.Project.RiskLevel, riskLevels)
	addRequiredSlice(&errors, file, "project.owners", manifest.Project.Owners)
	for i, owner := range manifest.Project.Owners {
		addRequired(&errors, file, fmt.Sprintf("project.owners[%d].name", i), owner.Name)
	}

	addEnum(&errors, file, "scope.verdict", manifest.Scope.Verdict, scopeVerdicts)
	addEnum(&errors, file, "scope.confidence", manifest.Scope.Confidence, confidenceLevels)
	addRequiredSlice(&errors, file, "scope.in_scope", manifest.Scope.InScope)
	for i, blocker := range manifest.Scope.Blockers {
		addRequired(&errors, file, fmt.Sprintf("scope.blockers[%d].id", i), blocker.ID)
		addRequired(&errors, file, fmt.Sprintf("scope.blockers[%d].severity", i), blocker.Severity)
		addEnum(&errors, file, fmt.Sprintf("scope.blockers[%d].severity", i), blocker.Severity, severities)
		addRequired(&errors, file, fmt.Sprintf("scope.blockers[%d].description", i), blocker.Description)
	}

	addRequiredSlice(&errors, file, "surfaces", manifest.Surfaces)
	for i, surface := range manifest.Surfaces {
		addRequired(&errors, file, fmt.Sprintf("surfaces[%d].name", i), surface.Name)
		addRequired(&errors, file, fmt.Sprintf("surfaces[%d].type", i), surface.Type)
		addEnum(&errors, file, fmt.Sprintf("surfaces[%d].type", i), surface.Type, surfaceTypes)
	}

	addEnum(&errors, file, "compliance.data_sensitivity", manifest.Compliance.DataSensitivity, dataSensitivity)
	for i, report := range manifest.Evidence.RequiredReports {
		addEnum(&errors, file, fmt.Sprintf("evidence.required_reports[%d]", i), report, reportTypes)
	}
	addEnum(&errors, file, "evidence.attestation_level", manifest.Evidence.AttestationLevel, attestationLevels)

	return errors
}

func validateSources(file string, manifest sourceManifest) []ValidationError {
	var errors []ValidationError

	addRequiredSlice(&errors, file, "sources", manifest.Sources)
	for i, source := range manifest.Sources {
		prefix := fmt.Sprintf("sources[%d]", i)
		addRequired(&errors, file, prefix+".id", source.ID)
		addPattern(&errors, file, prefix+".id", source.ID, upperIDPattern)
		addRequired(&errors, file, prefix+".path", source.Path)
		addRequired(&errors, file, prefix+".type", source.Type)
		addEnum(&errors, file, prefix+".type", source.Type, sourceTypes)
		addRequired(&errors, file, prefix+".domain", source.Domain)
		addRequired(&errors, file, prefix+".priority", source.Priority)
		addEnum(&errors, file, prefix+".priority", source.Priority, sourcePriorities)
		addRequired(&errors, file, prefix+".status", source.Status)
		addEnum(&errors, file, prefix+".status", source.Status, sourceStatuses)
		addRequired(&errors, file, prefix+".hash", source.Hash)
		addRequired(&errors, file, prefix+".owner", source.Owner)
		addRequired(&errors, file, prefix+".license", source.License)
		addRequired(&errors, file, prefix+".confidentiality", source.Confidentiality)
		addEnum(&errors, file, prefix+".confidentiality", source.Confidentiality, dataSensitivity)
		addRequiredSlice(&errors, file, prefix+".allowed_uses", source.AllowedUses)
		for j, allowedUse := range source.AllowedUses {
			addEnum(&errors, file, fmt.Sprintf("%s.allowed_uses[%d]", prefix, j), allowedUse, allowedUses)
		}
		addEnum(&errors, file, prefix+".redaction_policy", source.RedactionPolicy, redactionPolicies)
	}

	return errors
}

func validateMatrix(file string, manifest canonicalMatrix) []ValidationError {
	var errors []ValidationError

	addRequiredSlice(&errors, file, "units", manifest.Units)
	for i, unit := range manifest.Units {
		prefix := fmt.Sprintf("units[%d]", i)
		addRequired(&errors, file, prefix+".unit_id", unit.UnitID)
		addPattern(&errors, file, prefix+".unit_id", unit.UnitID, upperIDPattern)
		addRequired(&errors, file, prefix+".unit_type", unit.UnitType)
		addEnum(&errors, file, prefix+".unit_type", unit.UnitType, unitTypes)
		addRequired(&errors, file, prefix+".name", unit.Name)
		addRequired(&errors, file, prefix+".domain", unit.Domain)
		addRequired(&errors, file, prefix+".criticality", unit.Criticality)
		addEnum(&errors, file, prefix+".criticality", unit.Criticality, criticalityLevels)
		addRequiredSlice(&errors, file, prefix+".source_refs", unit.SourceRefs)
		for j, sourceRef := range unit.SourceRefs {
			refPrefix := fmt.Sprintf("%s.source_refs[%d]", prefix, j)
			addRequired(&errors, file, refPrefix+".source_id", sourceRef.SourceID)
			addPattern(&errors, file, refPrefix+".source_id", sourceRef.SourceID, upperIDPattern)
			addRequired(&errors, file, refPrefix+".locator", sourceRef.Locator)
		}
		addRequired(&errors, file, prefix+".business_rule", unit.BusinessRule)
		if unit.CanonicalContract.Status != "" {
			addRequired(&errors, file, prefix+".canonical_contract.path", unit.CanonicalContract.Path)
			addRequired(&errors, file, prefix+".canonical_contract.object_id", unit.CanonicalContract.ObjectID)
			addEnum(&errors, file, prefix+".canonical_contract.status", unit.CanonicalContract.Status, contractStatuses)
		}
		for j, dbRef := range unit.DBRefs {
			addRequired(&errors, file, fmt.Sprintf("%s.db_refs[%d].table", prefix, j), dbRef.Table)
		}
		for j, vectorRef := range unit.VectorRefs {
			addRequired(&errors, file, fmt.Sprintf("%s.vector_refs[%d].collection", prefix, j), vectorRef.Collection)
		}
		for j, apiRef := range unit.APIRefs {
			addRequired(&errors, file, fmt.Sprintf("%s.api_refs[%d].path", prefix, j), apiRef.Path)
		}
		for j, uiRef := range unit.UIRefs {
			addRequired(&errors, file, fmt.Sprintf("%s.ui_refs[%d].path", prefix, j), uiRef.Path)
		}
		addRequired(&errors, file, prefix+".status", unit.Status)
		addEnum(&errors, file, prefix+".status", unit.Status, unitStatuses)
	}

	return errors
}

func validateAdapter(file string, manifest adapterManifest) []ValidationError {
	var errors []ValidationError

	validateAdapterIdentity(&errors, file, manifest.Adapter)
	validateAdapterCompatibility(&errors, file, manifest.Compatibility)
	validateAdapterStackSupport(&errors, file, manifest.StackSupport)
	validateAdapterCapabilities(&errors, file, manifest.Capabilities)
	validateAdapterCommands(&errors, file, manifest.Commands)
	validateAdapterLimitations(&errors, file, manifest.Limitations)
	validateAdapterTestContract(&errors, file, manifest.TestContract)

	return errors
}

func validateAdapterIdentity(errors *[]ValidationError, file string, adapter adapterIdentity) {
	addRequired(errors, file, "adapter.id", adapter.ID)
	addPattern(errors, file, "adapter.id", adapter.ID, lowerIDPattern)
	addRequired(errors, file, "adapter.name", adapter.Name)
	addRequired(errors, file, "adapter.version", adapter.Version)
	addSemVer(errors, file, "adapter.version", adapter.Version)
	addRequired(errors, file, "adapter.status", adapter.Status)
	addEnum(errors, file, "adapter.status", adapter.Status, adapterLifecycleStatuses)
	addRequiredSlice(errors, file, "adapter.owners", adapter.Owners)
	for i, owner := range adapter.Owners {
		addRequired(errors, file, fmt.Sprintf("adapter.owners[%d].name", i), owner.Name)
	}
	if adapter.Package != nil {
		validateAdapterPackage(errors, file, "adapter.package", *adapter.Package)
	}
}

func validateAdapterPackage(errors *[]ValidationError, file string, path string, pkg adapterPackage) {
	addRequired(errors, file, path+".type", pkg.Type)
	addEnum(errors, file, path+".type", pkg.Type, adapterPackageTypes)
	addRequired(errors, file, path+".name", pkg.Name)
	addSemVer(errors, file, path+".version", pkg.Version)
	addDigest(errors, file, path+".checksum", pkg.Checksum)
}

func validateAdapterCompatibility(errors *[]ValidationError, file string, compatibility adapterCompatibility) {
	addRequired(errors, file, "compatibility.nomos_core.min_version", compatibility.NomosCore.MinVersion)
	addSemVer(errors, file, "compatibility.nomos_core.min_version", compatibility.NomosCore.MinVersion)
	addSemVer(errors, file, "compatibility.nomos_core.max_version", compatibility.NomosCore.MaxVersion)
	for i, version := range compatibility.NomosCore.TestedVersions {
		addSemVer(errors, file, fmt.Sprintf("compatibility.nomos_core.tested_versions[%d]", i), version)
	}

	addRequired(errors, file, "compatibility.manifest_contract.version", compatibility.ManifestContract.Version)
	addSemVer(errors, file, "compatibility.manifest_contract.version", compatibility.ManifestContract.Version)

	validateSchemaVersionSupport(errors, file, "compatibility.schemas.nomos_project", compatibility.Schemas.NomosProject)
	validateSchemaVersionSupport(errors, file, "compatibility.schemas.source_manifest", compatibility.Schemas.SourceManifest)
	validateSchemaVersionSupport(errors, file, "compatibility.schemas.canonical_matrix", compatibility.Schemas.CanonicalMatrix)
	validateSchemaVersionSupport(errors, file, "compatibility.schemas.adapter_manifest", compatibility.Schemas.AdapterManifest)

	for i, deprecated := range compatibility.Deprecates {
		prefix := fmt.Sprintf("compatibility.deprecates[%d]", i)
		addRequired(errors, file, prefix+".field", deprecated.Field)
		addRequired(errors, file, prefix+".since", deprecated.Since)
		addSemVer(errors, file, prefix+".since", deprecated.Since)
		addRequired(errors, file, prefix+".remove_after", deprecated.RemoveAfter)
		addSemVer(errors, file, prefix+".remove_after", deprecated.RemoveAfter)
		addRequired(errors, file, prefix+".reason", deprecated.Reason)
	}
}

func validateSchemaVersionSupport(errors *[]ValidationError, file string, path string, support *schemaVersionSupport) {
	if support == nil {
		return
	}
	addRequired(errors, file, path+".min_version", support.MinVersion)
	addSemVer(errors, file, path+".min_version", support.MinVersion)
	addSemVer(errors, file, path+".max_version", support.MaxVersion)
	addEnum(errors, file, path+".mode", support.Mode, schemaSupportModes)
}

func validateAdapterStackSupport(errors *[]ValidationError, file string, stackSupport []adapterStackSupport) {
	addRequiredSlice(errors, file, "stack_support", stackSupport)
	for i, support := range stackSupport {
		prefix := fmt.Sprintf("stack_support[%d]", i)
		addRequired(errors, file, prefix+".language", support.Language)
		addEnum(errors, file, prefix+".language", support.Language, adapterLanguages)
		addRequiredSlice(errors, file, prefix+".file_globs", support.FileGlobs)
		addRequiredSlice(errors, file, prefix+".surfaces", support.Surfaces)
		for j, surface := range support.Surfaces {
			addEnum(errors, file, fmt.Sprintf("%s.surfaces[%d]", prefix, j), surface, surfaceTypes)
		}
	}
}

func validateAdapterCapabilities(errors *[]ValidationError, file string, capabilities adapterCapabilities) {
	addSemVer(errors, file, "capabilities.contract_version", capabilities.ContractVersion)
	addRequiredSlice(errors, file, "capabilities.provides", capabilities.Provides)
	for i, capability := range capabilities.Provides {
		prefix := fmt.Sprintf("capabilities.provides[%d]", i)
		addRequired(errors, file, prefix+".id", capability.ID)
		addEnum(errors, file, prefix+".id", capability.ID, adapterCapabilityIDs)
		addRequired(errors, file, prefix+".category", capability.Category)
		addEnum(errors, file, prefix+".category", capability.Category, adapterCapabilityCategories)
		addRequired(errors, file, prefix+".status", capability.Status)
		addEnum(errors, file, prefix+".status", capability.Status, adapterCapabilityStatuses)
		addRequiredSlice(errors, file, prefix+".surfaces", capability.Surfaces)
		for j, surface := range capability.Surfaces {
			addEnum(errors, file, fmt.Sprintf("%s.surfaces[%d]", prefix, j), surface, surfaceTypes)
		}
		for j, language := range capability.Languages {
			addEnum(errors, file, fmt.Sprintf("%s.languages[%d]", prefix, j), language, adapterLanguages)
		}
		addRequiredSlice(errors, file, prefix+".inputs", capability.Inputs)
		for j, input := range capability.Inputs {
			addEnum(errors, file, fmt.Sprintf("%s.inputs[%d]", prefix, j), input, adapterInputKinds)
		}
		addRequiredSlice(errors, file, prefix+".outputs", capability.Outputs)
		for j, output := range capability.Outputs {
			addEnum(errors, file, fmt.Sprintf("%s.outputs[%d]", prefix, j), output, adapterOutputKinds)
		}
		addRequiredSlice(errors, file, prefix+".evidence", capability.Evidence)
		for j, evidence := range capability.Evidence {
			addEnum(errors, file, fmt.Sprintf("%s.evidence[%d]", prefix, j), evidence, adapterEvidenceKinds)
		}
		addEnum(errors, file, prefix+".confidence", capability.Confidence, confidenceLevels)
	}

	for i, requirement := range capabilities.Requires {
		prefix := fmt.Sprintf("capabilities.requires[%d]", i)
		addRequired(errors, file, prefix+".id", requirement.ID)
		addRequired(errors, file, prefix+".type", requirement.Type)
		addEnum(errors, file, prefix+".type", requirement.Type, adapterRequirementTypes)
		addSemVer(errors, file, prefix+".version", requirement.Version)
		addRequired(errors, file, prefix+".reason", requirement.Reason)
	}
}

func validateAdapterCommands(errors *[]ValidationError, file string, commands []adapterCommand) {
	for i, command := range commands {
		prefix := fmt.Sprintf("commands[%d]", i)
		addRequired(errors, file, prefix+".id", command.ID)
		addEnum(errors, file, prefix+".id", command.ID, adapterCommandIDs)
		addRequiredSlice(errors, file, prefix+".argv", command.Argv)
		addEnum(errors, file, prefix+".cwd", command.CWD, adapterCommandCWDs)
		addRequired(errors, file, prefix+".input.transport", command.Input.Transport)
		addEnum(errors, file, prefix+".input.transport", command.Input.Transport, adapterInputTransports)
		addRequired(errors, file, prefix+".output.transport", command.Output.Transport)
		addEnum(errors, file, prefix+".output.transport", command.Output.Transport, adapterOutputTransports)
		addIntRange(errors, file, prefix+".timeout_seconds", command.TimeoutSeconds, 1, 3600)
	}
}

func validateAdapterLimitations(errors *[]ValidationError, file string, limitations []adapterLimitation) {
	for i, limitation := range limitations {
		prefix := fmt.Sprintf("limitations[%d]", i)
		addRequired(errors, file, prefix+".id", limitation.ID)
		addPattern(errors, file, prefix+".id", limitation.ID, lowerIDPattern)
		addRequired(errors, file, prefix+".severity", limitation.Severity)
		addEnum(errors, file, prefix+".severity", limitation.Severity, severities)
		addRequired(errors, file, prefix+".description", limitation.Description)
	}
}

func validateAdapterTestContract(errors *[]ValidationError, file string, contract adapterTestContract) {
	for i, fixture := range contract.Fixtures {
		prefix := fmt.Sprintf("test_contract.fixtures[%d]", i)
		addRequired(errors, file, prefix+".id", fixture.ID)
		addPattern(errors, file, prefix+".id", fixture.ID, lowerIDPattern)
		addRequired(errors, file, prefix+".path", fixture.Path)
		addRequired(errors, file, prefix+".purpose", fixture.Purpose)
		for j, capabilityID := range fixture.ExpectedCapabilities {
			addEnum(errors, file, fmt.Sprintf("%s.expected_capabilities[%d]", prefix, j), capabilityID, adapterCapabilityIDs)
		}
	}

	addRequiredSlice(errors, file, "test_contract.required_checks", contract.RequiredChecks)
	for i, check := range contract.RequiredChecks {
		addEnum(errors, file, fmt.Sprintf("test_contract.required_checks[%d]", i), check, adapterRequiredChecks)
	}

	for i, compatibilityCase := range contract.CompatibilityMatrix {
		prefix := fmt.Sprintf("test_contract.compatibility_matrix[%d]", i)
		addRequired(errors, file, prefix+".nomos_core", compatibilityCase.NomosCore)
		addSemVer(errors, file, prefix+".nomos_core", compatibilityCase.NomosCore)
		addRequired(errors, file, prefix+".adapter", compatibilityCase.Adapter)
		addSemVer(errors, file, prefix+".adapter", compatibilityCase.Adapter)
		addRequired(errors, file, prefix+".status", compatibilityCase.Status)
		addEnum(errors, file, prefix+".status", compatibilityCase.Status, adapterCompatibilityStatuses)
	}
}
