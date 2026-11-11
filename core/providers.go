package core

import (
	"context"
	"errors"

	"kiko/core/muse"
)

func (s *Service) SetModel(ctx context.Context, model string) error {
	if !muse.ValidModel(model) {
		return errors.New("Choose one of the listed Muse models.")
	}
	return s.Store.SetModel(ctx, model)
}

func (s *Service) describeMedia(ctx context.Context, mime string, data []byte, question string) (string, error) {
	key, err := s.Keys.Get()
	if err != nil {
		return "", errors.New("Reconnect Meta Muse to inspect images.")
	}
	model, err := s.Store.DefaultModel(ctx)
	if err != nil {
		return "", err
	}
	return (muse.Vision{BaseURL: s.VisionURL}).Describe(ctx, key, model, mime, data, question)
}
