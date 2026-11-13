package security

import (
	"regexp"
	"strconv"
	"strings"
)

// DeviceInfo chứa thông tin thiết bị & trình duyệt được trích xuất từ User-Agent.
//
// Mục tiêu: từ 1 User-Agent thô, suy ra được:
//   - Loại thiết bị (mobile / tablet / desktop / smarttv)
//   - Hãng sản xuất + tên model THỰC (vd: "SM-A525F" ⇒ "Samsung Galaxy A52")
//   - Hệ điều hành + phiên bản
//   - Trình duyệt + phiên bản + engine
//   - Có phải trình duyệt trong app (Facebook In-App, Messenger, Zalo...) hay không
type DeviceInfo struct {
	DeviceType     string `json:"device_type"`
	Brand          string `json:"brand"`
	Model          string `json:"model"`
	ModelCode      string `json:"model_code"`
	OS             string `json:"os"`
	OSVersion      string `json:"os_version"`
	Browser        string `json:"browser"`
	BrowserVersion string `json:"browser_version"`
	Engine         string `json:"engine"`
	IsInAppBrowser bool   `json:"is_in_app_browser"`
	InAppType      string `json:"in_app_type"` // facebook | instagram | messenger | zalo | tiktok | unknown
	Platform       string `json:"platform"`
}

// ---------------------------------------------------------------------------
// Regex trích xuất
// ---------------------------------------------------------------------------

var (
	reAndroidVersion = regexp.MustCompile(`Android[\s/]+([0-9][0-9.]*)`)
	reIOSVersion     = regexp.MustCompile(`(?:iPhone|iPad|iPod).*?OS[\s_]+([0-9_]+)`)
	reIOSVersionAlt  = regexp.MustCompile(`CPU(?: iPhone)? OS ([0-9_]+)`)
	reWindowsVersion = regexp.MustCompile(`Windows NT ([0-9.]+)`)
	reMacVersion     = regexp.MustCompile(`Mac OS X ([0-9_.]+)`)
	reChromeVer      = regexp.MustCompile(`Chrome/([0-9.]+)`)
	reFirefoxVer     = regexp.MustCompile(`Firefox/([0-9.]+)`)
	reSafariVer      = regexp.MustCompile(`Version/([0-9.]+).*Safari`)
	reEdgeVer        = regexp.MustCompile(`Edg(?:e|A|iOS)?/([0-9.]+)`)
	reOPRVer         = regexp.MustCompile(`OPR/([0-9.]+)`)
	reSamsungVer     = regexp.MustCompile(`SamsungBrowser/([0-9.]+)`)
	reUCBrowserVer   = regexp.MustCompile(`UCBrowser/([0-9.]+)`)
	reOPPOSVer       = regexp.MustCompile(`(?:OPPO ?)?Browser/([0-9.]+)`)
	reVivoVer        = regexp.MustCompile(`(?:VIVO ?)?Browser/([0-9.]+)`)
	reMiuiVer        = regexp.MustCompile(`MiuiBrowser/([0-9.]+)`)
	reHuaweiVer      = regexp.MustCompile(`(?:Huawei ?)?Browser/([0-9.]+)`)
	reAndroidModel   = regexp.MustCompile(`Android [0-9.]+; ([^;)]+?)(?: Build/|[;)])`)
)

// windowsVersionMap ánh xạ "Windows NT x.y" ⇒ tên Windows thân thiện
var windowsVersionMap = map[string]string{
	"10.0": "Windows 10/11",
	"6.3":  "Windows 8.1",
	"6.2":  "Windows 8",
	"6.1":  "Windows 7",
	"6.0":  "Windows Vista",
	"5.1":  "Windows XP",
}

// ---------------------------------------------------------------------------
// Bảng tra cấu hình thiết bị phổ biến tại Việt Nam
// ---------------------------------------------------------------------------

