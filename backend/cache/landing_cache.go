package cache

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
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
	// byPageIDRows lưu TẤT CẢ các row của 1 landing theo `page_id` (bản gốc
	// `domain=''` + các bản theo domain) để resolve cấu hình theo domain.
	//
	// ⚠️ BẮT BUỘC (fix 2026-11-13): `landing_pages` có PK composite
	// `(domain, page_id)` ⇒ 1 landing có thể tồn tại NHIỀU row. Cấu hình
	// `google_sheet_ids` / `google_sheet_sync_mode` thường CHỈ nằm ở bản theo
	// domain (bản gốc là template, không có 2 khoá này). Nếu chỉ đọc bản gốc
	// ⇒ `google_sheet_ids` rỗng ⇒ backend rơi vào nhánh "đồng bộ mặc định"
	// ⇒ lead bắn vào MỌI sheet đang active ⇒ lead của landing này bị tràn
	// sang sheet của landing khác.
	byPageIDRows map[string]map[string]*models.LandingPage
	store        *db.Store
}

// NewLandingCache khởi tạo bộ nhớ đệm
func NewLandingCache(store *db.Store) *LandingCache {
	c := &LandingCache{
		byDomainPage: make(map[string]*models.LandingPage),
		byPageID:     make(map[string]*models.LandingPage),
		byPageIDRows: make(map[string]map[string]*models.LandingPage),
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

	c.byDomainPage = make(map[string]*models.LandingPage, len(landings))
	c.byPageID = make(map[string]*models.LandingPage, len(landings))
	c.byPageIDRows = make(map[string]map[string]*models.LandingPage, len(landings))

	for i := range landings {
		p := landings[i]
		key := makeKey(p.Domain, p.PageID)
		c.byDomainPage[key] = &p

		pageKey := strings.ToLower(strings.TrimSpace(p.PageID))
		if c.byPageIDRows[pageKey] == nil {
			c.byPageIDRows[pageKey] = make(map[string]*models.LandingPage)
		}
		c.byPageIDRows[pageKey][strings.ToLower(strings.TrimSpace(p.Domain))] = &p

		// ⚠️ `byPageID` là index phục vụ fallback "tìm landing theo page_id".
		//   `GetAllLandingsWithCode` sắp xếp `updated_at DESC` ⇒ phần tử ĐẦU là
		//   bản MỚI NHẤT. Nếu gán đè vô điều kiện, phần tử CUỐI (bản CŨ) sẽ
		//   thắng ⇒ server phục vụ code cũ, mất chỉnh sửa trong Code Studio.
		//
		//   ⇒ Chỉ gán khi CHƯA có ⇒ giữ bản mới nhất.
		//   (Lỗi này chỉ lộ ra khi 1 landing có nhiều row trong `landing_pages` —
		//   tức đúng trường hợp bị duplicate landing.)
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

	pageKey := strings.ToLower(strings.TrimSpace(p.PageID))
	if c.byPageIDRows[pageKey] == nil {
		c.byPageIDRows[pageKey] = make(map[string]*models.LandingPage)
	}
	c.byPageIDRows[pageKey][strings.ToLower(strings.TrimSpace(p.Domain))] = p

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

	// Xoá row theo domain khỏi index nhiều row. Bản gốc (`domain=''`) và các
	// bản domain khác vẫn được giữ lại — chúng là nguồn cấu hình dự phòng.
	pageKey := strings.ToLower(strings.TrimSpace(pageID))
	if rows, ok := c.byPageIDRows[pageKey]; ok {
		delete(rows, strings.ToLower(strings.TrimSpace(domain)))
		if len(rows) == 0 {
			delete(c.byPageIDRows, pageKey)
		}
	}

	log.Printf("[INFO] [CACHE INVALIDATION] Đã xóa cache cho landing_id='%s', domain='%s'", pageID, domain)
}

// InvalidateAll xóa sạch toàn bộ cache và nạp lại từ DB
func (c *LandingCache) InvalidateAll() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = c.Warmup(ctx)
}

// GetSheetSyncSettings trả về cấu hình đồng bộ Google Sheets của 1 landing page.
//
// ⚠️ BỐI CẢNH (fix 2026-11-13 — lead bắn tràn sang nhiều sheet):
//   `landing_pages` có PK composite `(domain, page_id)` ⇒ 1 landing tồn tại ở
//   NHIỀU row: bản GỐC (`domain=''`, là template) + bản THEO DOMAIN.
//   Cấu hình `google_sheet_ids` / `google_sheet_sync_mode` thường chỉ nằm ở
//   bản theo domain.
//
//   Bug cũ: handler đọc `Get("", page_id)` ⇒ luôn ra bản GỐC ⇒ không có
//   `google_sheet_ids` ⇒ rơi vào nhánh "đồng bộ mặc định" ⇒ khớp MỌI sheet
//   active có `filter_landing_id = ''` ⇒ lead của landing A bị gửi sang sheet
//   của landing B.
//
// ⇒ Hàm này gộp cấu hình từ MỌI row của landing theo thứ tự ưu tiên:
//   1. row khớp `domain` (domain khách vừa gửi form)
//   2. row gốc (`domain = ''`)
//   3. row mới cập nhật nhất
//
// Trả về:
//   mode      — 'selected' | 'all' | 'disabled' | '' (chưa cấu hình)
//   sheetIDs  — danh sách id sheet được gán riêng
//   hasConfig — landing có cấu hình Google Sheets hay không
func (c *LandingCache) GetSheetSyncSettings(domain, pageID string) (mode string, sheetIDs []int, hasConfig bool) {
	pageKey := strings.ToLower(strings.TrimSpace(pageID))
	domainKey := strings.ToLower(strings.TrimSpace(domain))

	c.mu.RLock()
	rows, ok := c.byPageIDRows[pageKey]
	var candidates []*models.LandingPage
	if ok {
		// Ưu tiên 1: row khớp domain
		if p, ok2 := rows[domainKey]; ok2 {
			candidates = append(candidates, p)
		}
		// Ưu tiên 2: row gốc (domain = '')
		if domainKey != "" {
			if p, ok2 := rows[""]; ok2 {
				candidates = append(candidates, p)
			}
		}
		// Ưu tiên 3: các row còn lại, sắp xếp `updated_at` mới nhất trước
		rest := make([]*models.LandingPage, 0, len(rows))
		for d, p := range rows {
			if d == domainKey || d == "" {
				continue
			}
			rest = append(rest, p)
		}
		sort.SliceStable(rest, func(i, j int) bool {
			return rest[i].UpdatedAt > rest[j].UpdatedAt
		})
		candidates = append(candidates, rest...)
	}
	c.mu.RUnlock()

	if len(candidates) == 0 {
		return "", nil, false
	}

	for _, p := range candidates {
		if p == nil || p.Settings == nil {
			continue
		}

		if !hasConfig {
			if _, ok := p.Settings["google_sheet_sync_mode"]; ok {
				hasConfig = true
			}
			if _, ok := p.Settings["google_sheet_ids"]; ok {
				hasConfig = true
			}
		}

		if mode == "" {
			if m, ok := p.Settings["google_sheet_sync_mode"].(string); ok && strings.TrimSpace(m) != "" {
				mode = strings.TrimSpace(m)
			}
		}

		if sheetIDs == nil {
			if ids, ok := extractSheetIDs(p.Settings["google_sheet_ids"]); ok {
				sheetIDs = ids
			}
		}
	}

	// Có danh sách sheet nhưng thiếu mode ⇒ coi như 'selected' (không đồng bộ
	// vào mọi sheet).
	if mode == "" && len(sheetIDs) > 0 {
		mode = "selected"
	}

	return mode, sheetIDs, hasConfig
}

// extractSheetIDs đọc `google_sheet_ids` từ settings (JSONB) ra []int.
// Trả về `ok = false` nếu khoá không tồn tại (chưa cấu hình).
func extractSheetIDs(raw interface{}) ([]int, bool) {
	switch t := raw.(type) {
	case []interface{}:
		out := make([]int, 0, len(t))
		for _, item := range t {
			switch n := item.(type) {
			case float64:
				out = append(out, int(n))
			case int:
				out = append(out, n)
			case int64:
				out = append(out, int(n))
			case string:
				if parsed, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
					out = append(out, parsed)
				}
			}
		}
		return out, true
	case []int:
		return t, true
	case []string:
		out := make([]int, 0, len(t))
		for _, s := range t {
			if parsed, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
				out = append(out, parsed)
			}
		}
		return out, true
	}
	return nil, false
}
