package profiles

import (
	"fmt"
	"sort"
	"strings"
)

// NodeTypeDef describes a node type within a profile.
type NodeTypeDef struct {
	Name        string   `json:"name"        yaml:"name"`
	Description string   `json:"description" yaml:"description"`
	Structural  bool     `json:"structural"  yaml:"structural"`
	CanContain  []string `json:"can_contain" yaml:"can_contain"`
}

// HierarchyLevel describes a depth in the document hierarchy.
type HierarchyLevel struct {
	Depth    int    `json:"depth"    yaml:"depth"`
	NodeType string `json:"node_type" yaml:"node_type"`
	Label    string `json:"label"    yaml:"label"`
}

// MetadataField describes an expected metadata field.
type MetadataField struct {
	Name     string `json:"name"     yaml:"name"`
	Required bool   `json:"required" yaml:"required"`
	Type     string `json:"type"     yaml:"type"`
}

// Profile defines the atomization profile for a document domain.
type Profile struct {
	ID          string            `json:"id"          yaml:"id"`
	Name        string            `json:"name"        yaml:"name"`
	Description string            `json:"description" yaml:"description"`
	Domain      string            `json:"domain"      yaml:"domain"`
	NodeTypes   []NodeTypeDef     `json:"node_types"  yaml:"node_types"`
	Hierarchy   []HierarchyLevel  `json:"hierarchy"   yaml:"hierarchy"`
	Metadata    []MetadataField   `json:"metadata"    yaml:"metadata"`
}

// NodeTypeNames returns sorted node type names.
func (p Profile) NodeTypeNames() []string {
	names := make([]string, 0, len(p.NodeTypes))
	for _, nt := range p.NodeTypes {
		names = append(names, nt.Name)
	}
	sort.Strings(names)
	return names
}

// RequiredMetadata returns only required metadata fields.
func (p Profile) RequiredMetadata() []MetadataField {
	var required []MetadataField
	for _, m := range p.Metadata {
		if m.Required {
			required = append(required, m)
		}
	}
	return required
}

// HasNodeType checks if the profile defines a given node type.
func (p Profile) HasNodeType(name string) bool {
	for _, nt := range p.NodeTypes {
		if nt.Name == name {
			return true
		}
	}
	return false
}

// MaxDepth returns the deepest hierarchy level.
func (p Profile) MaxDepth() int {
	max := 0
	for _, h := range p.Hierarchy {
		if h.Depth > max {
			max = h.Depth
		}
	}
	return max
}

var registry = map[string]Profile{}

// Register adds a profile to the global registry.
func Register(p Profile) {
	registry[strings.ToLower(p.ID)] = p
}

// Lookup returns a profile by ID.
func Lookup(id string) (Profile, error) {
	p, ok := registry[strings.ToLower(id)]
	if !ok {
		return Profile{}, fmt.Errorf("unknown profile %q; known: %s", id, strings.Join(KnownIDs(), ", "))
	}
	return p, nil
}

// KnownIDs returns sorted registered profile IDs.
func KnownIDs() []string {
	ids := make([]string, 0, len(registry))
	for k := range registry {
		ids = append(ids, k)
	}
	sort.Strings(ids)
	return ids
}

func init() {
	Register(LawRegulation())
	Register(GameRules())
	Register(RBOKLawbook())
}