// samsungModelMap: mã model kỹ thuật ⇒ tên thương mại
var samsungModelMap = map[string]string{
	"SM-A016": "Galaxy A01", "SM-A026": "Galaxy A02s", "SM-A105": "Galaxy A10",
	"SM-A125": "Galaxy A12", "SM-A135": "Galaxy A13", "SM-A145": "Galaxy A14",
	"SM-A155": "Galaxy A15", "SM-A165": "Galaxy A16", "SM-A215": "Galaxy A21",
	"SM-A225": "Galaxy A22", "SM-A235": "Galaxy A23", "SM-A245": "Galaxy A24",
	"SM-A255": "Galaxy A25", "SM-A305": "Galaxy A30", "SM-A315": "Galaxy A31",
	"SM-A325": "Galaxy A32", "SM-A336": "Galaxy A33", "SM-A346": "Galaxy A34",
	"SM-A356": "Galaxy A35", "SM-A366": "Galaxy A36", "SM-A525": "Galaxy A52",
	"SM-A526": "Galaxy A52 5G", "SM-A536": "Galaxy A53", "SM-A546": "Galaxy A54",
	"SM-A556": "Galaxy A55", "SM-A566": "Galaxy A56", "SM-G973": "Galaxy S10",
	"SM-G975": "Galaxy S10+", "SM-G985": "Galaxy S20", "SM-G991": "Galaxy S21",
	"SM-G996": "Galaxy S21+", "SM-S901": "Galaxy S22", "SM-S906": "Galaxy S22+",
	"SM-S908": "Galaxy S22 Ultra", "SM-S911": "Galaxy S23", "SM-S916": "Galaxy S23+",
	"SM-S918": "Galaxy S23 Ultra", "SM-S921": "Galaxy S24", "SM-S926": "Galaxy S24+",
	"SM-S928": "Galaxy S24 Ultra", "SM-N960": "Galaxy Note 9", "SM-N980": "Galaxy Note 20",
	"SM-N986": "Galaxy Note 20 Ultra", "SM-N970": "Galaxy Note 10", "SM-F711": "Galaxy Z Flip3",
	"SM-F721": "Galaxy Z Flip4", "SM-F731": "Galaxy Z Flip5", "SM-F916": "Galaxy Z Fold3",
	"SM-F926": "Galaxy Z Fold4", "SM-F936": "Galaxy Z Fold5", "SM-F956": "Galaxy Z Fold6",
	"SM-M128": "Galaxy M12", "SM-M135": "Galaxy M13", "SM-M146": "Galaxy M14",
	"SM-A125F": "Galaxy A12", "SM-A146": "Galaxy A14",
}

// xiaomiModelMap: mã model Xiaomi/Redmi/POCO
var xiaomiModelMap = map[string]string{
	"Redmi 9A": "Redmi 9A", "Redmi 9C": "Redmi 9C", "Redmi Note 8": "Redmi Note 8",
	"Redmi Note 9": "Redmi Note 9", "Redmi Note 10": "Redmi Note 10",
	"Redmi Note 11": "Redmi Note 11", "Redmi Note 12": "Redmi Note 12",
	"Redmi Note 13": "Redmi Note 13", "Redmi Note 14": "Redmi Note 14",
	"M2101K6G": "Redmi Note 10 Pro", "M2201": "Redmi Note 11 Pro",
	"M23013RK75": "Redmi Note 13 Pro", "2201116SG": "Redmi Note 11 Pro 5G",
	"22041219G": "Redmi Note 11 Pro+ 5G", "23013RK75C": "Redmi Note 13 Pro+ 5G",
	"M2101K6R": "Redmi Note 10S", "M2003J15SC": "Redmi Note 9 Pro",
	"M2004J2SC": "Redmi Note 9 4G", "M2007J17C": "Redmi Note 9S",
	"2201123G": "Xiaomi 12", "2201122G": "Xiaomi 12 Pro", "2211133C": "Xiaomi 13",
	"2210132C": "Xiaomi 13 Pro", "23013RK75": "Redmi Note 13 Pro",
	"23021RAA2G": "Xiaomi 14", "2312DRA50C": "Xiaomi 14 Ultra",
	"220712": "Poco X5 Pro", "22111320G": "Poco X5 Pro 5G",
	"220333QAG": "Poco F5", "23013PC75G": "Poco X5 Pro 5G",
}

