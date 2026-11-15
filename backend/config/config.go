package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Port                      string
	DatabaseURL               string
	DBHost                    string
	DBPort                    string
	DBUser                    string
	DBPassword                string
	DBName                    string
	DBSSLMode                 string
	AllowOrigins              string
	AdminKey                  string
	GoogleServiceAccountJSON  string
	GoogleServiceAccountEmail string
	GoogleServiceAccountFile  string

	// ─── Môi trường triển khai ──────────────────────────────────────
	//
	// Hệ thống chạy song song 3 môi trường dùng CHUNG mã nguồn:
	//
	//	  Env       | Code trên server | Port | Database          | Domain
	//	-----------|------------------|------|-------------------|---------------------------
	//	local      | C:\code\...      | 8080 | apex_leads (docker) | localhost
	//	test       | C:\apex-test     | 8081 | apex_leads_test    | test.hanghoaphaisinh.net
	//	production | C:\apex          | 8080 | apex_leads         | crm.hanghoaphaisinh.net
	//
	// `Environment` quyết định:
	//   - host nào được phục vụ giao diện CRM Admin (AdminHosts)
	//   - domain nào được coi là "hệ thống" trong check-domain (OnDemandAllowList)
	//   - có bật CAPI / Google Sheets hay không (DisableExternalSync)
	//
	// ⚠️ Môi trường `test` KHÔNG đẩy dữ liệu ra ngoài (Meta CAPI, Google
	//    Sheets) để lead thử nghiệm không làm nhiễu KPI doanh thu thật.
	Environment string
	// AdminHosts là danh sách host (không port) được phục vụ giao diện CRM Admin.
	// Mọi host khác là "landing domain" công khai.
	AdminHosts []string
	// OnDemandAllowList là danh sách domain được phép cấp SSL on-demand.
	// Rỗng ⇒ dùng mặc định theo môi trường.
	OnDemandAllowList []string
	// DisableExternalSync = true ⇒ KHÔNG gửi Meta CAPI / Google Sheets.
	DisableExternalSync bool
	// AdminHost là host quản trị chính (dùng để ghi log & môi trường).
	AdminHost string

	// Meta Conversions API — fallback khi chưa có cấu hình trong DB
	// (bảng `facebook_capi_configs`). Cấu hình chính nằm trong CRM.
	MetaCAPIEnabled         bool
	MetaCAPIDefaultPixelID  string
	MetaCAPIDefaultToken    string
	MetaCAPIDefaultVersion  string
	MetaCAPIDefaultCurrency string
	MetaCAPIDefaultValue    float64
	MetaCAPIGatewayEnabled  bool
	MetaCAPIGatewayEndpoint string
	MetaCAPIGatewayAccessKey string
}

// loadDotEnv tự động đọc file .env nếu có để nạp biến môi trường cho máy chủ
func loadDotEnv() {
	candidates := []string{
		".env",
		filepath.Join("..", ".env"),
		filepath.Join("backend", ".env"),
	}
	// Kiểm tra cả thư mục chứa file thực thi
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), ".env"))
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "..", ".env"))
	}

	for _, path := range candidates {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				// Loại bỏ dấu nháy kép hoặc đơn nếu có bao quanh
				val = strings.Trim(val, `"'`)
				if key != "" && os.Getenv(key) == "" {
					_ = os.Setenv(key, val)
				}
			}
		}
		break // Đã nạp thành công từ 1 file .env đầu tiên tìm thấy
	}
}

// ─── Môi trường triển khai ──────────────────────────────────────────────
//
// Ba giá trị được hỗ trợ: `local`, `test`, `production`.
//
// ⚠️ Mặc định là `production` — hành vi của hệ thống khi thiếu cấu hình KHÔNG
//    được đổi so với trước đây (an toàn cho môi trường đang chạy thật).
const (
	EnvLocal      = "local"
	EnvTest       = "test"
	EnvProduction = "production"
)

// adminHostsByEnv là danh sách host được phục vụ giao diện CRM Admin cho từng
// môi trường.
//
// ⚠️ Host nằm trong danh sách này KHÔNG bị chặn khi truy cập `/admin`, `/ads`.
//    Mọi host khác (landing domain công khai) bị chặn ⇒ CRM không lộ ra
//    ngoài internet.
var adminHostsByEnv = map[string][]string{
	EnvLocal: {
		"localhost",
		"127.0.0.1",
	},
	EnvTest: {
		"test.hanghoaphaisinh.net",
		"localhost",
		"127.0.0.1",
	},
	EnvProduction: {
		"crm.hanghoaphaisinh.net",
		"localhost",
		"127.0.0.1",
		"103.124.95.232",
	},
}

