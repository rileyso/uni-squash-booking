package app

import (
	"context"
	"errors"
	"strings"

	"github.com/rileyso/uni-squash-booking/internal/domain"
	"github.com/rileyso/uni-squash-booking/internal/sqlite"
)

var ErrCapabilityDisabled = errors.New("capability is disabled")

func (s *Service) SuspendAccount(ctx context.Context, identifier string) (sqlite.ModerationAccount, error) {
	if strings.TrimSpace(identifier) == "" {
		return sqlite.ModerationAccount{}, ErrInvalidInput
	}
	today := domain.CivilDateFromTime(s.now(), s.location).String()
	return s.store.SuspendAccount(ctx, identifier, today)
}

func (s *Service) ReinstateAccount(ctx context.Context, identifier string) (sqlite.ModerationAccount, error) {
	if strings.TrimSpace(identifier) == "" {
		return sqlite.ModerationAccount{}, ErrInvalidInput
	}
	return s.store.ReinstateAccount(ctx, identifier)
}

func (s *Service) PermanentlyDeleteAccount(ctx context.Context, exactUsername, confirmation, backupDestination string) (sqlite.ModerationAccount, error) {
	if !s.config.Capabilities.SelfDelete {
		return sqlite.ModerationAccount{}, ErrCapabilityDisabled
	}
	if exactUsername == "" || exactUsername != confirmation || strings.TrimSpace(backupDestination) == "" {
		return sqlite.ModerationAccount{}, ErrInvalidInput
	}
	if err := s.store.Backup(ctx, backupDestination); err != nil {
		return sqlite.ModerationAccount{}, err
	}
	return s.store.PermanentlyDeleteAccount(ctx, exactUsername, s.now())
}
