package platform

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type RateLimit struct {
	PerMinute int
	Burst     int
}

type RateLimiter struct {
	mutex       sync.Mutex
	buckets     map[string]*rateBucket
	limit       rate.Limit
	burst       int
	idleTTL     time.Duration
	lastSweepAt time.Time
	now         func() time.Time
}

type rateBucket struct {
	limiter    *rate.Limiter
	lastSeenAt time.Time
}

func NewRateLimiter(limit RateLimit, now func() time.Time) *RateLimiter {
	return &RateLimiter{
		buckets: map[string]*rateBucket{},
		limit:   rate.Limit(float64(limit.PerMinute) / 60),
		burst:   limit.Burst,
		idleTTL: 10 * time.Minute,
		now:     now,
	}
}

func (limiter *RateLimiter) Allow(key string) (bool, time.Duration) {
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()

	now := limiter.now()
	limiter.evictIdleBuckets(now)
	bucket, found := limiter.buckets[key]
	if !found {
		bucket = &rateBucket{limiter: rate.NewLimiter(limiter.limit, limiter.burst)}
		limiter.buckets[key] = bucket
	}
	bucket.lastSeenAt = now

	reservation := bucket.limiter.ReserveN(now, 1)
	if !reservation.OK() {
		return false, time.Minute
	}
	delay := reservation.DelayFrom(now)
	if delay > 0 {
		reservation.CancelAt(now)
		return false, delay
	}
	return true, 0
}

func (limiter *RateLimiter) evictIdleBuckets(now time.Time) {
	if now.Sub(limiter.lastSweepAt) < limiter.idleTTL {
		return
	}
	for key, bucket := range limiter.buckets {
		if now.Sub(bucket.lastSeenAt) >= limiter.idleTTL {
			delete(limiter.buckets, key)
		}
	}
	limiter.lastSweepAt = now
}

func RateLimitByClientAddress(limiter *RateLimiter, resolver ClientAddressResolver, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowed, retryAfter := limiter.Allow(resolver.Resolve(r).String())
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
			WriteError(w, r, ErrRateLimited)
			return
		}
		next.ServeHTTP(w, r)
	})
}
