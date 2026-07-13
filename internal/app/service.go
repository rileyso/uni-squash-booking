// Package app coordinates application use cases and readiness.
package app

import (
	"context"
	"fmt"
	"time"

	"github.com/rileyso/uni-squash-booking/internal/config"
	"github.com/rileyso/uni-squash-booking/internal/domain"
	"github.com/rileyso/uni-squash-booking/internal/sqlite"
)

type Service struct {
	config   config.Config
	store    *sqlite.Store
	location *time.Location
	now      func() time.Time
}

func New(configuration config.Config, store *sqlite.Store) (*Service, error) {
	location, err := time.LoadLocation("Australia/Sydney")
	if err != nil {
		return nil, fmt.Errorf("load Australia/Sydney: %w", err)
	}
	service := &Service{config: configuration, store: store, location: location, now: time.Now}
	if configuration.Synthetic {
		today := domain.CivilDateFromTime(service.now(), location)
		if err := store.LoadSyntheticFixtures(context.Background(), today, location); err != nil {
			return nil, err
		}
	}
	return service, nil
}

func (s *Service) Ready(ctx context.Context) error {
	return s.store.Ready(ctx, s.config.RecoveryGeneration)
}

func (s *Service) Synthetic() bool { return s.config.Synthetic }
