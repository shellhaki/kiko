package deck

import (
	"crypto/rand"
	"errors"
	"math"
)

type Chart struct {
	Labels []string  `json:"labels"`
	Values []float64 `json:"values"`
}
type Slide struct {
	ID      string   `json:"id"`
	Layout  string   `json:"layout"`
	Title   string   `json:"title"`
	Body    []string `json:"body,omitempty"`
	Left    []string `json:"left,omitempty"`
	Right   []string `json:"right,omitempty"`
	Notes   string   `json:"notes,omitempty"`
	ImageID string   `json:"imageId,omitempty"`
	Chart   *Chart   `json:"chart,omitempty"`
}
type Deck struct {
	Slides []Slide `json:"slides"`
}

func Prepare(deck *Deck) error {
	if len(deck.Slides) < 1 || len(deck.Slides) > 100 {
		return errors.New("a deck needs 1–100 slides")
	}
	seen := map[string]bool{}
	for i := range deck.Slides {
		s := &deck.Slides[i]
		if s.ID == "" {
			s.ID = rand.Text()
		}
		if seen[s.ID] || len(s.ID) > 120 {
			return errors.New("slide IDs must be unique")
		}
		seen[s.ID] = true
		switch s.Layout {
		case "title", "bullets", "columns", "image", "chart":
		default:
			return errors.New("choose title, bullets, columns, image or chart layout")
		}
		if len([]rune(s.Title)) > 100 || len(s.Notes) > 10000 {
			return errors.New("slide title or notes are too long")
		}
		for _, list := range [][]string{s.Body, s.Left, s.Right} {
			if len(list) > 6 {
				return errors.New("use at most six points per text column")
			}
			for _, point := range list {
				if len([]rune(point)) > 160 {
					return errors.New("keep each slide point to 160 characters")
				}
			}
		}
		if s.Layout == "image" && s.ImageID == "" {
			return errors.New("image slides require an attachment imageId")
		}
		if s.Layout == "chart" {
			if s.Chart == nil || len(s.Chart.Labels) == 0 || len(s.Chart.Labels) > 12 || len(s.Chart.Labels) != len(s.Chart.Values) {
				return errors.New("charts need 1–12 matching labels and values")
			}
			for j, value := range s.Chart.Values {
				if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > 1e12 || len([]rune(s.Chart.Labels[j])) > 40 {
					return errors.New("chart values or labels exceed limits")
				}
			}
		}
	}
	return nil
}
