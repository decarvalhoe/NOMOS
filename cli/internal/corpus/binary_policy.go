package corpus

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Action defines what the policy dictates for a given file.
type Action string

const (
	ActionAllow           Action = "allow"
	ActionSkip            Action = "skip"
	ActionExtractMetadata Action = "extract-metadata-only"
	ActionBlock           Action = "block"
)

// FileClass categorises a file by its format family.
type FileClass string

const (
	ClassText   FileClass = "text"
	ClassPDF    FileClass = "pdf"
	ClassOffice FileClass = "office"
	ClassImage  FileClass = "image"
	ClassBinary FileClass = "binary"
)

// PolicyResult captures the classification and policy decision for one file.
type PolicyResult struct {
	Path   string    `json:"path"`
	Class  FileClass `json:"class"`
	Action Action    `json:"action"`
	Reason string    `json:"reason"`
}

// Policy configures the behaviour for each file class.
type BinaryPolicy struct {
	PDF    Action `json:"pdf"`
	Office Action `json:"office"`
	Image  Action `json:"image"`
	Binary Action `json:"binary"`
}

// DefaultPolicy returns a sensible default:
//   - PDF    → extract-metadata-only
//   - Office → extract-metadata-only
//   - Image  → skip
//   - Binary → block
func DefaultBinaryPolicy() BinaryPolicy {
	return BinaryPolicy{
		PDF:    ActionExtractMetadata,
		Office: ActionExtractMetadata,
		Image:  ActionSkip,
		Binary: ActionBlock,
	}
}

// Classify determines the file class from extension and content sniffing.
func Classify(path string) (FileClass, error) {
	ext := strings.ToLower(filepath.Ext(path))

	// Extension-based fast path.
	if class, ok := classifyByExt(ext); ok {
		return class, nil
	}

	// Content sniffing for ambiguous or unknown extensions.
	f, err := os.Open(path)
	if err != nil {
		return ClassBinary, err
	}
	defer f.Close()

	return sniffContent(f)
}

// Apply returns the policy decision for a file class.
func (p BinaryPolicy) Apply(class FileClass) (Action, string) {
	switch class {
	case ClassText:
		return ActionAllow, "text files are always allowed"
	case ClassPDF:
		return p.PDF, "PDF file — policy: " + string(p.PDF)
	case ClassOffice:
		return p.Office, "Office document — policy: " + string(p.Office)
	case ClassImage:
		return p.Image, "image file — policy: " + string(p.Image)
	case ClassBinary:
		return p.Binary, "binary file — policy: " + string(p.Binary)
	default:
		return ActionBlock, "unknown file class"
	}
}

// EvaluateFile classifies a file and applies the policy in one call.
func (p BinaryPolicy) EvaluateFile(path string) (PolicyResult, error) {
	class, err := Classify(path)
	if err != nil {
		return PolicyResult{
			Path:   path,
			Class:  ClassBinary,
			Action: p.Binary,
			Reason: "classification error, treated as binary: " + err.Error(),
		}, err
	}

	action, reason := p.Apply(class)
	return PolicyResult{
		Path:   path,
		Class:  class,
		Action: action,
		Reason: reason,
	}, nil
}

// EvaluateDir walks a directory and evaluates every regular file.
func (p BinaryPolicy) EvaluateDir(root string) ([]PolicyResult, error) {
	var results []PolicyResult
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		result, _ := p.EvaluateFile(path)
		result.Path = filepath.ToSlash(rel)
		results = append(results, result)
		return nil
	})
	return results, err
}

// --- extension classification ---

var extToClass = map[string]FileClass{
	// Text
	".txt": ClassText, ".md": ClassText, ".mdx": ClassText,
	".yaml": ClassText, ".yml": ClassText, ".json": ClassText,
	".toml": ClassText, ".csv": ClassText, ".xml": ClassText,
	".html": ClassText, ".htm": ClassText, ".css": ClassText,
	".js": ClassText, ".ts": ClassText, ".tsx": ClassText, ".jsx": ClassText,
	".go": ClassText, ".py": ClassText, ".rb": ClassText, ".rs": ClassText,
	".java": ClassText, ".kt": ClassText, ".cs": ClassText, ".php": ClassText,
	".sql": ClassText, ".sh": ClassText, ".bash": ClassText, ".zsh": ClassText,
	".cue": ClassText, ".proto": ClassText, ".graphql": ClassText,
	".env": ClassText, ".cfg": ClassText, ".ini": ClassText,
	".rst": ClassText, ".adoc": ClassText, ".tex": ClassText,
	".swift": ClassText, ".scala": ClassText, ".r": ClassText,
	".tf": ClassText, ".hcl": ClassText,

	// PDF
	".pdf": ClassPDF,

	// Office
	".docx": ClassOffice, ".doc": ClassOffice,
	".xlsx": ClassOffice, ".xls": ClassOffice,
	".pptx": ClassOffice, ".ppt": ClassOffice,
	".odt": ClassOffice, ".ods": ClassOffice, ".odp": ClassOffice,
	".rtf": ClassOffice,

	// Image
	".png": ClassImage, ".jpg": ClassImage, ".jpeg": ClassImage,
	".gif": ClassImage, ".svg": ClassImage, ".bmp": ClassImage,
	".ico": ClassImage, ".webp": ClassImage, ".tiff": ClassImage,
	".tif": ClassImage,
}

func classifyByExt(ext string) (FileClass, bool) {
	class, ok := extToClass[ext]
	return class, ok
}

// --- content sniffing ---

// sniffContent reads the first 512 bytes and checks for binary content.
func sniffContent(r io.Reader) (FileClass, error) {
	buf := make([]byte, 512)
	n, err := r.Read(buf)
	if err != nil && err != io.EOF {
		return ClassBinary, err
	}
	buf = buf[:n]

	if len(buf) == 0 {
		return ClassText, nil
	}

	// PDF magic: %PDF
	if len(buf) >= 4 && string(buf[:4]) == "%PDF" {
		return ClassPDF, nil
	}

	// ZIP magic (covers .docx/.xlsx/.pptx which are ZIP-based).
	if len(buf) >= 4 && buf[0] == 0x50 && buf[1] == 0x4B && buf[2] == 0x03 && buf[3] == 0x04 {
		return ClassOffice, nil
	}

	// PNG magic
	if len(buf) >= 8 && buf[0] == 0x89 && string(buf[1:4]) == "PNG" {
		return ClassImage, nil
	}

	// JPEG magic
	if len(buf) >= 2 && buf[0] == 0xFF && buf[1] == 0xD8 {
		return ClassImage, nil
	}

	// GIF magic
	if len(buf) >= 4 && (string(buf[:4]) == "GIF8") {
		return ClassImage, nil
	}

	// Check for binary content: high ratio of non-text bytes.
	nonText := 0
	for _, b := range buf {
		if b == 0 {
			return ClassBinary, nil
		}
		if b < 0x20 && b != '\n' && b != '\r' && b != '\t' && b != 0x1B {
			nonText++
		}
	}
	if float64(nonText)/float64(len(buf)) > 0.10 {
		return ClassBinary, nil
	}

	return ClassText, nil
}
