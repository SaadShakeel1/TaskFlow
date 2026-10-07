package main

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const inactiveClientTTL = 10 * time.Minute

type clientLimiter struct {
	bucket   *rate.Limiter
	lastSeen time.Time
}

type rateLimiter struct {
	mu          sync.Mutex
	clients     map[string]*clientLimiter
	now         func() time.Time
	lastCleanup time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{clients: make(map[string]*clientLimiter), now: time.Now}
}

func (l *rateLimiter) allow(key string, limit rate.Limit, burst int) bool {
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	l.removeInactiveClients(now)

	client, exists := l.clients[key]
	if !exists {
		client = &clientLimiter{bucket: rate.NewLimiter(limit, burst), lastSeen: now}
		l.clients[key] = client
	}

	client.lastSeen = now
	return client.bucket.AllowN(now, 1)
}

func (l *rateLimiter) removeInactiveClients(now time.Time) {
	if now.Sub(l.lastCleanup) < inactiveClientTTL {
		return
	}

	for key, client := range l.clients {
		if now.Sub(client.lastSeen) > inactiveClientTTL {
			delete(l.clients, key)
		}
	}

	l.lastCleanup = now
}
