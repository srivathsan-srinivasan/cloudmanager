package auth

import (
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type Session struct {
	Provider  string
	AccountID string
	AuthMode  string
	ExpiresAt time.Time
	Env       map[string]string
}

func (s Session) Expired(now time.Time) bool {
	return !s.ExpiresAt.IsZero() && !now.Before(s.ExpiresAt)
}

type MemoryStore struct {
	mu       sync.Mutex
	sessions map[string]Session
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{sessions: make(map[string]Session)}
}

func (s *MemoryStore) Put(key string, session Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[key] = cloneSession(session)
}

func (s *MemoryStore) Get(key string, now time.Time) (Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[key]
	if !ok {
		return Session{}, false
	}
	if session.Expired(now) {
		delete(s.sessions, key)
		return Session{}, false
	}
	return cloneSession(session), true
}

func (s *MemoryStore) Clear(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, key)
}

func BuildEnv(base []string, session Session) []string {
	env := make(map[string]string, len(base)+len(session.Env))
	for _, item := range base {
		key, value, ok := strings.Cut(item, "=")
		if !ok || key == "" {
			continue
		}
		env[key] = value
	}
	for key, value := range session.Env {
		if key == "" {
			continue
		}
		env[key] = value
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+env[key])
	}
	return out
}

func CurrentEnv(session Session) []string {
	return BuildEnv(os.Environ(), session)
}

func cloneSession(session Session) Session {
	if session.Env == nil {
		return session
	}
	env := make(map[string]string, len(session.Env))
	for key, value := range session.Env {
		env[key] = value
	}
	session.Env = env
	return session
}
