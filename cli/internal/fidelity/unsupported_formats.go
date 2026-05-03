package fidelity

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FormatFinding describes a single unsupported-format issue.
type FormatFinding struct {
	Path     string `json:"path"`
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Blocking bool   `json:"blocking"`
}

// Finding codes.
const (
	CodeUnsupportedFormat = "UNSUPPORTED_FORMAT"
	CodeBinaryDetected    = "BINARY_DETECTED"
	CodeImageNoAlt        = "IMAGE_NO_ALT"
	CodeProprietaryFormat = "PROPRIETARY_FORMAT"
	CodeEmptyFile         = "EMPTY_FILE"
)

// FormatScanResult holds the outcome of scanning a directory for format issues.
type FormatScanResult struct {
	ScannedFiles  int             `json:"scanned_files"`
	SupportedFiles int            `json:"supported_files"`
	UnsupportedFiles int          `json:"unsupported_files"`
	Findings      []FormatFinding `json:"findings,omitempty"`
	Pass          bool            `json:"pass"`
}

// proprietary extensions that are not openly parseable.
var proprietaryExts = map[string]string{
	".doc":  "Microsoft Word (legacy)",
	".xls":  "Microsoft Excel (legacy)",
	".ppt":  "Microsoft PowerPoint (legacy)",
	".pages": "Apple Pages",
	".numbers": "Apple Numbers",
	".key":  "Apple Keynote",
	".mdb":  "Microsoft Access",
	".accdb": "Microsoft Access",
	".vsd":  "Microsoft Visio",
	".vsdx": "Microsoft Visio",
}

// image extensions where alt-text checking applies.
var imageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".bmp": true, ".webp": true, ".tiff": true, ".tif": true,
	".ico": true, ".svg": true,
}

// ScanFormats walks a directory and reports unsupported, binary,
// proprietary, and image-without-alt findings.
func ScanFormats(root string, registry *Registry) (FormatScanResult, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return FormatScanResult{}, err
	}

	result := FormatScanResult{Pass: true}

	err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			base := filepath.Base(path)
			if shouldSkipDir(base) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}

		rel, _ := filepath.Rel(absRoot, path)
		rel = filepath.ToSlash(rel)
		result.ScannedFiles++

		ext := strings.ToLower(filepath.Ext(path))

		// Check proprietary formats.
		if desc, ok := proprietaryExts[ext]; ok {
			result.Findings = append(result.Findings, FormatFinding{
				Path:     rel,
				Code:     CodeProprietaryFormat,
				Severity: "high",
				Message:  fmt.Sprintf("proprietary format: %s (%s)", ext, desc),
				Blocking: true,
			})
			result.UnsupportedFiles++
			result.Pass = false
			return nil
		}

		// Check images.
		if imageExts[ext] {
			result.UnsupportedFiles++
			result.Findings = append(result.Findings, FormatFinding{
				Path:     rel,
				Code:     CodeImageNoAlt,
				Severity: "medium",
				Message:  fmt.Sprintf("image file %s has no verifiable alt-text in isolation", ext),
				Blocking: false,
			})
			return nil
		}

		// Check if registry has an adapter.
		if _, ok := registry.ForFile(path); ok {
			result.SupportedFiles++
			// Check for empty file.
			info, err := d.Info()
			if err == nil && info.Size() == 0 {
				result.Findings = append(result.Findings, FormatFinding{
					Path:     rel,
					Code:     CodeEmptyFile,
					Severity: "low",
					Message:  "file is empty",
					Blocking: false,
				})
			}
			return nil
		}

		// Check if it's binary.
		if isBinary(path) {
			result.Findings = append(result.Findings, FormatFinding{
				Path:     rel,
				Code:     CodeBinaryDetected,
				Severity: "high",
				Message:  "binary file detected with no registered adapter",
				Blocking: true,
			})
			result.UnsupportedFiles++
			result.Pass = false
			return nil
		}

		// Unknown but text-like format — unsupported warning.
		result.Findings = append(result.Findings, FormatFinding{
			Path:     rel,
			Code:     CodeUnsupportedFormat,
			Severity: "medium",
			Message:  fmt.Sprintf("no adapter registered for extension %q", ext),
			Blocking: false,
		})
		result.UnsupportedFiles++
		return nil
	})

	sort.Slice(result.Findings, func(i, j int) bool {
		return result.Findings[i].Path < result.Findings[j].Path
	})

	return result, err
}

// CheckFile checks a single file for format support.
func CheckFile(path string, registry *Registry) FormatFinding {
	ext := strings.ToLower(filepath.Ext(path))

	if desc, ok := proprietaryExts[ext]; ok {
		return FormatFinding{
			Path: path, Code: CodeProprietaryFormat, Severity: "high",
			Message: fmt.Sprintf("proprietary format: %s (%s)", ext, desc), Blocking: true,
		}
	}
	if imageExts[ext] {
		return FormatFinding{
			Path: path, Code: CodeImageNoAlt, Severity: "medium",
			Message: fmt.Sprintf("image %s — no alt-text verifiable in isolation", ext),
		}
	}
	if _, ok := registry.ForFile(path); ok {
		return FormatFinding{Path: path, Code: "", Message: "supported"}
	}
	if isBinary(path) {
		return FormatFinding{
			Path: path, Code: CodeBinaryDetected, Severity: "high",
			Message: "binary file with no adapter", Blocking: true,
		}
	}
	return FormatFinding{
		Path: path, Code: CodeUnsupportedFormat, Severity: "medium",
		Message: fmt.Sprintf("no adapter for %q", ext),
	}
}

func isBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return false
	}
	for _, b := range buf[:n] {
		if b == 0 {
			return true
		}
	}
	return false
}

func shouldSkipDir(name string) bool {
	switch strings.ToLower(name) {
	case ".git", "node_modules", "__pycache__", ".venv", "vendor", ".tools":
		return true
	}
	return false
}
