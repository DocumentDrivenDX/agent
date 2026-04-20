package agent

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const defaultRouteAttemptCooldown = 30 * time.Second

func (s *service) RecordRouteAttempt(_ context.Context, attempt RouteAttempt) error {
	key := normalizeRouteAttemptKey(attempt)
	if key.Harness == "" && key.Provider == "" {
		return fmt.Errorf("route attempt requires harness or provider")
	}
	status := strings.ToLower(strings.TrimSpace(attempt.Status))
	if status == "" {
		return fmt.Errorf("route attempt status is required")
	}
	recordedAt := attempt.Timestamp
	if recordedAt.IsZero() {
		recordedAt = time.Now()
	}
	recordedAt = recordedAt.UTC()

	s.routeAttemptMu.Lock()
	defer s.routeAttemptMu.Unlock()
	if s.routeAttempts == nil {
		s.routeAttempts = make(map[routeAttemptKey]routeAttemptRecord)
	}

	if routeAttemptSucceeded(status) {
		s.clearRouteAttemptFailuresLocked(key)
		return nil
	}

	s.routeAttempts[key] = routeAttemptRecord{
		key:        key,
		status:     status,
		reason:     strings.TrimSpace(attempt.Reason),
		err:        strings.TrimSpace(attempt.Error),
		duration:   attempt.Duration,
		recordedAt: recordedAt,
	}
	return nil
}

func normalizeRouteAttemptKey(attempt RouteAttempt) routeAttemptKey {
	return routeAttemptKey{
		Harness:  strings.TrimSpace(attempt.Harness),
		Provider: strings.TrimSpace(attempt.Provider),
		Model:    strings.TrimSpace(attempt.Model),
		Endpoint: strings.TrimSpace(attempt.Endpoint),
	}
}

func routeAttemptSucceeded(status string) bool {
	switch status {
	case "success", "ok", "succeeded":
		return true
	default:
		return false
	}
}

func (s *service) clearRouteAttemptFailuresLocked(key routeAttemptKey) {
	for existing := range s.routeAttempts {
		if routeAttemptKeysMatch(existing, key) {
			delete(s.routeAttempts, existing)
		}
	}
}

func routeAttemptKeysMatch(existing, query routeAttemptKey) bool {
	if query.Harness != "" && existing.Harness != query.Harness {
		return false
	}
	if query.Provider != "" && existing.Provider != query.Provider {
		return false
	}
	if query.Model != "" && existing.Model != query.Model {
		return false
	}
	if query.Endpoint != "" && existing.Endpoint != query.Endpoint {
		return false
	}
	return true
}

func (s *service) activeRouteAttempts(now time.Time, ttl time.Duration) []routeAttemptRecord {
	if ttl <= 0 {
		ttl = defaultRouteAttemptCooldown
	}
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()

	s.routeAttemptMu.Lock()
	defer s.routeAttemptMu.Unlock()
	if len(s.routeAttempts) == 0 {
		return nil
	}
	out := make([]routeAttemptRecord, 0, len(s.routeAttempts))
	for key, record := range s.routeAttempts {
		if !record.recordedAt.Add(ttl).After(now) {
			delete(s.routeAttempts, key)
			continue
		}
		out = append(out, record)
	}
	return out
}

func routeAttemptCooldown(record routeAttemptRecord, ttl time.Duration) *CooldownState {
	if ttl <= 0 {
		ttl = defaultRouteAttemptCooldown
	}
	reason := record.reason
	if reason == "" {
		reason = "route_attempt_failure"
	}
	return &CooldownState{
		Reason:      reason,
		Until:       record.recordedAt.Add(ttl),
		FailCount:   1,
		LastError:   record.err,
		LastAttempt: record.recordedAt,
	}
}
