package platform_test

import (
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeClock struct{ current time.Time }

func (clock *fakeClock) now() time.Time           { return clock.current }
func (clock *fakeClock) advance(by time.Duration) { clock.current = clock.current.Add(by) }

func TestRateLimiterAllowsBurstThenBlocksWithRetryAfter(t *testing.T) {
	clock := &fakeClock{current: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)}
	limiter := platform.NewRateLimiter(platform.RateLimit{PerMinute: 6, Burst: 2}, clock.now)

	for attempt := 1; attempt <= 2; attempt++ {
		if allowed, _ := limiter.Allow("a"); !allowed {
			t.Fatalf("attempt %d should be allowed", attempt)
		}
	}
	allowed, retryAfter := limiter.Allow("a")
	if allowed {
		t.Fatal("third attempt should be blocked")
	}
	if retryAfter <= 0 || retryAfter > 10*time.Second {
		t.Errorf("retryAfter = %s, want within one token interval", retryAfter)
	}
	if allowed, _ := limiter.Allow("b"); !allowed {
		t.Error("a different key must have its own bucket")
	}
	clock.advance(10 * time.Second)
	if allowed, _ := limiter.Allow("a"); !allowed {
		t.Error("a token should be available after waiting")
	}
}

func TestRateLimitMiddlewareResponds429(t *testing.T) {
	clock := &fakeClock{current: time.Now()}
	limiter := platform.NewRateLimiter(platform.RateLimit{PerMinute: 1, Burst: 1}, clock.now)
	handler := platform.RateLimitByClientAddress(limiter, platform.NewClientAddressResolver(nil), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest("POST", "/", nil))
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest("POST", "/", nil))

	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d", first.Code)
	}
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want 429", second.Code)
	}
	if second.Header().Get("Retry-After") == "" {
		t.Error("missing Retry-After header")
	}
	if body := second.Body.String(); body != "{\"error\":{\"code\":\"rate_limited\"}}\n" {
		t.Errorf("body = %q", body)
	}
}
