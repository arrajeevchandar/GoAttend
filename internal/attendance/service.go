package attendance

import (
	"context"
	"errors"
	"time"
)

// Event represents a recorded attendance event.
type Event struct {
	ID         string
	UserID     string
	DeviceID   string
	When       time.Time
	Location   string
	ImageURL   string
	Status     string
	MatchScore *float64
	CreatedAt  time.Time
}

// Service coordinates attendance checks and deduplication.
type Service struct {
	repo        *Repository
	dedupWindow time.Duration
}

// NewService creates a service backed by a repository.
func NewService(repo *Repository, dedupWindow time.Duration) *Service {
	if dedupWindow <= 0 {
		dedupWindow = 5 * time.Minute
	}
	return &Service{repo: repo, dedupWindow: dedupWindow}
}

// RegisterDevice validates and persists device metadata.
func (s *Service) RegisterDevice(ctx context.Context, deviceID string) error {
	if deviceID == "" {
		return errors.New("device id required")
	}
	return s.repo.UpsertDevice(ctx, deviceID)
}

// CheckIn records a new attendance event with deduplication.
func (s *Service) CheckIn(ctx context.Context, userID, deviceID, location, imageURL string) (Event, error) {
	if userID == "" || deviceID == "" {
		return Event{}, errors.New("user and device required")
	}
	if recent, err := s.repo.RecentEvent(ctx, userID, deviceID, s.dedupWindow); err != nil {
		return Event{}, err
	} else if recent != nil {
		return *recent, nil
	}

	evt := Event{
		UserID:   userID,
		DeviceID: deviceID,
		When:     time.Now().UTC(),
		Location: location,
		ImageURL: imageURL,
		Status:   "pending",
	}
	return s.repo.InsertEvent(ctx, evt)
}
