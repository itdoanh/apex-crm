package security

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// GeoLocation chứa thông tin địa lý và mạng viễn thông phân giải từ IP
type GeoLocation struct {
	IP          string  `json:"ip"`
	Country     string  `json:"country"`
	CountryCode string  `json:"country_code"`
	City        string  `json:"city"`
	Region      string  `json:"region"`      // Miền Bắc, Miền Trung, Miền Nam, Quốc Tế
	RegionName  string  `json:"region_name"` // Tên gốc từ nhà cung cấp
	ISP         string  `json:"isp"`         // VNPT, Viettel, FPT Telecom, MobiFone,...
	Lat         float64 `json:"lat,omitempty"`
	Lon         float64 `json:"lon,omitempty"`
}

var (
	geoCache sync.Map // Map[string]*GeoLocation
	httpClient = &http.Client{
		Timeout: 1200 * time.Millisecond,
	}
)

// IsPrivateOrLocalIP kiểm tra xem IP có phải là IP nội bộ, loopback hoặc private không
func IsPrivateOrLocalIP(ipStr string) bool {
	ipStr = strings.TrimSpace(ipStr)
	if ipStr == "" || ipStr == "127.0.0.1" || ipStr == "::1" || ipStr == "localhost" {
		return true
	}
	parsed := net.ParseIP(ipStr)
	if parsed == nil {
		return true
	}
	return parsed.IsLoopback() || parsed.IsPrivate() || parsed.IsUnspecified() || parsed.IsLinkLocalUnicast()
}

