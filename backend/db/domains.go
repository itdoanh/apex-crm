package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"apex-backend/models"
)

// SeedDefaultDomains tạo tên miền và định tuyến mẫu ban đầu nếu CSDL chưa có
func (s *Store) SeedDefaultDomains(ctx context.Context) error {
	var count int
	err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM domains").Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	// 1. Thêm domain chính hanghoaphaisinh.net
	queryDomain := `
		INSERT INTO domains (domain, ns1, ns2, target_ip, dns_status, ssl_status, default_landing_id, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id;
	`
	var domainID int
	err = s.pool.QueryRow(ctx, queryDomain,
		"hanghoaphaisinh.net",
		"ns1.hanghoaphaisinh.net",
		"ns2.hanghoaphaisinh.net",
		"103.124.95.232",
		"active",
		"active",
		"mxv_080_online",
		"Tên miền chính thức của hệ thống Hàng Hóa Phái Sinh",
	).Scan(&domainID)
	if err != nil {
		return fmt.Errorf("lỗi seed domain chính: %w", err)
	}

	// 2. Thêm các routes mặc định
	queryRoute := `
		INSERT INTO domain_routes (domain_id, path_prefix, landing_id, is_active)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (domain_id, path_prefix) DO NOTHING;
	`
	// Route gốc: / -> mxv_080_online
	_, _ = s.pool.Exec(ctx, queryRoute, domainID, "/", "mxv_080_online", true)

	// Route thư mục: /khoa-hoc-dau-tu -> ld_dautu_01
	_, _ = s.pool.Exec(ctx, queryRoute, domainID, "/khoa-hoc-dau-tu", "ld_dautu_01", true)

	// 3. Thêm 1 domain phụ mẫu (dautuhanghoa.vn) để tiện thử nghiệm
	var d2ID int
	_ = s.pool.QueryRow(ctx, queryDomain,
		"dautuhanghoa.vn",
		"ns1.hanghoaphaisinh.net",
		"ns2.hanghoaphaisinh.net",
		"103.124.95.232",
		"pending",
		"active",
		"mxv_080_online",
		"Tên miền chạy chiến dịch quảng cáo Google & Facebook Ads",
	).Scan(&d2ID)
	if d2ID > 0 {
		_, _ = s.pool.Exec(ctx, queryRoute, d2ID, "/", "mxv_080_online", true)
		_, _ = s.pool.Exec(ctx, queryRoute, d2ID, "/ads", "mxv_080_online", true)
	}

	return nil
}

// ListDomains lấy danh sách tất cả các tên miền kèm theo các route/thư mục đã gán
func (s *Store) ListDomains() ([]models.Domain, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT
			id, domain, ns1, ns2, target_ip, dns_status, ssl_status,
			default_landing_id, notes,
			to_char(created_at, 'YYYY-MM-DD HH24:MI:SS') AS created_at,
			to_char(updated_at, 'YYYY-MM-DD HH24:MI:SS') AS updated_at
		FROM domains
		ORDER BY id ASC;
	`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("lỗi truy vấn danh sách domain: %w", err)
	}
	defer rows.Close()

	domains := make([]models.Domain, 0)
	for rows.Next() {
		var d models.Domain
		if err := rows.Scan(
			&d.ID,
			&d.Domain,
			&d.NS1,
			&d.NS2,
			&d.TargetIP,
			&d.DNSStatus,
			&d.SSLStatus,
			&d.DefaultLandingID,
			&d.Notes,
			&d.CreatedAt,
			&d.UpdatedAt,
		); err != nil {
			return nil, err
		}
		domains = append(domains, d)
	}

	// Lấy routes cho từng domain
	for i := range domains {
		routes, err := s.ListDomainRoutes(domains[i].ID)
		if err == nil {
			domains[i].Routes = routes
		}
	}

	return domains, nil
}

// GetDomain lấy chi tiết 1 tên miền theo ID
func (s *Store) GetDomain(id int) (*models.Domain, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT
			id, domain, ns1, ns2, target_ip, dns_status, ssl_status,
			default_landing_id, notes,
			to_char(created_at, 'YYYY-MM-DD HH24:MI:SS') AS created_at,
			to_char(updated_at, 'YYYY-MM-DD HH24:MI:SS') AS updated_at
		FROM domains
		WHERE id = $1;
	`
	var d models.Domain
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&d.ID,
		&d.Domain,
		&d.NS1,
		&d.NS2,
		&d.TargetIP,
		&d.DNSStatus,
		&d.SSLStatus,
		&d.DefaultLandingID,
		&d.Notes,
		&d.CreatedAt,
		&d.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	routes, _ := s.ListDomainRoutes(d.ID)
	d.Routes = routes
	return &d, nil
}

