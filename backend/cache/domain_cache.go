package cache

import (
	"context"
	"sort"
	"strings"
	"sync"

	"apex-backend/db"
	"apex-backend/models"
)

// DomainCache quản lý bộ nhớ đệm RAM định tuyến đa tên miền siêu tốc (<0.05ms)
type DomainCache struct {
	mu     sync.RWMutex
	store  *db.Store
	routes map[string][]models.DomainRoute // map[domain] -> danh sách routes (sắp xếp dài -> ngắn)
}

// NewDomainCache khởi tạo cache
func NewDomainCache(store *db.Store) *DomainCache {
	return &DomainCache{
		store:  store,
		routes: make(map[string][]models.DomainRoute),
	}
}

// Warmup nạp toàn bộ cấu hình routes từ CSDL vào RAM
func (c *DomainCache) Warmup(ctx context.Context) error {
	routes, err := c.store.GetAllActiveDomainRoutes()
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.routes = make(map[string][]models.DomainRoute)
	for _, r := range routes {
		dom := strings.ToLower(strings.TrimSpace(r.Domain))
		c.routes[dom] = append(c.routes[dom], r)
	}

	// Sắp xếp routes của từng domain theo độ dài PathPrefix giảm dần (Longest Prefix Match)
	// Đảm bảo route gốc '/' luôn được đánh giá ở cuối cùng
	for dom := range c.routes {
		sort.SliceStable(c.routes[dom], func(i, j int) bool {
			pi := c.routes[dom][i].PathPrefix
			pj := c.routes[dom][j].PathPrefix
			if pi == "/" {
				return false
			}
			if pj == "/" {
				return true
			}
			return len(pi) > len(pj)
		})
	}

	return nil
}

// Reload nạp lại cache từ DB
func (c *DomainCache) Reload(ctx context.Context) error {
	return c.Warmup(ctx)
}

// NormalizeHost loại bỏ port và tiền tố www.
func NormalizeHost(host string) string {
	h := strings.ToLower(strings.TrimSpace(host))
	// Cắt port nếu có (vd: localhost:8080 -> localhost)
	if idx := strings.Index(h, ":"); idx != -1 {
		h = h[:idx]
	}
	// Chuẩn hóa www.
	h = strings.TrimPrefix(h, "www.")
	return h
}

// HasDomain kiểm tra nhanh trong RAM cache xem domain có tồn tại trong hệ thống hay không (cho ACME SSL on-demand)
func (c *DomainCache) HasDomain(host string) bool {
	normHost := NormalizeHost(host)
	c.mu.RLock()
	defer c.mu.RUnlock()

	if _, exists := c.routes[normHost]; exists {
		return true
	}
	parts := strings.Split(normHost, ".")
	if len(parts) > 2 {
		parentDomain := strings.Join(parts[len(parts)-2:], ".")
		if _, exists := c.routes[parentDomain]; exists {
			return true
		}
	}
	return false
}

// ResolveRoute giải quyết nhanh định tuyến theo Host và URL Path:
// Trả về:
// - landingID: ID của landing page tương ứng
// - pathPrefix: Tiền tố đường dẫn khớp (vd: '/', '/khoa-hoc-dau-tu')
// - subPath: Phần đường dẫn còn lại (vd: '', 'index.html', 'tracker.js', 'style.css')
// - redirectURL: URL chuyển hướng nếu có
// - found: bool
func (c *DomainCache) ResolveRoute(host, reqPath string) (landingID, pathPrefix, subPath, redirectURL string, found bool) {
	normHost := NormalizeHost(host)
	cleanPath := strings.TrimSpace(reqPath)
	if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	// 1. Tìm theo tên miền chính xác
	routes, exists := c.routes[normHost]
	if !exists {
		// Thử tìm theo domain không có subdomain nếu normHost là subdomain
		parts := strings.Split(normHost, ".")
		if len(parts) > 2 {
			parentDomain := strings.Join(parts[len(parts)-2:], ".")
			routes, exists = c.routes[parentDomain]
		}
	}

	if !exists || len(routes) == 0 {
		return "", "", "", "", false
	}

	// 2. Duyệt qua các routes của domain này (đã được sắp xếp dài nhất trước)
	for _, r := range routes {
		if !r.IsActive {
			continue
		}

		pfx := r.PathPrefix
		if pfx == "/" {
			// Route gốc '/'
			if cleanPath == "/" || cleanPath == "" || cleanPath == "/index.html" {
				return r.LandingID, "/", "", r.RedirectURL, true
			}
			// Nếu là sub-resource tại root (vd: /tracker.js, /chucmung.html, /style.css)
			sub := strings.TrimPrefix(cleanPath, "/")
			return r.LandingID, "/", sub, r.RedirectURL, true
		}

		// Route thư mục con (vd: /khoa-hoc-dau-tu)
		if cleanPath == pfx || cleanPath == pfx+"/" || cleanPath == pfx+"/index.html" {
			return r.LandingID, pfx, "", r.RedirectURL, true
		}

		if strings.HasPrefix(cleanPath, pfx+"/") {
			sub := strings.TrimPrefix(cleanPath, pfx+"/")
			return r.LandingID, pfx, sub, r.RedirectURL, true
		}
	}

	return "", "", "", "", false
}

// ResolveChucMungRoute tìm landing phục vụ trang chúc mừng (`/chucmung.html`)
// cho một host.
//
// BỐI CẢNH (bug chồng chéo landing — 2026-11-05):
// Route gốc `/` của một domain có thể trỏ tới landing KHÔNG tồn tại (dangling route).
// Khi đó `/chucmung.html` ở root sẽ không có landing context ⇒ tracker.js rơi về
// fallback `mxv_080_online` ⇒ analytics bị ghi nhận nhầm landing.
//
// Hàm này tìm route có path_prefix chứa "chucmung" của host (vd: `/chucmung` →
// `landing_lznoew`) để làm fallback chính xác.
func (c *DomainCache) ResolveChucMungRoute(host string) (landingID string, found bool) {
	normHost := NormalizeHost(host)

	c.mu.RLock()
	defer c.mu.RUnlock()

	routes, exists := c.routes[normHost]
	if !exists {
		parts := strings.Split(normHost, ".")
		if len(parts) > 2 {
			parentDomain := strings.Join(parts[len(parts)-2:], ".")
			routes, exists = c.routes[parentDomain]
		}
	}
	if !exists {
		return "", false
	}

	for _, r := range routes {
		if !r.IsActive {
			continue
		}
		if strings.Contains(strings.ToLower(r.PathPrefix), "chucmung") && r.LandingID != "" {
			return r.LandingID, true
		}
	}
	return "", false
}