// NormalizeVietnameseCity chuẩn hóa tên tỉnh thành Việt Nam sang tiếng Việt có dấu chuẩn mực
func NormalizeVietnameseCity(rawCity string) string {
	cityLower := strings.ToLower(strings.TrimSpace(rawCity))
	cityLower = strings.ReplaceAll(cityLower, "city", "")
	cityLower = strings.ReplaceAll(cityLower, "province", "")
	cityLower = strings.TrimSpace(cityLower)

	switch {
	case strings.Contains(cityLower, "hanoi") || strings.Contains(cityLower, "ha noi"):
		return "Hà Nội"
	case strings.Contains(cityLower, "ho chi minh") || strings.Contains(cityLower, "saigon") || strings.Contains(cityLower, "hcm"):
		return "Hồ Chí Minh"
	case strings.Contains(cityLower, "da nang") || strings.Contains(cityLower, "danang"):
		return "Đà Nẵng"
	case strings.Contains(cityLower, "hai phong") || strings.Contains(cityLower, "haiphong"):
		return "Hải Phòng"
	case strings.Contains(cityLower, "can tho") || strings.Contains(cityLower, "cantho"):
		return "Cần Thơ"
	case strings.Contains(cityLower, "binh duong") || strings.Contains(cityLower, "thu dau mot"):
		return "Bình Dương"
	case strings.Contains(cityLower, "dong nai") || strings.Contains(cityLower, "bien hoa"):
		return "Đồng Nai"
	case strings.Contains(cityLower, "ba ria") || strings.Contains(cityLower, "vung tau"):
		return "Bà Rịa - Vũng Tàu"
	case strings.Contains(cityLower, "quang ninh") || strings.Contains(cityLower, "ha long"):
		return "Quảng Ninh"
	case strings.Contains(cityLower, "bac ninh"):
		return "Bắc Ninh"
	case strings.Contains(cityLower, "hai duong"):
		return "Hải Dương"
	case strings.Contains(cityLower, "hung yen"):
		return "Hưng Yên"
	case strings.Contains(cityLower, "nam dinh"):
		return "Nam Định"
	case strings.Contains(cityLower, "ninh binh"):
		return "Ninh Bình"
	case strings.Contains(cityLower, "thai binh"):
		return "Thái Bình"
	case strings.Contains(cityLower, "vinh phuc"):
		return "Vĩnh Phúc"
	case strings.Contains(cityLower, "phu tho") || strings.Contains(cityLower, "viet tri"):
		return "Phú Thọ"
	case strings.Contains(cityLower, "thai nguyen"):
		return "Thái Nguyên"
	case strings.Contains(cityLower, "bac giang"):
		return "Bắc Giang"
	case strings.Contains(cityLower, "thanh hoa"):
		return "Thanh Hóa"
	case strings.Contains(cityLower, "nghe an") || strings.Contains(cityLower, "vinh"):
		return "Nghệ An"
	case strings.Contains(cityLower, "ha tinh"):
		return "Hà Tĩnh"
	case strings.Contains(cityLower, "quang binh") || strings.Contains(cityLower, "dong hoi"):
		return "Quảng Bình"
	case strings.Contains(cityLower, "quang tri") || strings.Contains(cityLower, "dong ha"):
		return "Quảng Trị"
	case strings.Contains(cityLower, "hue") || strings.Contains(cityLower, "thua thien"):
		return "Thừa Thiên Huế"
	case strings.Contains(cityLower, "quang nam") || strings.Contains(cityLower, "tam ky") || strings.Contains(cityLower, "hoi an"):
		return "Quảng Nam"
	case strings.Contains(cityLower, "quang ngai"):
		return "Quảng Ngãi"
	case strings.Contains(cityLower, "binh dinh") || strings.Contains(cityLower, "qui nhon") || strings.Contains(cityLower, "quy nhon"):
		return "Bình Định"
	case strings.Contains(cityLower, "phu yen") || strings.Contains(cityLower, "tuy hoa"):
		return "Phú Yên"
	case strings.Contains(cityLower, "khanh hoa") || strings.Contains(cityLower, "nha trang"):
		return "Khánh Hòa"
	case strings.Contains(cityLower, "ninh thuan") || strings.Contains(cityLower, "phan rang"):
		return "Ninh Thuận"
	case strings.Contains(cityLower, "binh thuan") || strings.Contains(cityLower, "phan thiet"):
		return "Bình Thuận"
	case strings.Contains(cityLower, "dak lak") || strings.Contains(cityLower, "buon ma thuot") || strings.Contains(cityLower, "dac lac"):
		return "Đắk Lắk"
	case strings.Contains(cityLower, "gia lai") || strings.Contains(cityLower, "pleiku"):
		return "Gia Lai"
	case strings.Contains(cityLower, "kon tum") || strings.Contains(cityLower, "kontum"):
		return "Kon Tum"
	case strings.Contains(cityLower, "dak nong") || strings.Contains(cityLower, "dac nong"):
		return "Đắk Nông"
	case strings.Contains(cityLower, "lam dong") || strings.Contains(cityLower, "da lat") || strings.Contains(cityLower, "dalat"):
		return "Lâm Đồng"
	case strings.Contains(cityLower, "long an") || strings.Contains(cityLower, "tan an"):
		return "Long An"
	case strings.Contains(cityLower, "tien giang") || strings.Contains(cityLower, "my tho"):
		return "Tiền Giang"
	case strings.Contains(cityLower, "ben tre"):
		return "Bến Tre"
	case strings.Contains(cityLower, "tra vinh"):
		return "Trà Vinh"
	case strings.Contains(cityLower, "vinh long"):
		return "Vĩnh Long"
	case strings.Contains(cityLower, "dong thap") || strings.Contains(cityLower, "cao lanh"):
		return "Đồng Tháp"
	case strings.Contains(cityLower, "an giang") || strings.Contains(cityLower, "long xuyen"):
		return "An Giang"
	case strings.Contains(cityLower, "kien giang") || strings.Contains(cityLower, "rach gia") || strings.Contains(cityLower, "phu quoc"):
		return "Kiên Giang"
	case strings.Contains(cityLower, "hau giang") || strings.Contains(cityLower, "vi thanh"):
		return "Hậu Giang"
	case strings.Contains(cityLower, "soc trang"):
		return "Sóc Trăng"
	case strings.Contains(cityLower, "bac lieu"):
		return "Bạc Liêu"
	case strings.Contains(cityLower, "ca mau"):
		return "Cà Mau"
	case strings.Contains(cityLower, "tay ninh"):
		return "Tây Ninh"
	case strings.Contains(cityLower, "binh phuoc") || strings.Contains(cityLower, "dong xoai"):
		return "Bình Phước"
	case strings.Contains(cityLower, "lang son"):
		return "Lạng Sơn"
	case strings.Contains(cityLower, "lao cai"):
		return "Lào Cai"
	case strings.Contains(cityLower, "yen bai"):
		return "Yên Bái"
	case strings.Contains(cityLower, "son la"):
		return "Sơn La"
	case strings.Contains(cityLower, "hoa binh"):
		return "Hòa Bình"
	case strings.Contains(cityLower, "ha giang"):
		return "Hà Giang"
	case strings.Contains(cityLower, "tuyen quang"):
		return "Tuyên Quang"
	case strings.Contains(cityLower, "cao bang"):
		return "Cao Bằng"
	case strings.Contains(cityLower, "bac kan"):
		return "Bắc Kạn"
	case strings.Contains(cityLower, "dien bien"):
		return "Điện Biên"
	case strings.Contains(cityLower, "lai chau"):
		return "Lai Châu"
	default:
		if rawCity != "" {
			return rawCity
		}
		return "Khác / Chưa rõ"
	}
}