// GetDomainByName lấy chi tiết theo tên miền (không phân biệt hoa thường)
func (s *Store) GetDomainByName(domainName string) (*models.Domain, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cleanName := strings.ToLower(strings.TrimSpace(domainName))
	query := `
		SELECT
			id, domain, ns1, ns2, target_ip, dns_status, ssl_status,
			default_landing_id, notes,
			to_char(created_at, 'YYYY-MM-DD HH24:MI:SS') AS created_at,
			to_char(updated_at, 'YYYY-MM-DD HH24:MI:SS') AS updated_at
		FROM domains
		WHERE LOWER(domain) = $1;
	`
	var d models.Domain
	err := s.pool.QueryRow(ctx, query, cleanName).Scan(
		&d.ID,
		&d.Domain,
		&d.NS1,
		&d.NS2,
		&d.TargetIP,
		&d.DNSStatus,
		&d.SSLStatus,
		&d.DefaultLandingID,
		&d.Notes,
		&d.CreatedAt,
		&d.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	routes, _ := s.ListDomainRoutes(d.ID)
	d.Routes = routes
	return &d, nil
}

// SaveDomain tạo mới hoặc cập nhật thông tin tên miền
func (s *Store) SaveDomain(d *models.Domain) (*models.Domain, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d.Domain = strings.ToLower(strings.TrimSpace(d.Domain))
	if d.NS1 == "" {
		d.NS1 = "ns1.hanghoaphaisinh.net"
	}
	if d.NS2 == "" {
		d.NS2 = "ns2.hanghoaphaisinh.net"
	}
	if d.TargetIP == "" {
		d.TargetIP = "103.124.95.232"
	}
	if d.DNSStatus == "" {
		d.DNSStatus = "pending"
	}
	if d.SSLStatus == "" {
		d.SSLStatus = "active"
	}
	if d.DefaultLandingID == "" {
		d.DefaultLandingID = "mxv_080_online"
	}

	if d.ID > 0 {
		query := `
			UPDATE domains
			SET domain = $1, ns1 = $2, ns2 = $3, target_ip = $4,
			    default_landing_id = $5, notes = $6, updated_at = NOW()
			WHERE id = $7
			RETURNING to_char(updated_at, 'YYYY-MM-DD HH24:MI:SS');
		`
		err := s.pool.QueryRow(ctx, query,
			d.Domain, d.NS1, d.NS2, d.TargetIP, d.DefaultLandingID, d.Notes, d.ID,
		).Scan(&d.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("lỗi cập nhật domain: %w", err)
		}
	} else {
		query := `
			INSERT INTO domains (domain, ns1, ns2, target_ip, dns_status, ssl_status, default_landing_id, notes)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (domain) DO UPDATE
			SET ns1 = EXCLUDED.ns1,
			    ns2 = EXCLUDED.ns2,
			    target_ip = EXCLUDED.target_ip,
			    default_landing_id = EXCLUDED.default_landing_id,
			    notes = EXCLUDED.notes,
			    updated_at = NOW()
			RETURNING id, to_char(created_at, 'YYYY-MM-DD HH24:MI:SS'), to_char(updated_at, 'YYYY-MM-DD HH24:MI:SS');
		`
		err := s.pool.QueryRow(ctx, query,
			d.Domain, d.NS1, d.NS2, d.TargetIP, d.DNSStatus, d.SSLStatus, d.DefaultLandingID, d.Notes,
		).Scan(&d.ID, &d.CreatedAt, &d.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("lỗi thêm domain mới: %w", err)
		}

		// Tự động tạo route mặc định '/' cho default_landing_id
		_, _ = s.DeployDomainRoute(d.ID, "/", d.DefaultLandingID, "")
	}

	return s.GetDomain(d.ID)
}

// UpdateDomainDNSStatus cập nhật trạng thái DNS sau khi kiểm tra
func (s *Store) UpdateDomainDNSStatus(domainID int, dnsStatus, sslStatus string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		UPDATE domains
		SET dns_status = $1, ssl_status = $2, updated_at = NOW()
		WHERE id = $3;
	`
	_, err := s.pool.Exec(ctx, query, dnsStatus, sslStatus, domainID)
	return err
}

// DeleteDomain xóa tên miền và tất cả các route gắn liền
func (s *Store) DeleteDomain(id int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.pool.Exec(ctx, "DELETE FROM domains WHERE id = $1", id)
	return err
}

// ListDomainRoutes danh sách routes của 1 domain
func (s *Store) ListDomainRoutes(domainID int) ([]models.DomainRoute, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT
			dr.id, dr.domain_id, d.domain, dr.path_prefix, dr.landing_id,
			COALESCE(lp.name, dr.landing_id) AS landing_name,
			dr.is_active, dr.redirect_url,
			to_char(dr.deployed_at, 'YYYY-MM-DD HH24:MI:SS') AS deployed_at
		FROM domain_routes dr
		JOIN domains d ON d.id = dr.domain_id
		LEFT JOIN landing_pages lp ON lp.page_id = dr.landing_id
		WHERE dr.domain_id = $1
		ORDER BY LENGTH(dr.path_prefix) DESC, dr.path_prefix ASC;
	`
	rows, err := s.pool.Query(ctx, query, domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	routes := make([]models.DomainRoute, 0)
	for rows.Next() {
		var r models.DomainRoute
		if err := rows.Scan(
			&r.ID,
			&r.DomainID,
			&r.Domain,
			&r.PathPrefix,
			&r.LandingID,
			&r.LandingName,
			&r.IsActive,
			&r.RedirectURL,
			&r.DeployedAt,
		); err != nil {
			return nil, err
		}
		routes = append(routes, r)
	}
	return routes, nil
}

// DeployDomainRoute triển khai hoặc cập nhật route (thư mục/đường dẫn) cho landing page
func (s *Store) DeployDomainRoute(domainID int, pathPrefix, landingID, redirectURL string) (*models.DomainRoute, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Chuẩn hóa pathPrefix: luôn bắt đầu bằng '/' và loại bỏ dấu '/' ở cuối (trừ khi là '/')
	pathPrefix = strings.TrimSpace(pathPrefix)
	if !strings.HasPrefix(pathPrefix, "/") {
		pathPrefix = "/" + pathPrefix
	}
	if len(pathPrefix) > 1 {
		pathPrefix = strings.TrimRight(pathPrefix, "/")
	}

	landingID = strings.TrimSpace(landingID)
	if landingID == "" {
		landingID = "mxv_080_online"
	}

	query := `
		INSERT INTO domain_routes (domain_id, path_prefix, landing_id, is_active, redirect_url, deployed_at)
		VALUES ($1, $2, $3, true, $4, NOW())
		ON CONFLICT (domain_id, path_prefix)
		DO UPDATE SET
			landing_id = EXCLUDED.landing_id,
			is_active = true,
			redirect_url = EXCLUDED.redirect_url,
			deployed_at = NOW()
		RETURNING id, to_char(deployed_at, 'YYYY-MM-DD HH24:MI:SS');
	`
	var r models.DomainRoute
	r.DomainID = domainID
	r.PathPrefix = pathPrefix
	r.LandingID = landingID
	r.IsActive = true
	r.RedirectURL = redirectURL

	err := s.pool.QueryRow(ctx, query, domainID, pathPrefix, landingID, redirectURL).Scan(&r.ID, &r.DeployedAt)
	if err != nil {
		return nil, fmt.Errorf("lỗi deploy route: %w", err)
	}

	return &r, nil
}

// DeleteDomainRoute gỡ bỏ một route
func (s *Store) DeleteDomainRoute(routeID int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.pool.Exec(ctx, "DELETE FROM domain_routes WHERE id = $1", routeID)
	return err
}

// GetAllActiveDomainRoutes lấy tất cả các routes đang hoạt động trên toàn hệ thống để nạp vào RAM cache
func (s *Store) GetAllActiveDomainRoutes() ([]models.DomainRoute, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT
			dr.id, dr.domain_id, LOWER(d.domain) AS domain, dr.path_prefix, dr.landing_id,
			COALESCE(lp.name, dr.landing_id) AS landing_name,
			dr.is_active, dr.redirect_url,
			to_char(dr.deployed_at, 'YYYY-MM-DD HH24:MI:SS') AS deployed_at
		FROM domain_routes dr
		JOIN domains d ON d.id = dr.domain_id
		LEFT JOIN landing_pages lp ON lp.page_id = dr.landing_id
		WHERE dr.is_active = true
		ORDER BY LENGTH(dr.path_prefix) DESC;
	`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	routes := make([]models.DomainRoute, 0)
	for rows.Next() {
		var r models.DomainRoute
		if err := rows.Scan(
			&r.ID,
			&r.DomainID,
			&r.Domain,
			&r.PathPrefix,
			&r.LandingID,
			&r.LandingName,
			&r.IsActive,
			&r.RedirectURL,
			&r.DeployedAt,
		); err != nil {
			return nil, err
		}
		routes = append(routes, r)
	}
	return routes, nil
}

// GetRouteByLandingID lấy route gần nhất gắn với landing page
func (s *Store) GetRouteByLandingID(landingID string) (*models.DomainRoute, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT
			dr.id, dr.domain_id, LOWER(d.domain) AS domain, dr.path_prefix, dr.landing_id,
			COALESCE(lp.name, dr.landing_id) AS landing_name,
			dr.is_active, dr.redirect_url,
			to_char(dr.deployed_at, 'YYYY-MM-DD HH24:MI:SS') AS deployed_at
		FROM domain_routes dr
		JOIN domains d ON d.id = dr.domain_id
		LEFT JOIN landing_pages lp ON lp.page_id = dr.landing_id
		WHERE dr.landing_id = $1
		ORDER BY dr.deployed_at DESC
		LIMIT 1;
	`
	var r models.DomainRoute
	err := s.pool.QueryRow(ctx, query, landingID).Scan(
		&r.ID, &r.DomainID, &r.Domain, &r.PathPrefix, &r.LandingID,
		&r.LandingName, &r.IsActive, &r.RedirectURL, &r.DeployedAt,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// GetRouteByDomainAndLandingID lấy route gắn với landing page trên một tên miền cụ thể
func (s *Store) GetRouteByDomainAndLandingID(domainID int, landingID string) (*models.DomainRoute, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT
			dr.id, dr.domain_id, LOWER(d.domain) AS domain, dr.path_prefix, dr.landing_id,
			COALESCE(lp.name, dr.landing_id) AS landing_name,
			dr.is_active, dr.redirect_url,
			to_char(dr.deployed_at, 'YYYY-MM-DD HH24:MI:SS') AS deployed_at
		FROM domain_routes dr
		JOIN domains d ON d.id = dr.domain_id
		LEFT JOIN landing_pages lp ON lp.page_id = dr.landing_id
		WHERE dr.domain_id = $1 AND dr.landing_id = $2
		ORDER BY dr.deployed_at DESC
		LIMIT 1;
	`
	var r models.DomainRoute
	err := s.pool.QueryRow(ctx, query, domainID, landingID).Scan(
		&r.ID, &r.DomainID, &r.Domain, &r.PathPrefix, &r.LandingID,
		&r.LandingName, &r.IsActive, &r.RedirectURL, &r.DeployedAt,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}
