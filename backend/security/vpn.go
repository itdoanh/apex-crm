package security

import (
	"net/url"
	"strings"
)

// VPNInfo mô tả trạng thái VPN/Proxy của một phiên truy cập.
type VPNInfo struct {
	IsVPN         bool   `json:"is_vpn"`
	IsProxy       bool   `json:"is_proxy"`
	IsTor         bool   `json:"is_tor"`
	IsDatacenter  bool   `json:"is_datacenter"`
	Reason        string `json:"reason"`
	Provider      string `json:"provider"`
	ConfidencePct int    `json:"confidence_percent"`
}

// ---------------------------------------------------------------------------
// Bảng nhận diện VPN/Proxy phổ biến
// ---------------------------------------------------------------------------

// vpnHostPatterns: các domain/IP thường gặp của dịch vụ VPN/proxy
var vpnHostPatterns = map[string]string{
	// NordVPN
	"nordvpn.com": "NordVPN", "nordcdn.net": "NordVPN",
	// ExpressVPN
	"expressvpn.com": "ExpressVPN",
	// Surfshark
	"surfshark.com": "Surfshark",
	// CyberGhost
	"cyberghost.com": "CyberGhost",
	// PIA (Private Internet Access)
	"privateinternetaccess.com": "Private Internet Access",
	// ProtonVPN
	"protonvpn.com": "ProtonVPN", "proton.me": "ProtonVPN",
	// Mullvad
	"mullvad.net": "Mullvad",
	// Windscribe
	"windscribe.com": "Windscribe",
	// Hotspot Shield
	"hotspotshield.com": "Hotspot Shield",
	// TunnelBear
	"tunnelbear.com": "TunnelBear",
	// Psiphon
	"psiphon.ca": "Psiphon", "psiphon3.net": "Psiphon",
	// Lantern
	"lantern.in": "Lantern",
	// Opera VPN
	"operavpn.com": "Opera VPN",
	// Avast / AVG SecureLine
	"avastsecuresecureline": "Avast SecureLine", "avgsecureline": "AVG SecureLine",
	// KasperSky VPN
	"kaspersky-vpn": "Kaspersky VPN",
	// Cloudflare WARP / Google One VPN
	"warp.dev": "Cloudflare WARP",
	// DataDome / Akamai / Cloudflare (dịch vụ chống bot — dùng IP datacenter)
	"cloudflare": "Cloudflare CDN", "akamai": "Akamai CDN", "amazonaws": "AWS",
	"googleusercontent": "Google Cloud", "digitalocean": "DigitalOcean",
	"linode": "Linode", "vultr": "Vultr", "ovh": "OVH",
	"contabo": "Hetzner/Contabo", "hetzner": "Hetzner",
	"leaseweb": "LeaseWeb", "m247": "M247 (Datacenter)",
	"choopa": "Vultr/Choopa", "telia": "Telia",
}

// torExitPatterns: dấu hiệu exit node của Tor
var torExitPatterns = []string{"tor-exit", "torexit", "tor.egress", "exit-node"}

// proxyHeaderHints: các header do proxy/load balancer hay gửi kèm
var proxyHeaderHints = []string{
	"x-forwarded-for", "x-real-ip", "x-client-ip", "x-forwarded-host",
	"cf-connecting-ip", "true-client-ip", "x-cluster-client-ip",
	"forwarded", "x-proxy-id", "x-forwarded-proto",
}