// oppoModelMap: mã model OPPO (CPHxxxx / PJZ110)
var oppoModelMap = map[string]string{
	"CPH1719": "OPPO A74 5G", "CPH1829": "OPPO A78 5G", "CPH1871": "OPPO A79 5G",
	"CPH2211": "OPPO Reno8", "CPH2215": "OPPO Reno8 Z", "CPH2349": "OPPO A78",
	"CPH2461": "OPPO A57", "CPH2477": "OPPO A77 5G", "CPH2551": "OPPO Reno10",
	"CPH2371": "OPPO A57", "CPH2205": "OPPO A76", "CPH2075": "OPPO A55",
}

// vivoModelMap: mã model Vivo
var vivoModelMap = map[string]string{
	"V2111": "Vivo Y21", "V2027": "Vivo Y20", "V2054": "Vivo Y15s",
	"V2022": "Vivo Y33s", "V2118": "Vivo Y33s", "V2129": "Vivo Y22",
	"V2145": "Vivo Y35", "V2201": "Vivo Y22", "V2237": "Vivo V25",
	"V2302": "Vivo V27", "V2307": "Vivo V27 Pro", "V2338": "Vivo V29",
}

// realmeModelMap: mã model Realme (RMXxxx)
var realmeModelMap = map[string]string{
	"RMX3231": "Realme C25", "RMX3261": "Realme C30", "RMX3363": "Realme 9i",
	"RMX3388": "Realme 9 Pro+ 5G", "RMX3421": "Realme C35",
	"RMX3521": "Realme 10 Pro+ 5G", "RMX3560": "Realme 10 5G",
	"RMX3616": "Realme 10 Pro 5G", "RMX3660": "Realme 11 Pro+ 5G",
	"RMX3782": "Realme 11 Pro 5G",
}

// huaweiModelMap: mã model Huawei/Honor
var huaweiModelMap = map[string]string{
	"ANE-NX9": "Huawei P40", "ELS-NX9": "Huawei P40 Pro", "HRY-NX9": "Huawei P30 Pro",
	"VOG-L29": "Huawei P30 Pro", "ANE-LX1": "Huawei P40", "MAR-LX9": "Huawei P20 Pro",
	"ELS-AL00": "Huawei P40 Pro", "NAM-LX9": "Huawei P30", "OEA-LX9": "Huawei P30 Lite",
	"CRR-UL00": "Honor 10", "BVL-AL00": "Honor X10", "WHL-AL00": "Honor Play 4",
}

// nokiaModelMap: mã model Nokia
var nokiaModelMap = map[string]string{
	"TA-1333": "Nokia G11 Plus", "TA-1487": "Nokia G21", "TA-1384": "Nokia G50",
	"TA-1506": "Nokia G42", "TA-1337": "Nokia G11", "TA-1403": "Nokia C31",
}

// genericAndroidBrandMap: nhận diện hãng từ tiền tố mã model
var genericAndroidBrandMap = map[string]string{
	"SM-":   "Samsung",
	"GT-":   "Samsung",
	"SHV":   "Samsung",
	"SHW":   "Samsung",
	"CPH":   "OPPO",
	"PJA":   "OPPO",
	"PJZ":   "OPPO",
	"RMX":   "Realme",
	"Redmi": "Xiaomi",
	"MIX ":  "Xiaomi",
	"MI ":   "Xiaomi",
	"POCO":  "Xiaomi",
	"2207":  "Xiaomi",
	"2211":  "Xiaomi",
	"2212":  "Xiaomi",
	"2302":  "Xiaomi",
	"23013": "Xiaomi",
	"2112":  "Xiaomi",
	"V20":   "Vivo",
	"V21":   "Vivo",
	"V22":   "Vivo",
	"V23":   "Vivo",
	"V205":  "Vivo",
	"V211":  "Vivo",
	"TK-":   "Motorola",
	"moto":  "Motorola",
	"XT-":   "Motorola",
	"ANE-":  "Huawei",
	"ELS-":  "Huawei",
	"HRY-":  "Huawei",
	"VOG-":  "Huawei",
	"MAR-":  "Huawei",
	"TA-":   "Nokia",
	"Nokia": "Nokia",
	"CPH-":  "OPPO",
	"M2101": "Xiaomi",
	"M2201": "Xiaomi",
	"M2301": "Xiaomi",
	"2201":  "Xiaomi",
	"2204":  "Xiaomi",
	"2209":  "Xiaomi",
	"23021": "Xiaomi",
}

