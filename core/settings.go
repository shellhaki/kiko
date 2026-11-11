package core

import (
	"context"
	"errors"
)

func (s *Service) cancelRuns(ctx context.Context) error {
	s.runMu.Lock()
	s.closing = true
	pending := []*activeRun{}
	for _, active := range s.active {
		active.cancel()
		pending = append(pending, active)
	}
	s.runMu.Unlock()
	for _, active := range pending {
		select {
		case <-active.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
func (s *Service) resumeSends() { s.runMu.Lock(); s.closing = false; s.runMu.Unlock() }
func (s *Service) RemoveKey(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.resumeSends()
	if err := s.cancelRuns(ctx); err != nil {
		return err
	}
	if err := s.Keys.Delete(); err != nil {
		return errors.New("Couldn’t remove the saved key. Unlock your keychain and retry.")
	}
	return nil
}
func (s *Service) Reset(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.resumeSends()
	if err := s.cancelRuns(ctx); err != nil {
		return errors.New("Couldn’t stop active work. Wait and retry reset.")
	}
	prefs, err := s.Store.Preferences(ctx)
	if err != nil {
		return err
	}
	prefs.ResetInProgress = true
	if err = s.Store.SavePreferences(ctx, prefs); err != nil {
		return errors.New("Couldn’t prepare the data reset. Nothing was deleted.")
	}
	if err = s.Keys.Delete(); err != nil {
		return errors.New("Reset paused: couldn’t remove the key from secure storage. Unlock your keychain and retry reset.")
	}
	if s.SecretFor != nil {
		if err = s.SecretFor("tavily").Delete(); err != nil {
			return errors.New("Reset paused: couldn’t remove the Tavily key. Unlock your keychain and retry reset.")
		}
	}
	if err = s.Store.ResetData(ctx); err != nil {
		return errors.New("The key was removed, but local data cleanup failed. Retry reset to finish.")
	}
	return nil
}