// onDemandAllowListByEnv là danh sách domain được phép cấp SSL on-demand.
//
// ⚠️ Vì Caddy 2.8.4 KHÔNG hỗ trợ `ask` riêng cho từng site block (đã kiểm tra
//    2026-11-15), endpoint `check-domain` của MỖI môi trường phải trả 200 cho
//    đúng domain của môi trường đó. Danh sách này định nghĩa "đúng domain".
//
// ⇒ Môi trường `test` KHÔNG dùng suffix matching ⇒ backend test chỉ trả 200 cho
//    đúng `test.hanghoaphaisinh.net` + `ns1/ns2` ⇒ không cấp nhầm chứng chỉ cho
//    domain của production.
//
// ⚠️ NGOẠI LỆ: production GIỮ suffix matching `*.hanghoaphaisinh.net` (tương thích
//    ngược với code cũ trước 2026-11-15) — xem `IsOnDemandAllowed`.
//
//    Hệ quả: `test.hanghoaphaisinh.net` cũng được production cho phép ⇒ Caddy
//    (dùng global `on_demand_tls` hỏi backend production) cấp được SSL cho domain
//    test. Đây là CHỦ Ý — nếu không, test domain không truy cập được qua HTTPS.
//
//    Việc tách môi trường KHÔNG bị ảnh hưởng vì Caddy định tuyến theo HOST:
//    site block `test.hanghoaphaisinh.net` → 127.0.0.1:8081 (backend test),
//    site block `crm.hanghoaphaisinh.net` → 127.0.0.1:8080 (backend production).
var onDemandAllowListByEnv = map[string][]string{
	EnvLocal: {
		"localhost",
		"127.0.0.1",
	},
	EnvTest: {
		"test.hanghoaphaisinh.net",
		"ns1.hanghoaphaisinh.net",
		"ns2.hanghoaphaisinh.net",
	},
	EnvProduction: {
		"crm.hanghoaphaisinh.net",
		"hanghoaphaisinh.net",
		"ns1.hanghoaphaisinh.net",
		"ns2.hanghoaphaisinh.net",
		// Domain của môi trường test — Caddy dùng chung global on_demand_tls.
		"test.hanghoaphaisinh.net",
	},
}

// splitList tách chuỗi phân tách bởi dấu phẩy thành danh sách đã trim,
// loại bỏ phần tử rỗng.
func splitList(raw string) []string {
	out := make([]string, 0)
	for _, part := range strings.Split(raw, ",") {
		p := strings.ToLower(strings.TrimSpace(part))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// resolveEnvironment xác định môi trường đang chạy.
//
// Ưu tiên biến `APEX_ENV`; không có ⇒ suy ra từ `DB_NAME`:
//   - `apex_leads_test`  ⇒ test
//   - `apex_leads`       ⇒ production
//   - còn lại            ⇒ local
//
// ⚠️ Mặc định `production` nếu không suy ra được — an toàn cho môi trường thật.
func resolveEnvironment(dbName string) string {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("APEX_ENV")))
	switch env {
	case EnvLocal, EnvTest, EnvProduction:
		return env
	}
	switch strings.ToLower(strings.TrimSpace(dbName)) {
	case "apex_leads_test":
		return EnvTest
	case "apex_leads":
		return EnvProduction
	default:
		return EnvLocal
	}
}

// IsAdminHost kiểm tra host có phải host quản trị của môi trường đang chạy.
//
// Host này được phục vụ giao diện CRM Admin (`/ads`, `/admin`). Mọi host
// khác là "landing domain" công khai ⇒ KHÔNG được lộ CRM.
func (c *Config) IsAdminHost(host string) bool {
	norm := normalizeHost(host)
	if norm == "" {
		return false
	}
	for _, h := range c.AdminHosts {
		if normalizeHost(h) == norm {
			return true
		}
	}
	return false
}

