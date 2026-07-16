// Package app coordinates application use cases and readiness.
package app

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/rileyso/uni-squash-booking/internal/config"
	"github.com/rileyso/uni-squash-booking/internal/domain"
	"github.com/rileyso/uni-squash-booking/internal/sqlite"
)

type Service struct {
	config            config.Config
	store             *sqlite.Store
	location          *time.Location
	now               func() time.Time
	adminUsername     string
	adminPasswordHash string
}

func New(configuration config.Config, store *sqlite.Store) (*Service, error) {
	location, err := time.LoadLocation("Australia/Sydney")
	if err != nil {
		return nil, fmt.Errorf("load Australia/Sydney: %w", err)
	}
	service := &Service{config: configuration, store: store, location: location, now: time.Now}
	service.adminUsername = configuration.AdminUsername
	service.adminPasswordHash = configuration.AdminPasswordHash
	if configuration.Synthetic && service.adminPasswordHash == "" {
		service.adminPasswordHash, err = hashPIN("synthetic-admin-password")
		if err != nil {
			return nil, fmt.Errorf("hash synthetic administrator password: %w", err)
		}
	}
	if service.adminUsername != "" && service.adminPasswordHash != "" {
		fingerprint := fmt.Sprintf("%x", sha256.Sum256([]byte(service.adminUsername+"\x00"+service.adminPasswordHash)))
		if err := store.ApplyAdminCredentialFingerprint(context.Background(), fingerprint); err != nil {
			return nil, fmt.Errorf("apply administrator credential fingerprint: %w", err)
		}
	}
	if configuration.Synthetic {
		today := domain.CivilDateFromTime(service.now(), location)
		trialPINHash, err := hashPIN("1111")
		if err != nil {
			return nil, fmt.Errorf("hash synthetic trial PIN: %w", err)
		}
		if err := store.LoadSyntheticFixtures(context.Background(), today, location, trialPINHash); err != nil {
			return nil, err
		}
	}
	return service, nil
}

func (s *Service) Ready(ctx context.Context) error {
	return s.store.Ready(ctx, s.config.RecoveryGeneration)
}

func (s *Service) Synthetic() bool              { return s.config.Synthetic }
func (s *Service) SecureCookies() bool          { return s.config.Environment == config.Production }
func (s *Service) DeviceCookieSecret() string   { return s.config.DeviceCookieSecret }
func (s *Service) SelfDeleteEnabled() bool      { return s.config.Capabilities.SelfDelete }
func (s *Service) NamedAttendanceEnabled() bool { return s.config.Capabilities.PublicNamedAttendance }
