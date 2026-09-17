package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

var errCapacity = errors.New("too many pending downloads")

const (
	maxPendingTokens  = 1000
	maxPendingPerUser = 10
)

type tokenStore struct {
	mu     sync.Mutex
	tokens map[string]pendingDownload
}

type pendingDownload struct {
	owner    string
	download download
}

type download struct {
	URL       string
	Name      string
	ExpiresAt int64
}

func newTokenStore() *tokenStore {
	return &tokenStore{tokens: make(map[string]pendingDownload)}
}

func tokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func (s *tokenStore) create(owner string, d download) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeLocked(time.Now().Unix())

	// Limits and insertion share the consumption lock, so concurrent issuers
	// cannot exceed the bound. Scanning is bounded by maxPendingTokens.
	ownerCount := 0
	for _, entry := range s.tokens {
		if entry.owner == owner {
			ownerCount++
		}
	}
	if len(s.tokens) >= maxPendingTokens || ownerCount >= maxPendingPerUser {
		return "", errCapacity
	}
	for {
		var random [32]byte
		if _, err := rand.Read(random[:]); err != nil {
			return "", err
		}
		token := hex.EncodeToString(random[:])
		key := tokenHash(token)
		if _, exists := s.tokens[key]; !exists {
			s.tokens[key] = pendingDownload{owner: owner, download: d}
			return token, nil
		}
	}
}

func (s *tokenStore) consume(token string) (download, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := tokenHash(token)
	entry, exists := s.tokens[key]
	// Lookup, deletion, and expiry validation are atomic. Release the lock
	// before the caller performs any upstream I/O.
	delete(s.tokens, key)
	if !exists || entry.download.ExpiresAt <= time.Now().Unix() {
		return download{}, false
	}
	return entry.download, true
}

func (s *tokenStore) purge() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeLocked(time.Now().Unix())
}

func (s *tokenStore) purgeLocked(now int64) {
	for key, entry := range s.tokens {
		if entry.download.ExpiresAt <= now {
			delete(s.tokens, key)
		}
	}
}
