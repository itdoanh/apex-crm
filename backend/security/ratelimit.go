package security

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// clientEntry lưu trữ bộ limiter và thời điểm hoạt động cuối cùng của 1 IP
type clientEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// IPRateLimiter quản lý rate limiting phân tầng theo từng IP và endpoint
type IPRateLimiter struct {
	mu      sync.RWMutex
	clients map[string]*clientEntry
	rate    rate.Limit
	burst   int
	ttl     time.Duration
}

// NewIPRateLimiter khởi tạo một bộ giới hạn tần suất có tự động dọn dẹp RAM
func NewIPRateLimiter(r rate.Limit, burst int, ttl time.Duration) *IPRateLimiter {
	limiter := &IPRateLimiter{
		clients: make(map[string]*clientEntry),
		rate:    r,
		burst:   burst,
		ttl:     ttl,
	}

	// Chạy goroutine định kỳ dọn dẹp các IP không còn hoạt động, ngăn chặn OOM (Memory Leak)
	go limiter.cleanupStaleClients()

	return limiter
}

// Allow kiểm tra xem một IP có được phép thực hiện request tiếp theo hay không
func (i *IPRateLimiter) Allow(ip string) bool {
	i.mu.Lock()
	defer i.mu.Unlock()

	entry, exists := i.clients[ip]
	if !exists {
		entry = &clientEntry{
			limiter:  rate.NewLimiter(i.rate, i.burst),
			lastSeen: time.Now(),
		}
		i.clients[ip] = entry
	}

	entry.lastSeen = time.Now()
	return entry.limiter.Allow()
}

// cleanupStaleClients dọn dẹp các bản ghi IP quá hạn
func (i *IPRateLimiter) cleanupStaleClients() {
	ticker := time.NewTicker(3 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		i.mu.Lock()
		now := time.Now()
		for ip, entry := range i.clients {
			if now.Sub(entry.lastSeen) > i.ttl {
				delete(i.clients, ip)
			}
		}
		i.mu.Unlock()
	}
}
