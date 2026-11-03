package handlers

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"apex-backend/cache"
	"apex-backend/config"
	"apex-backend/db"
	"apex-backend/models"
	"apex-backend/security"
)

// DomainHandler xử lý các API quản lý tên miền và DNS routing
type DomainHandler struct {
	cfg         *config.Config
	store       *db.Store
	domainCache *cache.DomainCache
}

// NewDomainHandler khởi tạo handler
func NewDomainHandler(cfg *config.Config, store *db.Store, domainCache *cache.DomainCache) *DomainHandler {
	return &DomainHandler{
		cfg:         cfg,
		store:       store,
		domainCache: domainCache,
	}
}

// HandleListDomains trả về danh sách tất cả các tên miền và routes
func (h *DomainHandler) HandleListDomains(w http.ResponseWriter, r *http.Request) {
	domains, err := h.store.ListDomains()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    domains,
		"total":   len(domains),
		"system_info": map[string]interface{}{
			"ns1":       "ns1.hanghoaphaisinh.net",
			"ns2":       "ns2.hanghoaphaisinh.net",
			"target_ip": "103.124.95.232",
		},
	})
}

// HandleSaveDomain thêm mới hoặc chỉnh sửa tên miền
func (h *DomainHandler) HandleSaveDomain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var d models.Domain
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		http.Error(w, "Payload JSON không hợp lệ", http.StatusBadRequest)
		return
	}

	d.Domain = strings.ToLower(strings.TrimSpace(d.Domain))
	// Loại bỏ tiền tố http://, https://, và dấu gạch chéo
	d.Domain = strings.TrimPrefix(d.Domain, "https://")
	d.Domain = strings.TrimPrefix(d.Domain, "http://")
	d.Domain = strings.TrimRight(d.Domain, "/")
	if idx := strings.Index(d.Domain, "/"); idx != -1 {
		d.Domain = d.Domain[:idx]
	}

	if d.Domain == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Tên miền không được để trống"})
		return
	}

	saved, err := h.store.SaveDomain(&d)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	// Làm mới RAM Cache
	_ = h.domainCache.Reload(context.Background())

	// Ghi nhận Audit Log
	clientIP := ExtractClientIP(r)
	ua := r.UserAgent()
	deviceType, browser, osName := security.ParseUserAgent(ua)
	var adminID *int
	var username string
	if admin, ok := r.Context().Value(AdminContextKey).(*models.AdminUser); ok && admin != nil {
		adminID = &admin.ID
		username = admin.Username
	}
	_ = h.store.RecordAuditLog(&models.AdminAuditLog{
		AdminID:    adminID,
		Username:   username,
		Action:     "SAVE_DOMAIN",
		Resource:   "domains",
		ResourceID: d.Domain,
		Status:     "success",
		Details:    map[string]interface{}{"domain": d.Domain, "target_ip": d.TargetIP, "default_landing": d.DefaultLandingID, "ns1": d.NS1, "ns2": d.NS2},
		IPAddress:  clientIP,
		UserAgent:  ua,
		DeviceType: deviceType,
		Browser:    browser,
		OS:         osName,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Đã lưu tên miền thành công",
		"data":    saved,
	})
}