// ---------------------------------------------------------------------------
// Hàm chính
// ---------------------------------------------------------------------------

// ParseDeviceInfo phân tích User-Agent ⇒ thông tin thiết bị & trình duyệt chi tiết.
func ParseDeviceInfo(ua string) *DeviceInfo {
	info := &DeviceInfo{
		DeviceType: "desktop",
		Brand:      "",
		Model:      "",
		ModelCode:  "",
		OS:         "Other",
		Browser:    "Other",
		Engine:     "",
		Platform:   "",
	}

	ua = strings.TrimSpace(ua)
	if ua == "" {
		return info
	}
	uaLower := strings.ToLower(ua)

	// ------------------------------------------------------------------
	// 1. Trình duyệt trong app (Facebook / Instagram / Messenger / Zalo...)
	// ------------------------------------------------------------------
	// ⚠️ PHẢI KIỂM TRA TRƯỚC vì UA của Facebook In-App chứa cả "Chrome/"
	//    và "Safari/" ⇒ nếu kiểm tra Chrome trước sẽ nhận nhầm là Chrome.
	switch {
	case strings.Contains(uaLower, "fb_iab"):
		info.IsInAppBrowser = true
		info.InAppType = "facebook"
	case strings.Contains(uaLower, "fban") || strings.Contains(uaLower, "fbav"):
		info.IsInAppBrowser = true
		info.InAppType = "facebook"
	case strings.Contains(uaLower, "instagram"):
		info.IsInAppBrowser = true
		info.InAppType = "instagram"
	case strings.Contains(uaLower, "messenger"):
		info.IsInAppBrowser = true
		info.InAppType = "messenger"
	case strings.Contains(uaLower, "zalo"):
		info.IsInAppBrowser = true
		info.InAppType = "zalo"
	case strings.Contains(uaLower, "tiktok") || strings.Contains(uaLower, "bytedancewebview"):
		info.IsInAppBrowser = true
		info.InAppType = "tiktok"
	case strings.Contains(uaLower, "line/"):
		info.IsInAppBrowser = true
		info.InAppType = "line"
	case strings.Contains(uaLower, "coccoc"):
		info.IsInAppBrowser = true
		info.InAppType = "coccoc"
	case strings.Contains(uaLower, "trident/"), strings.Contains(uaLower, "iemobile"):
		info.IsInAppBrowser = true
		info.InAppType = "explorer-mobile"
	}

	// ------------------------------------------------------------------
	// 2. Loại thiết bị
	// ------------------------------------------------------------------
	switch {
	case strings.Contains(uaLower, "ipad"), strings.Contains(uaLower, "tablet"),
		strings.Contains(uaLower, "playbook"), strings.Contains(uaLower, "kindle"),
		strings.Contains(uaLower, "silk/"), strings.Contains(uaLower, "sm-t"):
		info.DeviceType = "tablet"
	case strings.Contains(uaLower, "mobi"), strings.Contains(uaLower, "mobile"),
		strings.Contains(uaLower, "iphone"), strings.Contains(uaLower, "ipod"),
		strings.Contains(uaLower, "android"), strings.Contains(uaLower, "windows phone"),
		strings.Contains(uaLower, "opera mini"), strings.Contains(uaLower, "opera mobi"):
		info.DeviceType = "mobile"
	case strings.Contains(uaLower, "smart-tv"), strings.Contains(uaLower, "smarttv"),
		strings.Contains(uaLower, "appletv"), strings.Contains(uaLower, "googletv"),
		strings.Contains(uaLower, "hbbtv"), strings.Contains(uaLower, "netcast"),
		strings.Contains(uaLower, "web0s"), strings.Contains(uaLower, "aft"):
		info.DeviceType = "smarttv"
	}

	// ------------------------------------------------------------------
	// 3. Hệ điều hành + phiên bản
	// ------------------------------------------------------------------
	switch {
	case strings.Contains(uaLower, "android"):
		info.OS = "Android"
		if m := reAndroidVersion.FindStringSubmatch(ua); len(m) > 1 {
			info.OSVersion = m[1]
		}
	case strings.Contains(uaLower, "iphone"), strings.Contains(uaLower, "ipad"),
		strings.Contains(uaLower, "ipod"), strings.Contains(uaLower, "ios"):
		info.OS = "iOS"
		if m := reIOSVersion.FindStringSubmatch(ua); len(m) > 1 {
			info.OSVersion = strings.ReplaceAll(m[1], "_", ".")
		} else if m := reIOSVersionAlt.FindStringSubmatch(ua); len(m) > 1 {
			info.OSVersion = strings.ReplaceAll(m[1], "_", ".")
		}
	case strings.Contains(uaLower, "windows"):
		info.OS = "Windows"
		if m := reWindowsVersion.FindStringSubmatch(ua); len(m) > 1 {
			if name, ok := windowsVersionMap[m[1]]; ok {
				info.OS = name
			}
			info.OSVersion = m[1]
		}
	case strings.Contains(uaLower, "mac os x"), strings.Contains(uaLower, "macintosh"):
		info.OS = "macOS"
		if m := reMacVersion.FindStringSubmatch(ua); len(m) > 1 {
			info.OSVersion = strings.ReplaceAll(m[1], "_", ".")
		}
	case strings.Contains(uaLower, "cros "):
		info.OS = "ChromeOS"
	case strings.Contains(uaLower, "linux"):
		info.OS = "Linux"
	}

	// ------------------------------------------------------------------
	// 4. Trình duyệt + phiên bản + engine
	// ------------------------------------------------------------------
	switch {
	case info.IsInAppBrowser:
		// Browser trong app — dùng nhãn riêng cho dễ đọc trong báo cáo
		switch info.InAppType {
		case "facebook":
			info.Browser = "Facebook In-App"
		case "instagram":
			info.Browser = "Instagram In-App"
		case "messenger":
			info.Browser = "Messenger In-App"
		case "zalo":
			info.Browser = "Zalo In-App"
		case "tiktok":
			info.Browser = "TikTok In-App"
		case "line":
			info.Browser = "LINE In-App"
		case "coccoc":
			info.Browser = "Cốc Cốc"
		default:
			info.Browser = "In-App Browser"
		}
		// Phiên bản: thường nằm ở FBAN/FBAV/Instagram
		if m := regexp.MustCompile(`FBAN(?:/FBIOS)?/([0-9._]+)`).FindStringSubmatch(ua); len(m) > 1 {
			info.BrowserVersion = m[1]
		} else if m := regexp.MustCompile(`Instagram ([0-9._]+)`).FindStringSubmatch(ua); len(m) > 1 {
			info.BrowserVersion = m[1]
		} else if m := reChromeVer.FindStringSubmatch(ua); len(m) > 1 {
			info.BrowserVersion = m[1]
		}
		info.Engine = "Blink"

	case strings.Contains(uaLower, "edg/"), strings.Contains(uaLower, "edge/"),
		strings.Contains(uaLower, "edga/"), strings.Contains(uaLower, "edgios/"):
		info.Browser = "Edge"
		if m := reEdgeVer.FindStringSubmatch(ua); len(m) > 1 {
			info.BrowserVersion = m[1]
		}
		info.Engine = "Blink"

	case strings.Contains(uaLower, "opr/"), strings.Contains(uaLower, "opera"):
		info.Browser = "Opera"
		if m := reOPRVer.FindStringSubmatch(ua); len(m) > 1 {
			info.BrowserVersion = m[1]
		} else if m := reChromeVer.FindStringSubmatch(ua); len(m) > 1 {
			info.BrowserVersion = m[1]
		}
		info.Engine = "Blink"

	case strings.Contains(uaLower, "samsungbrowser"):
		info.Browser = "Samsung Internet"
		if m := reSamsungVer.FindStringSubmatch(ua); len(m) > 1 {
			info.BrowserVersion = m[1]
		}
		info.Engine = "Blink"

	case strings.Contains(uaLower, "miuibrowser"):
		info.Browser = "Mi Browser"
		if m := reMiuiVer.FindStringSubmatch(ua); len(m) > 1 {
			info.BrowserVersion = m[1]
		}
		info.Engine = "Blink"

	case strings.Contains(uaLower, "ucbrowser"):
		info.Browser = "UC Browser"
		if m := reUCBrowserVer.FindStringSubmatch(ua); len(m) > 1 {
			info.BrowserVersion = m[1]
		}
		info.Engine = "Blink"

	case strings.Contains(uaLower, "huaweibrowser"):
		info.Browser = "Huawei Browser"
		if m := reHuaweiVer.FindStringSubmatch(ua); len(m) > 1 {
			info.BrowserVersion = m[1]
		}
		info.Engine = "Blink"

	case strings.Contains(uaLower, "oppo browser"), strings.Contains(uaLower, "oppobrowser"):
		info.Browser = "OPPO Browser"
		if m := reOPPOSVer.FindStringSubmatch(ua); len(m) > 1 {
			info.BrowserVersion = m[1]
		}
		info.Engine = "Blink"

	case strings.Contains(uaLower, "vivo browser"), strings.Contains(uaLower, "vivobrowser"):
		info.Browser = "Vivo Browser"
		if m := reVivoVer.FindStringSubmatch(ua); len(m) > 1 {
			info.BrowserVersion = m[1]
		}
		info.Engine = "Blink"

	case strings.Contains(uaLower, "firefox"):
		info.Browser = "Firefox"
		if m := reFirefoxVer.FindStringSubmatch(ua); len(m) > 1 {
			info.BrowserVersion = m[1]
		}
		info.Engine = "Gecko"

	case strings.Contains(uaLower, "coc_coc_browser"), strings.Contains(uaLower, "coccoc"):
		info.Browser = "Cốc Cốc"
		if m := reChromeVer.FindStringSubmatch(ua); len(m) > 1 {
			info.BrowserVersion = m[1]
		}
		info.Engine = "Blink"

	case strings.Contains(uaLower, "chrome"), strings.Contains(uaLower, "crios"):
		info.Browser = "Chrome"
		if m := reChromeVer.FindStringSubmatch(ua); len(m) > 1 {
			info.BrowserVersion = m[1]
		}
		info.Engine = "Blink"

	case strings.Contains(uaLower, "safari"):
		info.Browser = "Safari"
		if m := reSafariVer.FindStringSubmatch(ua); len(m) > 1 {
			info.BrowserVersion = m[1]
		}
		info.Engine = "WebKit"
	}

	// ------------------------------------------------------------------
	// 5. Model thiết bị
	// ------------------------------------------------------------------
	fillModel(info, ua)

	return info
}

