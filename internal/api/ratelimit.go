package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type rateLimiter struct {
	mu       sync.Mutex
	base     time.Duration
	interval time.Duration
	last     time.Time
	sleep    func(time.Duration)
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
	usage := 0
	for _, name := range []string{"X-App-Usage", "X-Business-Use-Case-Usage", "X-Page-Usage"} {
		if value := maxUsage(headers.Get(name)); value > usage {
			usage = value
		}
	}
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