// HandleDeleteDomain xóa tên miền
func (h *DomainHandler) HandleDeleteDomain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID int `json:"id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if req.ID <= 0 {
		if idStr := r.URL.Query().Get("id"); idStr != "" {
			req.ID, _ = strconv.Atoi(idStr)
		}
	}

	if req.ID <= 0 {
		http.Error(w, "Thiếu id tên miền", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteDomain(req.ID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	// Làm mới RAM Cache
	_ = h.domainCache.Reload(context.Background())

	// Ghi nhận Audit Log
	clientIP := ExtractClientIP(r)
	ua := r.UserAgent()
	deviceType, browser, osName := security.ParseUserAgent(ua)
	var adminID *int
	var username string
	if admin, ok := r.Context().Value(AdminContextKey).(*models.AdminUser); ok && admin != nil {
		adminID = &admin.ID
		username = admin.Username
	}
	_ = h.store.RecordAuditLog(&models.AdminAuditLog{
		AdminID:    adminID,
		Username:   username,
		Action:     "DELETE_DOMAIN",
		Resource:   "domains",
		ResourceID: strconv.Itoa(req.ID),
		Status:     "success",
		Details:    map[string]interface{}{"domain_id": req.ID},
		IPAddress:  clientIP,
		UserAgent:  ua,
		DeviceType: deviceType,
		Browser:    browser,
		OS:         osName,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Đã xóa tên miền",
	})
}

// HandleCheckDNS kiểm tra bản ghi DNS thực tế của tên miền qua mạng
func (h *DomainHandler) HandleCheckDNS(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DomainID int    `json:"domain_id"`
		Domain   string `json:"domain"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	var targetDomain *models.Domain
	var err error

	if req.DomainID > 0 {
		targetDomain, err = h.store.GetDomain(req.DomainID)
	} else if req.Domain != "" {
		targetDomain, err = h.store.GetDomainByName(req.Domain)
	}

	if err != nil || targetDomain == nil {
		// Tạo đối tượng tạm nếu chưa lưu vào DB
		if req.Domain != "" {
			targetDomain = &models.Domain{
				Domain:   strings.ToLower(strings.TrimSpace(req.Domain)),
				NS1:      "ns1.hanghoaphaisinh.net",
				NS2:      "ns2.hanghoaphaisinh.net",
				TargetIP: "103.124.95.232",
			}
		} else {
			http.Error(w, "Không tìm thấy tên miền để kiểm tra", http.StatusBadRequest)
			return
		}
	}

	res := models.DNSCheckResult{
		Domain:      targetDomain.Domain,
		ExpectedNS:  []string{targetDomain.NS1, targetDomain.NS2},
		ExpectedIP:  targetDomain.TargetIP,
		FoundNS:     make([]string, 0),
		FoundIPs:    make([]string, 0),
		CheckedAt:   time.Now().Format("2006-01-02 15:04:05"),
		Status:      "pending",
	}

	// 1. Tra cứu bản ghi NameServer (NS)
	nsRecords, err := net.LookupNS(targetDomain.Domain)
	if err == nil {
		for _, ns := range nsRecords {
			host := strings.ToLower(strings.TrimSuffix(ns.Host, "."))
			res.FoundNS = append(res.FoundNS, host)
			if strings.Contains(host, "hanghoaphaisinh.net") || host == strings.ToLower(targetDomain.NS1) || host == strings.ToLower(targetDomain.NS2) {
				res.IsNSMatched = true
			}
		}
	}

	// 2. Tra cứu bản ghi IP (A record)
	ipRecords, err := net.LookupIP(targetDomain.Domain)
	if err == nil {
		for _, ip := range ipRecords {
			ipStr := ip.String()
			res.FoundIPs = append(res.FoundIPs, ipStr)
			if ipStr == targetDomain.TargetIP || strings.HasPrefix(ipStr, "127.") || ipStr == "::1" {
				res.IsIPMatched = true
			}
		}
	}

	// 3. Đánh giá trạng thái
	if res.IsNSMatched || res.IsIPMatched || strings.Contains(targetDomain.Domain, "localhost") {
		res.Status = "active"
		res.Message = "Tên miền đã trỏ DNS thành công! Sẵn sàng phục vụ Landing Page và nhận Leads."
	} else if len(res.FoundNS) > 0 || len(res.FoundIPs) > 0 {
		res.Status = "pending"
		res.Message = "Tên miền chưa trỏ về NS hoặc IP của hệ thống. Vui lòng vào trang quản trị tên miền để cấu hình trỏ về ns1.hanghoaphaisinh.net / ns2.hanghoaphaisinh.net."
	} else {
		res.Status = "error"
		res.Message = "Chưa tìm thấy bản ghi DNS nào cho tên miền này. Vui lòng kiểm tra lại tên miền hoặc chờ thời gian lan truyền DNS (thường từ 5 - 15 phút)."
	}

	// 4. Nếu domain đã có trong DB, cập nhật trạng thái
	if targetDomain.ID > 0 {
		sslStatus := "dns_error"
		if res.Status == "active" {
			sslStatus = "pending"
		}
		_ = h.store.UpdateDomainDNSStatus(targetDomain.ID, res.Status, sslStatus)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    res,
	})
}

