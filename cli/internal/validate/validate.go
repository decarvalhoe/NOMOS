package validate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

type Result struct {
	Valid bool         `json:"valid"`
	Files []FileResult `json:"files"`
}

type FileResult struct {
	Path         string            `json:"path"`
	ManifestType string            `json:"manifest_type,omitempty"`
	Valid        bool              `json:"valid"`
	Errors       []ValidationError `json:"errors,omitempty"`
}

type ValidationError struct {
	File    string `json:"file"`
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type options struct {
	format string
	files  []string
}

var (
	lowerIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	upperIDPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]*$`)
	semVerPattern  = regexp.MustCompile(`^[0-9]+[.][0-9]+[.][0-9]+(-[0-9A-Za-z.-]+)?([+][0-9A-Za-z.-]+)?$`)
	digestPattern  = regexp.MustCompile(`^(sha256|sha384|sha512):[A-Fa-f0-9]+$`)
)

func Command(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		printUsage(stdout)
		return 0
	}

	opts, err := parseOptions(args)
	if err != nil {
		fmt.Fprintf(stderr, "validate: %s\n\n", err)
		printUsage(stderr)
		return 2
	}

	result := ValidateFiles(opts.files)
	writeResult(result, opts.format, stdout)
	if result.Valid {
		return 0
	}
	return 1
}

func ValidateFiles(paths []string) Result {
	result := Result{
		Valid: true,
		Files: make([]FileResult, 0, len(paths)),
	}

	for _, path := range paths {
		fileResult := ValidateFile(path)
		if !fileResult.Valid {
			result.Valid = false
		}
		result.Files = append(result.Files, fileResult)
	}

	return result
}

func ValidateFile(path string) FileResult {
	data, err := os.ReadFile(path)
	if err != nil {
		return invalidFile(path, "", ValidationError{
			File:    path,
			Path:    "",
			Code:    "read_error",
			Message: err.Error(),
		})
	}

	return ValidateBytes(path, data)
}

func ValidateBytes(path string, data []byte) FileResult {
	var root yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&root); err != nil {
		return invalidFile(path, "", ValidationError{
			File:    path,
			Path:    "",
			Code:    "invalid_yaml",
			Message: err.Error(),
		})
	}

	manifestType := detectManifestType(&root)
	if manifestType == "" {
		return invalidFile(path, "", ValidationError{
			File:    path,
			Path:    "",
			Code:    "unknown_manifest",
			Message: "manifest must contain project/scope/surfaces (or project/scope/mode for a canonical corpus), sources, units, or adapter/compatibility/stack_support/capabilities/test_contract",
		})
	}

	switch manifestType {
	case "nomos-project":
		var manifest projectManifest
		if err := decodeKnown(data, &manifest); err != nil {
			return invalidFile(path, manifestType, decodeError(path, err))
		}
		return resultFor(path, manifestType, validateProject(path, manifest))
	case "source-manifest":
		var manifest sourceManifest
		if err := decodeKnown(data, &manifest); err != nil {
			return invalidFile(path, manifestType, decodeError(path, err))
		}
		return resultFor(path, manifestType, validateSources(path, manifest))
	case "canonical-matrix":
		var manifest canonicalMatrix
		if err := decodeKnown(data, &manifest); err != nil {
			return invalidFile(path, manifestType, decodeError(path, err))
		}
		return resultFor(path, manifestType, validateMatrix(path, manifest))
	case "adapter-manifest":
		var manifest adapterManifest
		if err := decodeKnown(data, &manifest); err != nil {
			return invalidFile(path, manifestType, decodeError(path, err))
		}
		return resultFor(path, manifestType, validateAdapter(path, manifest))
	default:
		panic("unreachable manifest type")
	}
}

func parseOptions(args []string) (options, error) {
	opts := options{format: "text"}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--format":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--format requires a value")
			}
			i++
			opts.format = args[i]
		case strings.HasPrefix(arg, "--format="):
			opts.format = strings.TrimPrefix(arg, "--format=")
		case arg == "--help" || arg == "-h":
			return opts, fmt.Errorf("usage requested")
		case strings.HasPrefix(arg, "-"):
			return opts, fmt.Errorf("unknown option %q", arg)
		default:
			opts.files = append(opts.files, arg)
		}
	}

	if opts.format != "text" && opts.format != "json" {
		return opts, fmt.Errorf("unsupported format %q", opts.format)
	}
	if len(opts.files) == 0 {
		return opts, fmt.Errorf("at least one manifest path is required")
	}

	return opts, nil
}

func printUsage(writer io.Writer) {
	fmt.Fprintln(writer, "Usage:")
	fmt.Fprintln(writer, "  nomos validate [--format text|json] <manifest.yaml> [...]")
}

func writeResult(result Result, format string, writer io.Writer) {
	if format == "json" {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(result)
		return
	}

	for _, file := range result.Files {
		if file.Valid {
			fmt.Fprintf(writer, "ok: %s (%s)\n", file.Path, file.ManifestType)
			continue
		}
		fmt.Fprintf(writer, "invalid: %s", file.Path)
		if file.ManifestType != "" {
			fmt.Fprintf(writer, " (%s)", file.ManifestType)
		}
		fmt.Fprintln(writer)
		for _, validationErr := range file.Errors {
			fmt.Fprintf(
				writer,
				"  - [%s] %s: %s\n",
				validationErr.Code,
				emptyAsRoot(validationErr.Path),
				validationErr.Message,
			)
		}
	}
}

func decodeKnown(data []byte, out any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	return decoder.Decode(out)
}

func decodeError(file string, err error) ValidationError {
	return ValidationError{
		File:    file,
		Path:    "",
		Code:    "decode_error",
		Message: err.Error(),
	}
}

func resultFor(path string, manifestType string, errors []ValidationError) FileResult {
	return FileResult{
		Path:         path,
		ManifestType: manifestType,
		Valid:        len(errors) == 0,
		Errors:       errors,
	}
}

func invalidFile(path string, manifestType string, err ValidationError) FileResult {
	return FileResult{
		Path:         path,
		ManifestType: manifestType,
		Valid:        false,
		Errors:       []ValidationError{err},
	}
}

func detectManifestType(root *yaml.Node) string {
	mapping := documentMapping(root)
	if mapping == nil {
		return ""
	}
	if hasKeys(mapping, "project", "scope", "surfaces") || hasKeys(mapping, "project", "scope", "mode") {
		return "nomos-project"
	}
	if hasKeys(mapping, "sources") {
		return "source-manifest"
	}
	if hasKeys(mapping, "units") {
		return "canonical-matrix"
	}
	if hasKeys(mapping, "adapter", "compatibility", "stack_support", "capabilities", "test_contract") {
		return "adapter-manifest"
	}
	return ""
}

func documentMapping(root *yaml.Node) *yaml.Node {
	if root == nil {
		return nil
	}
	if root.Kind == yaml.DocumentNode && len(root.Content) == 1 {
		return documentMapping(root.Content[0])
	}
	if root.Kind != yaml.MappingNode {
		return nil
	}
	return root
}

func hasKeys(node *yaml.Node, keys ...string) bool {
	for _, key := range keys {
		if !hasKey(node, key) {
			return false
		}
	}
	return true
}

func hasKey(node *yaml.Node, key string) bool {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return true
		}
	}
	return false
}

func addRequired(errors *[]ValidationError, file string, path string, value string) {
	if strings.TrimSpace(value) == "" {
		*errors = append(*errors, validationError(file, path, "required", "value is required"))
	}
}

func addRequiredSlice[T any](errors *[]ValidationError, file string, path string, values []T) {
	if len(values) == 0 {
		*errors = append(*errors, validationError(file, path, "required", "at least one item is required"))
	}
}

func addEnum(errors *[]ValidationError, file string, path string, value string, allowed []string) {
	if value == "" {
		return
	}
	if !slices.Contains(allowed, value) {
		*errors = append(*errors, validationError(
			file,
			path,
			"enum",
			fmt.Sprintf("must be one of: %s", strings.Join(allowed, ", ")),
		))
	}
}

func addPattern(errors *[]ValidationError, file string, path string, value string, pattern *regexp.Regexp) {
	if value == "" {
		return
	}
	if !pattern.MatchString(value) {
		*errors = append(*errors, validationError(
			file,
			path,
			"pattern",
			fmt.Sprintf("must match %s", pattern.String()),
		))
	}
}

func addSemVer(errors *[]ValidationError, file string, path string, value string) {
	addPattern(errors, file, path, value, semVerPattern)
}

func addDigest(errors *[]ValidationError, file string, path string, value string) {
	addPattern(errors, file, path, value, digestPattern)
}

func addIntRange(errors *[]ValidationError, file string, path string, value *int, min int, max int) {
	if value == nil {
		return
	}
	if *value < min || *value > max {
		*errors = append(*errors, validationError(
			file,
			path,
			"range",
			fmt.Sprintf("must be between %d and %d", min, max),
		))
	}
}

func validationError(file string, path string, code string, message string) ValidationError {
	return ValidationError{
		File:    file,
		Path:    path,
		Code:    code,
		Message: message,
	}
}

func emptyAsRoot(path string) string {
	if path == "" {
		return "<root>"
	}
	return path
}