// fillModel suy ra hãng + model thiết bị từ User-Agent.
func fillModel(info *DeviceInfo, ua string) {
	// 5a. iPhone / iPad — model chính xác từ mã phần cứng
	if info.OS == "iOS" {
		info.Brand = "Apple"
		if model := detectAppleModel(ua); model != "" {
			info.Model = model
			info.ModelCode = model
		} else {
			info.Model = "iPhone/iPad"
		}
		return
	}

	// 5b. Android — lấy mã model trong UA
	modelCode := ""
	if m := reAndroidModel.FindStringSubmatch(ua); len(m) > 1 {
		modelCode = strings.TrimSpace(m[1])
	}
	if modelCode == "" {
		// Thử pattern "Build/<model>" hoặc model trực tiếp sau dấu ")"
		if m := regexp.MustCompile(`Build/([A-Za-z0-9._\-]+)`).FindStringSubmatch(ua); len(m) > 1 {
			modelCode = strings.TrimSpace(m[1])
		}
	}
	if modelCode == "" {
		if info.DeviceType == "mobile" {
			info.Brand = "Android"
			info.Model = "Android Unknown"
		}
		return
	}

	info.ModelCode = modelCode

	// Tra bản đồ theo hãng
	switch {
	case strings.HasPrefix(modelCode, "SM-"):
		info.Brand = "Samsung"
		if name, ok := samsungModelMap[modelCode]; ok {
			info.Model = name
		} else if len(modelCode) > 3 {
			// SM-A525F ⇒ tìm tiền tố SM-A525
			for k, v := range samsungModelMap {
				if strings.HasPrefix(modelCode, k) {
					info.Model = v
					break
				}
			}
		}
	case strings.HasPrefix(modelCode, "CPH"):
		info.Brand = "OPPO"
		if name, ok := oppoModelMap[modelCode]; ok {
			info.Model = name
		}
	case strings.HasPrefix(modelCode, "RMX"):
		info.Brand = "Realme"
		if name, ok := realmeModelMap[modelCode]; ok {
			info.Model = name
		}
	case strings.HasPrefix(modelCode, "ANE"), strings.HasPrefix(modelCode, "ELS"),
		strings.HasPrefix(modelCode, "HRY"), strings.HasPrefix(modelCode, "VOG"),
		strings.HasPrefix(modelCode, "MAR"), strings.HasPrefix(modelCode, "OEA"):
		info.Brand = "Huawei"
		for k, v := range huaweiModelMap {
			if strings.HasPrefix(modelCode, k) {
				info.Model = v
				break
			}
		}
	case strings.HasPrefix(modelCode, "TA-"):
		info.Brand = "Nokia"
		for k, v := range nokiaModelMap {
			if strings.HasPrefix(modelCode, k) {
				info.Model = v
				break
			}
		}
	case strings.HasPrefix(modelCode, "V2"), strings.HasPrefix(modelCode, "V1"):
		info.Brand = "Vivo"
		for k, v := range vivoModelMap {
			if strings.HasPrefix(modelCode, k) {
				info.Model = v
				break
			}
		}
	case strings.Contains(modelCode, "Redmi"), strings.Contains(modelCode, "POCO"),
		strings.Contains(modelCode, "MI "), strings.Contains(modelCode, "MIX "),
		regexp.MustCompile(`^(M2|220|221|230|210|211|231)\d`).MatchString(modelCode):
		info.Brand = "Xiaomi"
		if name, ok := xiaomiModelMap[modelCode]; ok {
			info.Model = name
		} else {
			for k, v := range xiaomiModelMap {
				if strings.HasPrefix(modelCode, k) {
					info.Model = v
					break
				}
			}
		}
	default:
		// Nhận diện hãng theo tiền tố
		for prefix, brand := range genericAndroidBrandMap {
			if strings.HasPrefix(modelCode, prefix) {
				info.Brand = brand
				break
			}
		}
		if info.Brand == "" && strings.Contains(modelCode, "Pixel") {
			info.Brand = "Google"
		}
	}

	if info.Model == "" {
		info.Model = modelCode
	}
	if info.Brand == "" {
		info.Brand = "Khác"
	}
}