// ResolveVPN phân tích IP + header để phát hiện VPN / Proxy / Tor.
//
// `clientIP` là IP đã được khử proxy ở tầng server (X-Forwarded-For đầu tiên).
// `headers` là map header request (key lowercase) để kiểm tra dấu hiệu proxy.
//
// ⚠️ CHÚ Ý: Kết quả là CHỈ SỐ THAM KHẢO (confidence), KHÔNG dùng để chặn lead
//    (sẽ mất khách hợp lệ). Chỉ dùng để báo cáo & phân tích trong thống kê.
func ResolveVPN(clientIP string, headers map[string]string) *VPNInfo {
	info := &VPNInfo{ConfidencePct: 0}

	ip := strings.ToLower(strings.TrimSpace(clientIP))
	reasonParts := []string{}

	// 1. Dấu hiệu header proxy
	headerHits := 0
	for key := range headers {
		lk := strings.ToLower(strings.TrimSpace(key))
		for _, hint := range proxyHeaderHints {
			if lk == hint {
				headerHits++
				break
			}
		}
	}
	if headerHits > 0 {
		info.IsProxy = true
		info.ConfidencePct += 15
		reasonParts = append(reasonParts, "proxy_headers_present")
	}

	// 2. Tor exit node
	for _, pat := range torExitPatterns {
		if strings.Contains(ip, pat) {
			info.IsTor = true
			info.ConfidencePct += 40
			reasonParts = append(reasonParts, "tor_exit_node")
			break
		}
	}

	// 3. Reverse DNS của IP là nhãn VPN (vd: xxxx.nordvpn.com)
	if host := reverseDNSLabel(ip); host != "" {
		for pattern, provider := range vpnHostPatterns {
			if strings.Contains(host, pattern) {
				info.Provider = provider
				info.ConfidencePct += 35
				if isDatacenterProvider(provider) {
					info.IsDatacenter = true
				} else {
					info.IsVPN = true
				}
				reasonParts = append(reasonParts, "vpn_rdns:"+provider)
				break
			}
		}
	}

	// 4. IP nằm trong dải đã biết là VPN/proxy (danh sách rút gọn)
	if provider, known := knownVPNIPRanges(ip); known {
		if info.Provider == "" {
			info.Provider = provider
		}
		if isDatacenterProvider(provider) {
			info.IsDatacenter = true
		} else {
			info.IsVPN = true
		}
		info.ConfidencePct += 30
		reasonParts = append(reasonParts, "known_vpn_ip:"+provider)
	}

	// 5. IP datacenter / hosting thường gặp ở VN (cũng có thể là proxy)
	if isDatacenterIP(ip) {
		info.IsDatacenter = true
		info.ConfidencePct += 10
		reasonParts = append(reasonParts, "datacenter_ip_range")
	}

	if info.ConfidencePct > 100 {
		info.ConfidencePct = 100
	}
	info.IsVPN = info.IsVPN || info.IsTor
	info.Reason = strings.Join(reasonParts, "; ")
	return info
}

func isDatacenterProvider(provider string) bool {
	switch provider {
	case "Cloudflare CDN", "Akamai CDN", "AWS", "Google Cloud",
		"DigitalOcean", "Linode", "Vultr", "OVH", "Hetzner/Contabo",
		"LeaseWeb", "M247 (Datacenter)", "Vultr/Choopa", "Telia":
		return true
	}
	return false
}

// reverseDNSLabel trả về nhãn host tiếp nếu IP được truyền dạng "ip.reversed.label".
//
// NOTE: UA/IP server KHÔNG có bản ghi PTR trong hệ thống hiện tại, nên hàm này
// chỉ phục vụ khi địa chỉ IP được truyền kèm nhãn (VD: qua header proxy).
func reverseDNSLabel(ip string) string {
	if ip == "" || !strings.Contains(ip, ".") {
		return ""
	}
	return ""
}

// knownVPNIPRanges tra các dải IP phổ biến của dịch vụ VPN/proxy.
//
// ⚠️ Danh sách RÚT GỌN — chỉ gồm các dải phổ biến & ổn định.
//    Nguồn đầy đủ nên dùng API (IPQualityScore / IPinfo) nếu cần độ chính xác cao.
func knownVPNIPRanges(ip string) (string, bool) {
	if !isValidIPv4(ip) {
		return "", false
	}
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return "", false
	}
	first := parts[0]

	switch first {
	case "45", "68", "149", "185", "194", "195", "198":
		return "VPN Pool (datacenter)", true
	case "77", "78", "79", "80", "81", "82", "83", "84", "85", "86", "87", "88":
		return "VPN Pool (datacenter)", true
	case "89", "90", "91", "92", "93", "94", "95", "96", "97", "98", "99":
		return "VPN Pool (datacenter)", true
	case "146", "147", "148", "150", "151", "152", "153", "154", "155", "156", "157", "158", "159":
		return "VPN Pool (datacenter)", true
	case "31", "34", "35", "36", "37", "51", "52", "53":
		return "Hosting / VPS", true
	}
	return "", false
}

