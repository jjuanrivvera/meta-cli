package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"
)

const maximumThrottleDelay = 5 * time.Minute

type rateLimiter struct {
	mu       sync.Mutex
	base     time.Duration
	interval time.Duration
	last     time.Time
	sleep    func(time.Duration)
}

func isThrottled(headers http.Header, err error) bool {
	var apiErr *APIError
	ok := errors.As(err, &apiErr)
	return (ok && isThrottleCode(apiErr.Code)) || maxUsageHeaders(headers) >= 100
}

func maxUsageHeaders(headers http.Header) int {
	usage := 0
	for _, name := range []string{"X-App-Usage", "X-Business-Use-Case-Usage", "X-Page-Usage"} {
		if value := maxUsage(headers.Get(name)); value > usage {
			usage = value
		}
	}
	return usage
}

func throttleDelay(headers http.Header, now time.Time) (time.Duration, bool) {
	if delay, ok := retryAfter(headers.Get("Retry-After"), now); ok {
		return delay, true
	}
	minutes := estimatedRegainMinutes(headers.Get("X-Business-Use-Case-Usage"))
	if minutes > 0 {
		return time.Duration(minutes) * time.Minute, true
	}
	return 0, false
}

func estimatedRegainMinutes(raw string) int {
	if raw == "" {
		return 0
	}
	var value any
	if json.Unmarshal([]byte(raw), &value) != nil {
		return 0
	}
	maximum := 0
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			for key, child := range typed {
				if key == "estimated_time_to_regain_access" {
					if number, ok := child.(float64); ok && int(number) > maximum {
						maximum = int(number)
					}
				}
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
	return maximum
}

func newRateLimiter(rps float64, sleep func(time.Duration)) *rateLimiter {
	if rps <= 0 {
		rps = 10
	}
	interval := time.Duration(float64(time.Second) / rps)
	return &rateLimiter{base: interval, interval: interval, sleep: sleep}
}

func (limiter *rateLimiter) Wait() {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if delay := limiter.interval - time.Since(limiter.last); limiter.last.IsZero() || delay <= 0 {
		limiter.last = time.Now()
	} else {
		limiter.sleep(delay)
		limiter.last = time.Now()
	}
}

func (limiter *rateLimiter) Observe(headers http.Header, status int) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	usage := maxUsageHeaders(headers)
	if status == http.StatusTooManyRequests {
		limiter.interval *= 2
	} else if usage >= 90 {
		limiter.interval = limiter.base * 4
	} else if usage >= 75 {
		limiter.interval = limiter.base * 2
	} else if limiter.interval > limiter.base {
		limiter.interval -= (limiter.interval - limiter.base) / 4
	}
}

func maxUsage(raw string) int {
	if raw == "" {
		return 0
	}
	var value any
	if json.Unmarshal([]byte(raw), &value) != nil {
		return 0
	}
	max := 0
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			for key, child := range typed {
				if key == "call_count" || key == "total_cputime" || key == "total_time" {
					if number, ok := child.(float64); ok && int(number) > max {
						max = int(number)
					}
				}
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
	return max
}
