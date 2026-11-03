package models

// Domain đại diện cho một tên miền được quản lý và trỏ về hệ thống
type Domain struct {
	ID               int           `json:"id"`
	Domain           string        `json:"domain"`             // vd: hanghoaphaisinh.net, dautuhanghoa.vn
	NS1              string        `json:"ns1"`                // ns1.hanghoaphaisinh.net
	NS2              string        `json:"ns2"`                // ns2.hanghoaphaisinh.net
	TargetIP         string        `json:"target_ip"`          // IP máy chủ đích
	DNSStatus        string        `json:"dns_status"`         // 'active' (đã trỏ đúng), 'pending' (chờ trỏ), 'error'
	SSLStatus        string        `json:"ssl_status"`         // 'active', 'pending'
	DefaultLandingID string        `json:"default_landing_id"` // Landing mặc định khi vào root '/'
	Notes            string        `json:"notes"`
	Routes           []DomainRoute `json:"routes,omitempty"`
	CreatedAt        string        `json:"created_at"`
	UpdatedAt        string        `json:"updated_at"`
}

// DomainRoute đại diện cho một đường dẫn/thư mục trên tên miền được gắn với Landing Page
type DomainRoute struct {
	ID          int    `json:"id"`
	DomainID    int    `json:"domain_id"`
	Domain      string `json:"domain,omitempty"`
	PathPrefix  string `json:"path_prefix"`  // vd: '/', '/khoa-hoc-dau-tu', '/ads', '/t11'
	LandingID   string `json:"landing_id"`   // mxv_080_online, ld_dautu_01...
	LandingName string `json:"landing_name,omitempty"`
	IsActive    bool   `json:"is_active"`
	RedirectURL string `json:"redirect_url,omitempty"`
	DeployedAt  string `json:"deployed_at"`
}

// DNSCheckResult kết quả kiểm tra bản ghi DNS thực tế của tên miền
type DNSCheckResult struct {
	Domain        string   `json:"domain"`
	ExpectedNS    []string `json:"expected_ns"`
	FoundNS       []string `json:"found_ns"`
	ExpectedIP    string   `json:"expected_ip"`
	FoundIPs      []string `json:"found_ips"`
	IsNSMatched   bool     `json:"is_ns_matched"`
	IsIPMatched   bool     `json:"is_ip_matched"`
	Status        string   `json:"status"` // 'active', 'pending', 'error'
	Message       string   `json:"message"`
	CheckedAt     string   `json:"checked_at"`
}