// ClassifyRegion phân loại tỉnh thành thành 3 Vùng miền: Miền Bắc, Miền Trung, Miền Nam hoặc Quốc Tế
func ClassifyRegion(countryCode, city string) string {
	if countryCode != "" && countryCode != "VN" && countryCode != "VNM" {
		return "Quốc Tế"
	}

	bacCities := []string{
		"Hà Nội", "Hải Phòng", "Quảng Ninh", "Bắc Ninh", "Hải Dương", "Hưng Yên",
		"Nam Định", "Ninh Bình", "Thái Bình", "Vĩnh Phúc", "Phú Thọ", "Thái Nguyên",
		"Bắc Giang", "Lạng Sơn", "Lào Cai", "Yên Bái", "Sơn La", "Hòa Bình",
		"Hà Giang", "Tuyên Quang", "Cao Bằng", "Bắc Kạn", "Điện Biên", "Lai Châu",
	}
	for _, c := range bacCities {
		if strings.EqualFold(city, c) || strings.Contains(city, c) {
			return "Miền Bắc"
		}
	}

	trungCities := []string{
		"Đà Nẵng", "Thừa Thiên Huế", "Quảng Nam", "Quảng Ngãi", "Bình Định", "Phú Yên",
		"Khánh Hòa", "Ninh Thuận", "Bình Thuận", "Thanh Hóa", "Nghệ An", "Hà Tĩnh",
		"Quảng Bình", "Quảng Trị", "Đắk Lắk", "Gia Lai", "Kon Tum", "Đắk Nông", "Lâm Đồng",
	}
	for _, c := range trungCities {
		if strings.EqualFold(city, c) || strings.Contains(city, c) {
			return "Miền Trung"
		}
	}

	namCities := []string{
		"Hồ Chí Minh", "Bình Dương", "Đồng Nai", "Bà Rịa - Vũng Tàu", "Long An",
		"Tiền Giang", "Bến Tre", "Trà Vinh", "Vĩnh Long", "Đồng Tháp", "An Giang",
		"Kiên Giang", "Hậu Giang", "Sóc Trăng", "Bạc Liêu", "Cà Mau", "Tây Ninh", "Bình Phước", "Cần Thơ",
	}
	for _, c := range namCities {
		if strings.EqualFold(city, c) || strings.Contains(city, c) {
			return "Miền Nam"
		}
	}

	return "Miền Bắc" // Mặc định nếu thuộc VN
}

// NormalizeISP rút gọn và chuẩn hóa tên nhà mạng viễn thông / ISP
func NormalizeISP(rawISP string) string {
	ispLower := strings.ToLower(rawISP)
	switch {
	case strings.Contains(ispLower, "vnpt") || strings.Contains(ispLower, "vietnam post") || strings.Contains(ispLower, "vinaphone"):
		return "VNPT Telecom"
	case strings.Contains(ispLower, "viettel") || strings.Contains(ispLower, "military"):
		return "Viettel Telecom"
	case strings.Contains(ispLower, "fpt"):
		return "FPT Telecom"
	case strings.Contains(ispLower, "mobifone"):
		return "MobiFone"
	case strings.Contains(ispLower, "cmc"):
		return "CMC Telecom"
	case strings.Contains(ispLower, "vietnamobile"):
		return "Vietnamobile"
	case strings.Contains(ispLower, "spt") || strings.Contains(ispLower, "saigon post"):
		return "SPT Telecom"
	case strings.Contains(ispLower, "google"):
		return "Google Cloud"
	case strings.Contains(ispLower, "cloudflare"):
		return "Cloudflare"
	case strings.Contains(ispLower, "amazon") || strings.Contains(ispLower, "aws"):
		return "AWS"
	case strings.Contains(ispLower, "local") || strings.Contains(ispLower, "lan"):
		return "Mạng Nội Bộ"
	default:
		if strings.TrimSpace(rawISP) != "" {
			return rawISP
		}
		return "Khác"
	}
}

