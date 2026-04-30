package detect

const ReportFormat = "nomos.detect.v1"

type Report struct {
	Format       string            `json:"format"`
	Root         string            `json:"root"`
	FilesScanned int               `json:"filesScanned"`
	Languages    []LanguageFinding `json:"languages"`
	Tools        []ToolFinding     `json:"tools"`
	CI           []CIFinding       `json:"ci"`
	Surfaces     []SurfaceFinding  `json:"surfaces"`
}

type LanguageFinding struct {
	Name     string     `json:"name"`
	Files    int        `json:"files"`
	Evidence []Evidence `json:"evidence"`
}

type ToolFinding struct {
	Name     string     `json:"name"`
	Kind     string     `json:"kind"`
	Evidence []Evidence `json:"evidence"`
}

type CIFinding struct {
	Provider string     `json:"provider"`
	Evidence []Evidence `json:"evidence"`
}

type SurfaceFinding struct {
	Name       string     `json:"name"`
	Confidence string     `json:"confidence"`
	Evidence   []Evidence `json:"evidence"`
}

type Evidence struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}