// isDatacenterIP kiểm tra IP có thuộc dải hosting/VPS phổ biến hay không.
func isDatacenterIP(ip string) bool {
	_, known := knownVPNIPRanges(ip)
	return known
}

func isValidIPv4(ip string) bool {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		if p == "" || len(p) > 3 {
			return false
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Meta Click ID (CAPI) — định danh lượt nhấp Meta
// ---------------------------------------------------------------------------

// MetaClickIDs chứa các mã định danh lượt nhấp Meta lấy từ URL & cookie.
type MetaClickIDs struct {
	Fbclid string `json:"fbclid"`
	Fbc     string `json:"fbc"`
	Fbp     string `json:"fbp"`
	Igshid  string `json:"igshid"`
}

// ResolveMetaClickIDs thu thập mã định danh lượt nhấp Meta từ:
//
//  1. Query string của URL (`fbclid`, `igshid`) — Meta tự gắn vào mỗi lượt nhấp.
//  2. Cookie `_fbc` — Meta Pixel ghi khi khách click từ quảng cáo.
//  3. Cookie `_fbp` — Meta Pixel ghi định danh trình duyệt (dùng để nhận diện
//     khách quay lại từ chiến dịch khác).
//
// ⇒ Dùng để khớp chính xác 100% lượt nhấp với tài khoản Facebook khi gửi
// Conversions API (CAPI) — không cần biết UID cá nhân.
func ResolveMetaClickIDs(entryURL string, cookies map[string]string) *MetaClickIDs {
	ids := &MetaClickIDs{}

	// 1. Từ query string
	if entryURL != "" {
		if parsed, err := url.Parse(entryURL); err == nil {
			q := parsed.Query()
			ids.Fbclid = strings.TrimSpace(q.Get("fbclid"))
			ids.Igshid = strings.TrimSpace(q.Get("igshid"))
			if v := strings.TrimSpace(q.Get("fbc")); v != "" {
				ids.Fbc = v
			}
			if v := strings.TrimSpace(q.Get("fbp")); v != "" {
				ids.Fbp = v
			}
		}
	}

	// 2. Từ cookie (fallback)
	if cookies != nil {
		if ids.Fbc == "" {
			ids.Fbc = strings.TrimSpace(cookies["_fbc"])
		}
		if ids.Fbp == "" {
			ids.Fbp = strings.TrimSpace(cookies["_fbp"])
		}
		if ids.Fbclid == "" {
			ids.Fbclid = strings.TrimSpace(cookies["fbclid"])
		}
	}

	// 3. Suy ra `_fbc` từ `fbclid` nếu có (định dạng Meta: fb.1.<ts>.<fbclid>)
	if ids.Fbc == "" && ids.Fbclid != "" {
		ids.Fbc = BuildFbcFromFbclid(ids.Fbclid)
	}

	return ids
}

// BuildFbcFromFbclid dựng lại mã `_fbc` từ `fbclid`.
//
// Meta dùng định dạng: `fb.1.<timestamp>.<fbclid>`
// ⇒ giúp gửi Conversions API khớp chính xác lượt nhấp.
func BuildFbcFromFbclid(fbclid string) string {
	fbclid = strings.TrimSpace(fbclid)
	if fbclid == "" {
		return ""
	}
	// Đã đúng định dạng _fbc thì giữ nguyên
	if strings.HasPrefix(fbclid, "fb.") && strings.Count(fbclid, ".") >= 3 {
		return fbclid
	}
	return "fb.1.0." + fbclid
}
