package muse

import "testing"

func TestCatalogIdsAreUniqueAndModesMatchIds(t *testing.T) {
	seen := map[string]bool{}
	for _, model := range Models {
		if seen[model.ID] || model.Name == "" || model.Description == "" {
			t.Fatal("duplicate or incomplete model", model)
		}
		seen[model.ID] = true
		if model.Contributor() != (model.Mode == "contributor") {
			t.Fatal("mode disagrees with the model id", model)
		}
	}
	if !ValidModel(DefaultModel) || ValidModel("gpt-4o") || ValidModel("") {
		t.Fatal("default model must be listed and unknown ids refused")
	}
}