// IsOnDemandAllowed kiểm tra domain có được phép cấp SSL on-demand.
//
// Endpoint `check-domain` của Caddy gọi hàm này. Vì Caddy 2.8.4 không hỗ trợ
// `ask` riêng theo site block ⇒ mỗi môi trường chỉ trả 200 cho đúng danh sách
// domain của mình ⇒ không cấp nhầm chứng chỉ cho domain của môi trường khác.
//
// ⚠️ Production giữ **tương thích ngược**: vẫn cho phép mọi subdomain
// `*.hanghoaphaisinh.net` (hành vi của code cũ trước 2026-11-15). Nếu bỏ,
// các landing domain dạng `lophoc.hanghoaphaisinh.net` chưa kịp khai báo trong
// CRM sẽ mất SSL. Môi trường `test` KHÔNG dùng suffix matching ⇒ không cấp
// nhầm chứng chỉ cho domain của production.
func (c *Config) IsOnDemandAllowed(domain string) bool {
	norm := normalizeHost(domain)
	if norm == "" {
		return false
	}
	for _, d := range c.OnDemandAllowList {
		if normalizeHost(d) == norm {
			return true
		}
	}
	// Tương thích ngược — chỉ áp dụng cho production.
	if c.Environment == EnvProduction && strings.HasSuffix(norm, ".hanghoaphaisinh.net") {
		return true
	}
	return false
}

// normalizeHost chuẩn hoá host: lowercase, bỏ port và tiền tố `www.`.
func normalizeHost(host string) string {
	h := strings.ToLower(strings.TrimSpace(host))
	if idx := strings.Index(h, ":"); idx != -1 {
		h = h[:idx]
	}
	h = strings.TrimPrefix(h, "www.")
	return strings.TrimSpace(h)
}

// IsExternalSyncEnabled trả về true nếu môi trường này ĐƯỢC PHÉP đẩy dữ liệu
// ra ngoài (Meta CAPI, Google Sheets).
//
// Môi trường `test` mặc định KHÔNG đẩy ⇒ lead thử nghiệm không làm nhiễu KPI
// doanh thu thật và không ghi vào Google Sheets production.
func (c *Config) IsExternalSyncEnabled() bool {
	return !c.DisableExternalSync
}