// HandleCheckDomainAllowed phục vụ endpoint kiểm tra quyền cấp SSL On-Demand cho Caddy hoặc CertMagic (ask endpoint)
// Trả về HTTP 200 OK nếu domain hợp lệ để cấp SSL tự động, hoặc HTTP 403 Forbidden nếu không được phép
func (h *DomainHandler) HandleCheckDomainAllowed(w http.ResponseWriter, r *http.Request) {
	domain := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("domain")))
	if domain == "" {
		http.Error(w, "Thiếu tham số domain", http.StatusBadRequest)
		return
	}

	// Cắt bỏ port nếu có
	if idx := strings.Index(domain, ":"); idx != -1 {
		domain = domain[:idx]
	}
	domain = strings.TrimPrefix(domain, "www.")

	// 1. Cho phép các domain hệ thống mặc định
	if domain == "localhost" ||
		domain == "127.0.0.1" ||
		domain == "crm.hanghoaphaisinh.net" ||
		domain == "hanghoaphaisinh.net" ||
		domain == "ns1.hanghoaphaisinh.net" ||
		domain == "ns2.hanghoaphaisinh.net" ||
		strings.HasSuffix(domain, ".hanghoaphaisinh.net") {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
		return
	}

	// 2. Tra cứu trong RAM Cache siêu tốc (<0.05ms)
	if h.domainCache != nil && h.domainCache.HasDomain(domain) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
		return
	}

	// 3. Fallback tra cứu CSDL
	if d, err := h.store.GetDomainByName(domain); err == nil && d != nil {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
		return
	}

	// Từ chối cấp SSL cho domain lạ chưa được thêm vào CRM để chống cạn kiệt Let's Encrypt rate limit
	http.Error(w, "Domain not registered in system", http.StatusForbidden)
}


