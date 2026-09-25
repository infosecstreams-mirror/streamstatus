package main

import (
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

type client struct {
	limiter  *rate.Limiter
	lastSeen int64
}

// rateLimiter manages a map of IP addresses to rate limiters
type rateLimiter struct {
	ips map[string]*client
	mu  *sync.RWMutex
	r   rate.Limit
	b   int
}

// newRateLimiter creates a new IP rate limiter (r = requests per second, b = burst limit)
func newRateLimiter(r float64, b int) *rateLimiter {
	limiter := &rateLimiter{
		ips: make(map[string]*client),
		mu:  &sync.RWMutex{},
		r:   rate.Limit(r),
		b:   b,
	}

	// Cleanup routine to prevent memory leaks from old IPs
	go func() {
		for {
			time.Sleep(time.Minute)
			limiter.mu.Lock()
			for ip, c := range limiter.ips {
				lastSeen := time.Unix(0, atomic.LoadInt64(&c.lastSeen))
				if time.Since(lastSeen) > 3*time.Minute {
					delete(limiter.ips, ip)
				}
			}
			limiter.mu.Unlock()
		}
	}()

	return limiter
}

// getLimiter returns the rate limiter for the provided IP address
func (i *rateLimiter) getLimiter(ip string) *rate.Limiter {
	i.mu.RLock()
	c, exists := i.ips[ip]
	i.mu.RUnlock()

	if !exists {
		i.mu.Lock()
		defer i.mu.Unlock()
		
		// Double check it wasn't added while we were waiting for the lock
		c, exists = i.ips[ip]
		if !exists {
			c = &client{
				limiter: rate.NewLimiter(i.r, i.b),
			}
			i.ips[ip] = c
		}
	}

	atomic.StoreInt64(&c.lastSeen, time.Now().UnixNano())
	return c.limiter
}

// limitMiddleware applies the rate limiter to an HTTP handler
func (i *rateLimiter) limitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract IP from X-Forwarded-For or Forwarded header since we are behind Caddy
		ip := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ip = strings.Split(forwarded, ",")[0]
		} else if forwarded := r.Header.Get("Forwarded"); forwarded != "" {
			// e.g. "for=192.168.0.1;proto=https"
			parts := strings.Split(forwarded, ";")
			for _, part := range parts {
				if strings.HasPrefix(strings.TrimSpace(part), "for=") {
					ip = strings.TrimPrefix(strings.TrimSpace(part), "for=")
					break
				}
			}
		}

		limiter := i.getLimiter(ip)
		if !limiter.Allow() {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	}
}
