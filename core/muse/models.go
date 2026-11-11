package muse

import "strings"

type Model struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Mode        string `json:"mode"`
	Description string `json:"description"`
}

const DefaultModel = "muse-spark-1.3-contributor"

var Models = []Model{
	{"muse-spark-1.3-contributor", "Muse Spark 1.3", "contributor", "Latest Spark at a lower price. Meta may train on your content."},
	{"muse-spark-1.3", "Muse Spark 1.3", "standard", "Latest Spark. Your content is not used for training."},
	{"muse-spark-1.2-contributor", "Muse Spark 1.2", "contributor", "Previous Spark at a lower price. Meta may train on your content."},
	{"muse-spark-1.2", "Muse Spark 1.2", "standard", "Previous Spark. Your content is not used for training."},
	{"muse-ember-1.0", "Muse Ember", "standard", "Fast and low cost for high-volume work."},
	{"muse-wonder-1.0", "Muse Wonder (preview)", "standard", "Strongest for long, complex agent work."},
}

func ValidModel(id string) bool {
	for _, model := range Models {
		if model.ID == id {
			return true
		}
	}
	return false
}

func (m Model) Contributor() bool { return strings.HasSuffix(m.ID, "-contributor") }