// HandleDeployRoute triển khai landing page lên domain + path_prefix
func (h *DomainHandler) HandleDeployRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DomainID    int    `json:"domain_id"`
		Domain      string `json:"domain"`
		PathPrefix  string `json:"path_prefix"`
		LandingID   string `json:"landing_id"`
		RedirectURL string `json:"redirect_url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload JSON không hợp lệ", http.StatusBadRequest)
		return
	}

	if req.DomainID <= 0 && req.Domain != "" {
		dom, err := h.store.GetDomainByName(req.Domain)
		if err == nil && dom != nil {
			req.DomainID = dom.ID
		}
	}

	if req.DomainID <= 0 {
		http.Error(w, "Không tìm thấy tên miền để triển khai", http.StatusBadRequest)
		return
	}

	if req.LandingID == "" {
		req.LandingID = "mxv_080_online"
	}
	if req.PathPrefix == "" {
		req.PathPrefix = "/"
	}

	route, err := h.store.DeployDomainRoute(req.DomainID, req.PathPrefix, req.LandingID, req.RedirectURL)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	// Làm mới RAM Cache ngay lập tức
	_ = h.domainCache.Reload(context.Background())

	// Ghi nhận Audit Log
	clientIP := ExtractClientIP(r)
	ua := r.UserAgent()
	deviceType, browser, osName := security.ParseUserAgent(ua)
	var adminID *int
	var username string
	if admin, ok := r.Context().Value(AdminContextKey).(*models.AdminUser); ok && admin != nil {
		adminID = &admin.ID
		username = admin.Username
	}
	_ = h.store.RecordAuditLog(&models.AdminAuditLog{
		AdminID:    adminID,
		Username:   username,
		Action:     "DEPLOY_DOMAIN_ROUTE",
		Resource:   "domain_routes",
		ResourceID: req.PathPrefix,
		Status:     "success",
		Details:    map[string]interface{}{"domain_id": req.DomainID, "path_prefix": req.PathPrefix, "landing_id": req.LandingID, "redirect_url": req.RedirectURL},
		IPAddress:  clientIP,
		UserAgent:  ua,
		DeviceType: deviceType,
		Browser:    browser,
		OS:         osName,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Đã triển khai Landing Page lên tên miền thành công",
		"data":    route,
	})
}

// HandleDeleteRoute gỡ bỏ một route khỏi tên miền
func (h *DomainHandler) HandleDeleteRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		RouteID int `json:"route_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if req.RouteID <= 0 {
		if idStr := r.URL.Query().Get("id"); idStr != "" {
			req.RouteID, _ = strconv.Atoi(idStr)
		}
	}

	if req.RouteID <= 0 {
		http.Error(w, "Thiếu route_id", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteDomainRoute(req.RouteID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	// Làm mới RAM Cache
	_ = h.domainCache.Reload(context.Background())

	// Ghi nhận Audit Log
	clientIP := ExtractClientIP(r)
	ua := r.UserAgent()
	deviceType, browser, osName := security.ParseUserAgent(ua)
	var adminID *int
	var username string
	if admin, ok := r.Context().Value(AdminContextKey).(*models.AdminUser); ok && admin != nil {
		adminID = &admin.ID
		username = admin.Username
	}
	_ = h.store.RecordAuditLog(&models.AdminAuditLog{
		AdminID:    adminID,
		Username:   username,
		Action:     "DELETE_DOMAIN_ROUTE",
		Resource:   "domain_routes",
		ResourceID: strconv.Itoa(req.RouteID),
		Status:     "success",
		Details:    map[string]interface{}{"route_id": req.RouteID},
		IPAddress:  clientIP,
		UserAgent:  ua,
		DeviceType: deviceType,
		Browser:    browser,
		OS:         osName,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Đã gỡ bỏ đường dẫn khỏi tên miền",
	})
}

// HandleCheckSSL kiểm tra chi tiết bản ghi DNS và trạng thái chứng chỉ SSL của tên miền
func (h *DomainHandler) HandleCheckSSL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DomainID int    `json:"domain_id"`
		Domain   string `json:"domain"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.DomainID <= 0 {
		if idStr := r.URL.Query().Get("domain_id"); idStr != "" {
			req.DomainID, _ = strconv.Atoi(idStr)
		}
	}
	if req.Domain == "" {
		req.Domain = strings.TrimSpace(r.URL.Query().Get("domain"))
	}

	var targetDomain *models.Domain
	var err error
	if req.DomainID > 0 {
		targetDomain, err = h.store.GetDomain(req.DomainID)
	} else if req.Domain != "" {
		targetDomain, err = h.store.GetDomainByName(req.Domain)
	}

	if err != nil || targetDomain == nil {
		if req.Domain != "" {
			targetDomain = &models.Domain{
				Domain:   strings.ToLower(strings.TrimSpace(req.Domain)),
				TargetIP: "103.124.95.232",
			}
		} else {
			http.Error(w, "Không tìm thấy tên miền để kiểm tra SSL", http.StatusBadRequest)
			return
		}
	}

	cleanDomain := strings.ToLower(strings.TrimSpace(targetDomain.Domain))
	if idx := strings.Index(cleanDomain, ":"); idx != -1 {
		cleanDomain = cleanDomain[:idx]
	}
	cleanDomain = strings.TrimPrefix(cleanDomain, "http://")
	cleanDomain = strings.TrimPrefix(cleanDomain, "https://")
	cleanDomain = strings.TrimRight(cleanDomain, "/")

	serverIP := "103.124.95.232"

	// 1. Kiểm tra DNS A Records
	foundIPs := make([]string, 0)
	isIPMatched := false
	ipRecords, err := net.LookupIP(cleanDomain)
	if err == nil {
		for _, ip := range ipRecords {
			ipStr := ip.String()
			foundIPs = append(foundIPs, ipStr)
			if ipStr == serverIP || strings.HasPrefix(ipStr, "127.") || ipStr == "::1" {
				isIPMatched = true
			}
		}
	}

	result := map[string]interface{}{
		"domain":     cleanDomain,
		"server_ip":  serverIP,
		"found_ips":  foundIPs,
		"is_dns_ok":  isIPMatched,
		"ssl_active": false,
		"ssl_status": "pending",
		"issuer":     "",
		"expires_at": "",
		"days_left":  0,
		"message":    "",
	}

	if !isIPMatched {
		result["ssl_status"] = "dns_error"
		if len(foundIPs) > 0 {
			result["message"] = fmt.Sprintf("Tên miền đang trỏ về IP [%s]. Vui lòng cập nhật bản ghi A về IP máy chủ %s để Caddy kích hoạt chứng chỉ SSL.", strings.Join(foundIPs, ", "), serverIP)
		} else {
			result["message"] = fmt.Sprintf("Chưa tìm thấy bản ghi DNS của tên miền. Vui lòng trỏ bản ghi A về IP máy chủ %s và đợi lan truyền DNS.", serverIP)
		}
		if targetDomain.ID > 0 {
			_ = h.store.UpdateDomainDNSStatus(targetDomain.ID, "pending", "dns_error")
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    result,
		})
		return
	}

	// 2. Nếu DNS đã trỏ đúng IP server: Thử nghiệm kết nối TLS / HTTPS trực tiếp
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	tlsConfig := &tls.Config{
		ServerName:         cleanDomain,
		InsecureSkipVerify: false,
	}

	conn, errTLS := tls.DialWithDialer(dialer, "tcp", cleanDomain+":443", tlsConfig)
	if errTLS == nil && conn != nil {
		defer conn.Close()
		state := conn.ConnectionState()
		if len(state.PeerCertificates) > 0 {
			cert := state.PeerCertificates[0]
			daysLeft := int(time.Until(cert.NotAfter).Hours() / 24)
			issuer := cert.Issuer.CommonName
			if issuer == "" && len(cert.Issuer.Organization) > 0 {
				issuer = cert.Issuer.Organization[0]
			}
			result["ssl_active"] = true
			result["ssl_status"] = "active"
			result["issuer"] = issuer
			result["expires_at"] = cert.NotAfter.Format("2006-01-02 15:04:05")
			result["days_left"] = daysLeft
			result["message"] = fmt.Sprintf("Chứng chỉ SSL (%s) đang hoạt động hợp lệ! Còn %d ngày hiệu lực.", issuer, daysLeft)

			if targetDomain.ID > 0 {
				_ = h.store.UpdateDomainDNSStatus(targetDomain.ID, "active", "active")
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"data":    result,
			})
			return
		}
	}

	// 3. Nếu chưa có chứng chỉ TLS nhưng DNS đã trỏ: Gửi HTTP request để kích hoạt Caddy On-Demand TLS
	go func(d string) {
		client := &http.Client{Timeout: 8 * time.Second}
		_, _ = client.Get("http://" + d)
	}(cleanDomain)

	result["ssl_active"] = false
	result["ssl_status"] = "issuing"
	result["message"] = fmt.Sprintf("DNS đã trỏ chuẩn về %s! Hệ thống đã gửi yêu cầu kích hoạt cấp chứng chỉ SSL tự động (Let's Encrypt / ZeroSSL) tới Caddy. Quá trình cấp phát mất từ 30s - 60s, vui lòng bấm kiểm tra lại sau 1 phút.", serverIP)

	if targetDomain.ID > 0 {
		_ = h.store.UpdateDomainDNSStatus(targetDomain.ID, "active", "pending")
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    result,
	})
}