// appleModelMap: mã phần cứng iOS ⇒ tên thương mại
var appleModelMap = map[string]string{
	"iPhone10,3": "iPhone X", "iPhone10,6": "iPhone X", "iPhone11,2": "iPhone XS",
	"iPhone11,4": "iPhone XS Max", "iPhone11,8": "iPhone XR", "iPhone12,1": "iPhone 11",
	"iPhone12,3": "iPhone 11 Pro", "iPhone12,5": "iPhone 11 Pro Max", "iPhone12,8": "iPhone SE 2",
	"iPhone13,1": "iPhone 12 mini", "iPhone13,2": "iPhone 12", "iPhone13,3": "iPhone 12 Pro",
	"iPhone13,4": "iPhone 12 Pro Max", "iPhone14,2": "iPhone 13 Pro", "iPhone14,3": "iPhone 13 Pro Max",
	"iPhone14,4": "iPhone 13 mini", "iPhone14,5": "iPhone 13", "iPhone14,6": "iPhone SE 3",
	"iPhone14,7": "iPhone 14", "iPhone14,8": "iPhone 14 Plus", "iPhone15,2": "iPhone 14 Pro",
	"iPhone15,3": "iPhone 14 Pro Max", "iPhone15,4": "iPhone 15", "iPhone15,5": "iPhone 15 Plus",
	"iPhone16,1": "iPhone 15 Pro", "iPhone16,2": "iPhone 15 Pro Max",
	"iPhone17,1": "iPhone 16 Pro", "iPhone17,2": "iPhone 16 Pro Max", "iPhone17,3": "iPhone 16",
	"iPhone17,4": "iPhone 16 Plus", "iPhone17,5": "iPhone 16e",
	"iPad13,1": "iPad Air 3", "iPad13,2": "iPad Air 3", "iPad13,4": "iPad Pro 11\"",
	"iPad13,5": "iPad Pro 11\"", "iPad13,6": "iPad Pro 11\"", "iPad13,7": "iPad Pro 11\"",
	"iPad13,8": "iPad Pro 12.9\"", "iPad13,9": "iPad Pro 12.9\"", "iPad13,10": "iPad Pro 12.9\"",
	"iPad13,11": "iPad Pro 12.9\"", "iPad14,1": "iPad mini 5", "iPad14,2": "iPad mini 5",
	"iPad14,3": "iPad Pro 11\" 4th", "iPad14,4": "iPad Pro 11\" 4th",
}