func Load() *Config {
	loadDotEnv()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	allowOrigins := os.Getenv("ALLOW_ORIGINS")
	if allowOrigins == "" {
		allowOrigins = "*"
	}

	adminKey := os.Getenv("ADMIN_KEY")
	if adminKey == "" {
		adminKey = "apex080"
	}

	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}

	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}

	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "postgres"
	}

	dbPass := os.Getenv("DB_PASSWORD")
	if dbPass == "" {
		dbPass = "postgres"
	}

	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "apex_leads"
	}

	dbSSLMode := os.Getenv("DB_SSLMODE")
	if dbSSLMode == "" {
		dbSSLMode = "disable"
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			url.QueryEscape(dbUser),
			url.QueryEscape(dbPass),
			dbHost,
			dbPort,
			dbName,
			dbSSLMode,
		)
	}

	// Nạp Service Account Key cho Google Sheets Email Bot (mặc định cho toàn hệ thống)
	saFile := os.Getenv("GOOGLE_SERVICE_ACCOUNT_FILE")
	saJSON := os.Getenv("GOOGLE_SERVICE_ACCOUNT_JSON")

	if saJSON == "" {
		candidates := []string{
			saFile,
			"service-account.json",
			filepath.Join("backend", "service-account.json"),
			filepath.Join("..", "service-account.json"),
			filepath.Join("config", "service-account.json"),
			filepath.Join("backend", "config", "service-account.json"),
		}
		if exe, err := os.Executable(); err == nil {
			candidates = append(candidates, filepath.Join(filepath.Dir(exe), "service-account.json"))
			candidates = append(candidates, filepath.Join(filepath.Dir(exe), "..", "service-account.json"))
			candidates = append(candidates, filepath.Join(filepath.Dir(exe), "backend", "service-account.json"))
		}
		for _, path := range candidates {
			if path == "" {
				continue
			}
			if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
				saJSON = string(data)
				saFile = path
				break
			}
		}
	}

	var saEmail string
	if saJSON != "" {
		var parsed struct {
			ClientEmail string `json:"client_email"`
		}
		if err := json.Unmarshal([]byte(saJSON), &parsed); err == nil {
			saEmail = parsed.ClientEmail
		}
	}
	if saEmail == "" {
		saEmail = os.Getenv("GOOGLE_BOT_EMAIL")
	}
	if saEmail == "" {
		saEmail = "sheet-bot@rinva-drive-tool.iam.gserviceaccount.com"
	}

	// ─── Meta Conversions API (fallback khi chưa cấu hình trong DB) ───
	metaPixel := strings.TrimSpace(os.Getenv("META_CAPI_DEFAULT_PIXEL_ID"))
	metaToken := strings.TrimSpace(os.Getenv("META_CAPI_DEFAULT_ACCESS_TOKEN"))
	metaVersion := strings.TrimSpace(os.Getenv("META_CAPI_DEFAULT_API_VERSION"))
	if metaVersion == "" {
		metaVersion = "v27.0"
	}
	metaCurrency := strings.TrimSpace(os.Getenv("META_CAPI_DEFAULT_CURRENCY"))
	if metaCurrency == "" {
		metaCurrency = "VND"
	}
	metaValue := 6000000.0
	if v := strings.TrimSpace(os.Getenv("META_CAPI_DEFAULT_COURSE_VALUE")); v != "" {
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			metaValue = parsed
		}
	}
	metaGatewayEnabled := strings.EqualFold(strings.TrimSpace(os.Getenv("META_CAPI_GATEWAY_ENABLED")), "true")
	metaGatewayEndpoint := strings.TrimSpace(os.Getenv("META_CAPI_GATEWAY_ENDPOINT"))
	metaGatewayKey := strings.TrimSpace(os.Getenv("META_CAPI_GATEWAY_ACCESS_KEY"))
	metaCAPIEnabled := metaPixel != "" && (metaToken != "" || (metaGatewayEnabled && metaGatewayKey != ""))

	// ─── Môi trường triển khai ──────────────────────────────────────
	environment := resolveEnvironment(dbName)

	// Admin hosts: cho phép ghi đè qua .env (danh sách phân tách bởi dấu phẩy).
	adminHosts := splitList(os.Getenv("APEX_ADMIN_HOSTS"))
	if len(adminHosts) == 0 {
		adminHosts = append([]string(nil), adminHostsByEnv[environment]...)
	}

	// Danh sách domain được phép cấp SSL on-demand.
	onDemandAllowList := splitList(os.Getenv("APEX_ON_DEMAND_DOMAINS"))
	if len(onDemandAllowList) == 0 {
		onDemandAllowList = append([]string(nil), onDemandAllowListByEnv[environment]...)
	}

	// Tắt đồng bộ ra ngoài (Meta CAPI + Google Sheets).
	// Mặc định: TẮT cho môi trường `test`; production/local thì BẬT.
	disableExternalSync := environment == EnvTest
	if v := strings.TrimSpace(os.Getenv("APEX_DISABLE_EXTERNAL_SYNC")); v != "" {
		disableExternalSync = strings.EqualFold(v, "true") || v == "1"
	}

	adminHost := strings.ToLower(strings.TrimSpace(os.Getenv("APEX_ADMIN_HOST")))
	if adminHost == "" && len(adminHosts) > 0 {
		adminHost = adminHosts[0]
	}

	return &Config{
		Port:                      port,
		DatabaseURL:               databaseURL,
		DBHost:                    dbHost,
		DBPort:                    dbPort,
		DBUser:                    dbUser,
		DBPassword:                dbPass,
		DBName:                    dbName,
		DBSSLMode:                 dbSSLMode,
		AllowOrigins:              allowOrigins,
		AdminKey:                  adminKey,
		GoogleServiceAccountJSON:  saJSON,
		GoogleServiceAccountEmail: saEmail,
		GoogleServiceAccountFile:  saFile,
		Environment:               environment,
		AdminHosts:                adminHosts,
		OnDemandAllowList:         onDemandAllowList,
		DisableExternalSync:       disableExternalSync,
		AdminHost:                 adminHost,
		MetaCAPIEnabled:           metaCAPIEnabled,
		MetaCAPIDefaultPixelID:    metaPixel,
		MetaCAPIDefaultToken:      metaToken,
		MetaCAPIDefaultVersion:    metaVersion,
		MetaCAPIDefaultCurrency:   metaCurrency,
		MetaCAPIDefaultValue:      metaValue,
		MetaCAPIGatewayEnabled:    metaGatewayEnabled,
		MetaCAPIGatewayEndpoint:   metaGatewayEndpoint,
		MetaCAPIGatewayAccessKey:  metaGatewayKey,
	}
}