// ResolveIP tra cứu địa chỉ thực tế và nhà mạng từ IP (kèm Cache & Timeout bảo vệ)
func ResolveIP(ip string) *GeoLocation {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return &GeoLocation{
			IP:          "",
			Country:     "Việt Nam",
			CountryCode: "VN",
			City:        "Hà Nội",
			Region:      "Miền Bắc",
			ISP:         "Mạng Nội Bộ",
		}
	}

	// 1. Kiểm tra cache RAM siêu tốc (<0.01ms)
	if cached, ok := geoCache.Load(ip); ok {
		if loc, ok := cached.(*GeoLocation); ok {
			return loc
		}
	}

	// 2. Nếu là IP nội bộ / Localhost / Dev
	if IsPrivateOrLocalIP(ip) {
		loc := &GeoLocation{
			IP:          ip,
			Country:     "Việt Nam",
			CountryCode: "VN",
			City:        "Hà Nội (Local)",
			Region:      "Miền Bắc",
			RegionName:  "Hanoi",
			ISP:         "Mạng Nội Bộ / Localhost",
		}
		geoCache.Store(ip, loc)
		return loc
	}

	// 3. Tra cứu từ ip-api.com với timeout 1.2s
	url := fmt.Sprintf("http://ip-api.com/json/%s?fields=status,message,country,countryCode,region,regionName,city,isp,lat,lon,query", ip)
	req, err := http.NewRequest("GET", url, nil)
	if err == nil {
		req.Header.Set("User-Agent", "ApexFintech-GeoIP/2.0")
		resp, err := httpClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)

			var apiResp struct {
				Status      string  `json:"status"`
				Country     string  `json:"country"`
				CountryCode string  `json:"countryCode"`
				Region      string  `json:"region"`
				RegionName  string  `json:"regionName"`
				City        string  `json:"city"`
				ISP         string  `json:"isp"`
				Lat         float64 `json:"lat"`
				Lon         float64 `json:"lon"`
			}
			if err := json.Unmarshal(body, &apiResp); err == nil && apiResp.Status == "success" {
				normCity := NormalizeVietnameseCity(apiResp.City)
				if normCity == "Khác / Chưa rõ" && apiResp.RegionName != "" {
					normCity = NormalizeVietnameseCity(apiResp.RegionName)
				}
				normRegion := ClassifyRegion(apiResp.CountryCode, normCity)
				normISP := NormalizeISP(apiResp.ISP)
				normCountry := apiResp.Country
				if apiResp.CountryCode == "VN" {
					normCountry = "Việt Nam"
				}

				loc := &GeoLocation{
					IP:          ip,
					Country:     normCountry,
					CountryCode: apiResp.CountryCode,
					City:        normCity,
					Region:      normRegion,
					RegionName:  apiResp.RegionName,
					ISP:         normISP,
					Lat:         apiResp.Lat,
					Lon:         apiResp.Lon,
				}
				geoCache.Store(ip, loc)
				return loc
			}
		}
	}

	// 4. Fallback mặc định khi mất mạng / timeout
	fallbackLoc := &GeoLocation{
		IP:          ip,
		Country:     "Việt Nam",
		CountryCode: "VN",
		City:        "Hà Nội",
		Region:      "Miền Bắc",
		RegionName:  "Hanoi",
		ISP:         "Viễn thông VN",
	}
	geoCache.Store(ip, fallbackLoc)
	return fallbackLoc
}