// detectAppleModel tra mã phần cứng iOS (vd: iPhone15,4) ⇒ tên thiết bị.
func detectAppleModel(ua string) string {
	// Mã phần cứng nằm trong dạng "CPU iPhone OS 17_1 like Mac OS X)" hoặc
	// "CPU OS 16_6 like Mac OS X)" — không có mã model trực tiếp trong UA iOS.
	//
	// ⇒ Dùng "iPhone OS <phiên bản>" để suy đoán dải model phổ biến tại VN.
	ver := ""
	if m := reIOSVersionAlt.FindStringSubmatch(ua); len(m) > 1 {
		ver = strings.ReplaceAll(m[1], "_", ".")
	} else if m := reIOSVersion.FindStringSubmatch(ua); len(m) > 1 {
		ver = strings.ReplaceAll(m[1], "_", ".")
	}
	if ver == "" {
		return ""
	}
	major := ver
	if idx := strings.Index(ver, "."); idx != -1 {
		major = ver[:idx]
	}
	majNum, err := strconv.Atoi(major)
	if err != nil {
		return ""
	}

	// Bảng đồ phiên bản iOS ⇒ dòng iPhone phổ biến nhất tại Việt Nam
	iosToModel := map[int]string{
		18: "iPhone 16", 17: "iPhone 15", 16: "iPhone 14",
		15: "iPhone 13", 14: "iPhone 12", 13: "iPhone 11",
		12: "iPhone XS", 11: "iPhone X", 10: "iPhone 8",
	}
	if name, ok := iosToModel[majNum]; ok {
		return name
	}
	return ""
}

// DeviceDisplayName trả về tên hiển thị gọn cho báo cáo (VD: "Samsung Galaxy A52").
func DeviceDisplayName(info *DeviceInfo) string {
	if info == nil {
		return ""
	}
	if info.Brand != "" && info.Model != "" && !strings.Contains(info.Model, info.Brand) {
		return strings.TrimSpace(info.Brand + " " + info.Model)
	}
	if info.Model != "" {
		return info.Model
	}
	if info.Brand != "" {
		return info.Brand
	}
	return info.OS
}
