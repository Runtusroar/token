package service

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"ai-relay/internal/config"
	"ai-relay/internal/model"
	"ai-relay/internal/pkg"
	"ai-relay/internal/repository"
)

// ChannelService handles channel selection logic for upstream AI providers.
// It caches the channel list per model to avoid querying the database on every
// proxy request. The cache is invalidated after cacheTTL.
type ChannelService struct {
	Repo   *repository.ChannelRepo
	Config *config.Config

	mu       sync.RWMutex
	cache    map[string]channelCacheEntry
	cacheTTL time.Duration
}

type channelCacheEntry struct {
	channels []model.Channel
	fetchedAt time.Time
}

// getChannels returns active channels for the model, using a short-lived cache.
func (s *ChannelService) getChannels(modelName string) ([]model.Channel, error) {
	ttl := s.cacheTTL
	if ttl == 0 {
		ttl = 10 * time.Second
	}

	// Fast path: read from cache.
	s.mu.RLock()
	if entry, ok := s.cache[modelName]; ok && time.Since(entry.fetchedAt) < ttl {
		s.mu.RUnlock()
		return entry.channels, nil
	}
	s.mu.RUnlock()

	// Slow path: query DB and update cache.
	channels, err := s.Repo.FindActiveByModel(modelName)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	if s.cache == nil {
		s.cache = make(map[string]channelCacheEntry)
	}
	s.cache[modelName] = channelCacheEntry{channels: channels, fetchedAt: time.Now()}
	s.mu.Unlock()

	return channels, nil
}

// InvalidateCache clears the channel cache (e.g. after admin edits a channel).
func (s *ChannelService) InvalidateCache() {
	s.mu.Lock()
	s.cache = nil
	s.mu.Unlock()
}

// SelectChannel picks an active channel that supports modelName using a two-
// phase strategy:
//  1. Group channels by priority; keep only the group with the lowest
//     (i.e. highest-priority) priority value.
//  2. Within that group, perform weighted-random selection using each
//     channel's Weight field.
//
// The returned channel's ApiKey has been decrypted and is ready to use.
func (s *ChannelService) SelectChannel(modelName string) (*model.Channel, error) {
	channels, err := s.getChannels(modelName)
	if err != nil {
		return nil, fmt.Errorf("channel: query: %w", err)
	}
	if len(channels) == 0 {
		return nil, fmt.Errorf("channel: no active channel for model %q", modelName)
	}

	// Step 1: isolate the best-priority group (lowest priority number).
	bestPriority := channels[0].Priority
	var group []model.Channel
	for _, ch := range channels {
		if ch.Priority == bestPriority {
			group = append(group, ch)
		}
	}

	// Step 2: weighted random selection.
	totalWeight := 0
	for _, ch := range group {
		w := ch.Weight
		if w <= 0 {
			w = 1
		}
		totalWeight += w
	}

	pick := rand.Intn(totalWeight) //nolint:gosec // non-crypto use
	cumulative := 0
	var selected *model.Channel
	for i := range group {
		w := group[i].Weight
		if w <= 0 {
			w = 1
		}
		cumulative += w
		if pick < cumulative {
			selected = &group[i]
			break
		}
	}
	if selected == nil {
		selected = &group[0]
	}

	// Decrypt API key.
	plainKey, err := pkg.Decrypt(selected.ApiKey, s.Config.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("channel: decrypt api key for channel %d: %w", selected.ID, err)
	}

	// Return a copy with the decrypted key so the caller can use it safely.
	result := *selected
	result.ApiKey = plainKey
	return &result, nil
}
