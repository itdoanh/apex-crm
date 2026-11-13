package cache

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"apex-backend/db"
	"apex-backend/models"
)

// LandingCache quản lý bộ nhớ đệm RAM tốc độ cực cao (<0.1ms) cho Landing Page & Feature Flags
type LandingCache struct {
	mu           sync.RWMutex
	byDomainPage map[string]*models.LandingPage
	byPageID     map[string]*models.LandingPage
	store        *db.Store
}

// NewLandingCache khởi tạo bộ nhớ đệm
func NewLandingCache(store *db.Store) *LandingCache {
	c := &LandingCache{
		byDomainPage: make(map[string]*models.LandingPage),
		byPageID:     make(map[string]*models.LandingPage),
		store:        store,
	}
	return c
}

func makeKey(domain, pageID string) string {
	return strings.ToLower(fmt.Sprintf("%s:%s", strings.TrimSpace(domain), strings.TrimSpace(pageID)))
}

// Warmup tải trước toàn bộ Landing Pages từ PostgreSQL vào RAM khi khởi động
func (c *LandingCache) Warmup(ctx context.Context) error {
	landings, err := c.store.GetAllLandingsWithCode(ctx)
	if err != nil {
		// Fallback nếu có lỗi
		landings, err = c.store.ListLandingPages()
		if err != nil {
			return err
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range landings {
		p := landings[i]
		key := makeKey(p.Domain, p.PageID)
		c.byDomainPage[key] = &p

		// ⚠️ `byPageID` là index phục vụ fallback "tìm landing theo page_id".
		//   `GetAllLandingsWithCode` sắp xếp `updated_at DESC` ⇒ phần tử ĐẦU là
		//   bản MỚI NHẤT. Nếu gán đè vô điều kiện, phần tử CUỐI (bản CŨ) sẽ
		//   thắng ⇒ server phục vụ code cũ, mất chỉnh sửa trong Code Studio.
		//
		//   ⇒ Chỉ gán khi CHƯA có ⇒ giữ bản mới nhất.
		//   (Lỗi này chỉ lộ ra khi 1 landing có nhiều row trong `landing_pages` —
		//   tức đúng trường hợp bị duplicate landing.)
		pageKey := strings.ToLower(p.PageID)
		if _, exists := c.byPageID[pageKey]; !exists {
			c.byPageID[pageKey] = &p
		}
	}

	log.Printf("[INFO] [CACHE WARMUP] Đã nạp %d cấu hình Landing Page vào bộ nhớ đệm RAM siêu tốc", len(landings))
	return nil
}

// Get lấy Landing Page từ cache theo domain và page_id. Nếu cache miss, query DB và cập nhật cache.
func (c *LandingCache) Get(domain, pageID string) *models.LandingPage {
	domain = strings.TrimSpace(domain)
	pageID = strings.TrimSpace(pageID)

	c.mu.RLock()
	key := makeKey(domain, pageID)
	if p, ok := c.byDomainPage[key]; ok {
		c.mu.RUnlock()
		return p
	}
	// Fallback tìm theo domain rỗng
	emptyDomainKey := makeKey("", pageID)
	if p, ok := c.byDomainPage[emptyDomainKey]; ok {
		c.mu.RUnlock()
		return p
	}
	// Fallback tìm theo page_id độc lập
	if p, ok := c.byPageID[strings.ToLower(pageID)]; ok {
		c.mu.RUnlock()
		return p
	}
	c.mu.RUnlock()

	// Cache miss -> Query DB
	p, err := c.store.GetLandingPage(domain, pageID)
	if err == nil && p != nil {
		c.Set(p)
		return p
	}

	pByID, err := c.store.GetLandingPageByID(pageID)
	if err == nil && pByID != nil {
		c.Set(pByID)
		return pByID
	}

	return nil
}

// GetByPageID lấy landing page trực tiếp theo pageID
func (c *LandingCache) GetByPageID(pageID string) *models.LandingPage {
	c.mu.RLock()
	if p, ok := c.byPageID[strings.ToLower(strings.TrimSpace(pageID))]; ok {
		c.mu.RUnlock()
		return p
	}
	c.mu.RUnlock()

	p, err := c.store.GetLandingPageByID(pageID)
	if err == nil && p != nil {
		c.Set(p)
		return p
	}
	return nil
}

// Set cập nhật hoặc ghi đè một Landing Page vào cache (In-memory Pub/Sub event)
func (c *LandingCache) Set(p *models.LandingPage) {
	if p == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	key := makeKey(p.Domain, p.PageID)
	c.byDomainPage[key] = p
	c.byPageID[strings.ToLower(p.PageID)] = p
	log.Printf("[INFO] [CACHE UPDATED] Đã cập nhật cache cho landing_id='%s', domain='%s'", p.PageID, p.Domain)
}

// Invalidate xóa cache của một landing page (kích hoạt khi Admin cập nhật cấu hình trên CRM)
func (c *LandingCache) Invalidate(domain, pageID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := makeKey(domain, pageID)
	delete(c.byDomainPage, key)
	emptyDomainKey := makeKey("", pageID)
	delete(c.byDomainPage, emptyDomainKey)
	delete(c.byPageID, strings.ToLower(strings.TrimSpace(pageID)))

	log.Printf("[INFO] [CACHE INVALIDATION] Đã xóa cache cho landing_id='%s', domain='%s'", pageID, domain)
}

// InvalidateAll xóa sạch toàn bộ cache và nạp lại từ DB
func (c *LandingCache) InvalidateAll() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = c.Warmup(ctx)
}
