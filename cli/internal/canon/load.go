package canon

import "github.com/RBOKproject/Nomos/cli/internal/docload"

// LoadPromotionBundle is the engine's loader for a canon-promotion bundle
// (specs/canon-promotion.cue), YAML or JSON. `nomos canon validate` and the
// contract registry's compatibility read both go through it.
func LoadPromotionBundle(path string) (PromotionBundle, error) {
	var b PromotionBundle
	if err := docload.Load(path, &b); err != nil {
		return PromotionBundle{}, err
	}
	return b, nil
}
