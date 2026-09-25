package main

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// rateLimiter manages a map of IP addresses to rate limiters
type rateLimiter struct {
	ips map[string]*rate.Limiter
	mu  *sync.RWMutex
	r   rate.Limit
	b   int
}

// newRateLimiter creates a new IP rate limiter (r = requests per second, b = burst limit)
func newRateLimiter(r float64, b int) *rateLimiter {
	limiter := &rateLimiter{
		ips: make(map[string]*rate.Limiter),
		mu:  &sync.RWMutex{},
		r:   rate.Limit(r),
		b:   b,
	}

	// Simple cleanup routine to prevent memory leaks from old IPs over time
	go func() {
		for {
			time.Sleep(time.Hour)
			limiter.mu.Lock()
			// For simplicity in this small app, we can just clear the map occasionally
			limiter.ips = make(map[string]*rate.Limiter)
			limiter.mu.Unlock()
		}
	}()

	return limiter
}

// getLimiter returns the rate limiter for the provided IP address
func (i *rateLimiter) getLimiter(ip string) *rate.Limiter {
	i.mu.RLock()
	limiter, exists := i.ips[ip]
	i.mu.RUnlock()

	if !exists {
		i.mu.Lock()
		defer i.mu.Unlock()
		
		// Double check it wasn't added while we were waiting for the lock
		limiter, exists = i.ips[ip]
		if !exists {
			limiter = rate.NewLimiter(i.r, i.b)
			i.ips[ip] = limiter
		}
		return limiter
	}

	return limiter
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
