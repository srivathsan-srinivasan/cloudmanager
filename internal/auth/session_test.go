package auth

import (
	"strings"
	"testing"
	"time"
)

func TestMemoryStoreExpiresSessions(t *testing.T) {
	store := NewMemoryStore()
	store.Put("aws/prod", Session{
		Provider:  "AWS",
		AccountID: "1111",
		ExpiresAt: time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC),
		Env:       map[string]string{"AWS_ACCESS_KEY_ID": "temp"},
	})

	if _, ok := store.Get("aws/prod", time.Date(2026, 4, 29, 9, 59, 0, 0, time.UTC)); !ok {
		t.Fatal("expected session before expiry")
	}
	if _, ok := store.Get("aws/prod", time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)); ok {
		t.Fatal("expected session to expire")
	}
}

func TestBuildEnvOverridesWithoutMutatingSession(t *testing.T) {
	session := Session{Env: map[string]string{"AWS_PROFILE": "jit", "AWS_REGION": "us-east-1"}}
	env := BuildEnv([]string{"PATH=/bin", "AWS_PROFILE=old"}, session)
	joined := strings.Join(env, "\n")

	if !strings.Contains(joined, "AWS_PROFILE=jit") {
		t.Fatalf("expected session env override, got %v", env)
	}
	if !strings.Contains(joined, "PATH=/bin") {
		t.Fatalf("expected base env to remain, got %v", env)
	}
	session.Env["AWS_PROFILE"] = "changed"
	if strings.Contains(strings.Join(env, "\n"), "changed") {
		t.Fatal("expected built env to be detached from session map")
	}
}
