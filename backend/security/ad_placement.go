package security

import (
	"net/url"
	"strings"
)

// AdPlacementInfo chứa thông tin nhận diện chi tiết nền tảng và vị trí hiển thị quảng cáo
type AdPlacementInfo struct {
	Platform    string `json:"ad_platform"`  // Meta Ads, Google Ads, TikTok Ads, Zalo Ads, Direct, Referral, Organic
	Placement   string `json:"ad_placement"` // Instagram, Facebook Feed, Audience Network, Messenger, Google Search, GDN, YouTube, PMax, TikTok In-Feed, Pangle, Zalo Feed, Báo Mới / Zing
	SourceTag   string `json:"source_tag"`   // ig, fb, an, msg, g, d, ytv, pmax,...
	Campaign    string `json:"campaign,omitempty"`
	Content     string `json:"content,omitempty"`
}

// ResolveAdPlacement phân tích thông minh URL, UTM tags, query parameters và Referrer để xác định chính xác vị trí quảng cáo
func ResolveAdPlacement(entryURLStr, referrerStr, utmSource, utmMedium, utmCampaign, utmContent string) *AdPlacementInfo {
	info := &AdPlacementInfo{
		Platform:  "Trực Tiếp",
		Placement: "Trực Tiếp (Direct)",
		SourceTag: "direct",
		Campaign:  utmCampaign,
		Content:   utmContent,
	}

	// 1. Phân tích Query Parameters từ entryURL
	queryParams := make(url.Values)
	var urlHost string
	if entryURLStr != "" {
		if parsedURL, err := url.Parse(entryURLStr); err == nil {
			queryParams = parsedURL.Query()
			urlHost = strings.ToLower(parsedURL.Host)
		}
	}

	// Helper lấy query param không phân biệt hoa thường
	getQuery := func(key string) string {
		if val := queryParams.Get(key); val != "" {
			return strings.TrimSpace(val)
		}
		for k, v := range queryParams {
			if strings.EqualFold(k, key) && len(v) > 0 {
				return strings.TrimSpace(v[0])
			}
		}
		return ""
	}

	// 2. Phân tích Referrer Host
	var refHost string
	if referrerStr != "" {
		if parsedRef, err := url.Parse(referrerStr); err == nil {
			refHost = strings.ToLower(parsedRef.Host)
		}
	}

	// Chuẩn hóa utm_source, utm_medium
	srcLower := strings.ToLower(strings.TrimSpace(utmSource))
	medLower := strings.ToLower(strings.TrimSpace(utmMedium))
	campLower := strings.ToLower(strings.TrimSpace(utmCampaign))

	// Trích xuất các query parameters đặc thù của các ad networks
	siteSource := strings.ToLower(getQuery("site_source_name")) // Meta: fb, ig, an, msg
	placementParam := strings.ToLower(getQuery("placement"))     // Meta, TikTok: Instagram_Feed, Facebook_Desktop_Feed, placement_v2
	fbPlacement := strings.ToLower(getQuery("fb_placement"))
	fbclid := getQuery("fbclid")
	igshid := getQuery("igshid")
	network := strings.ToLower(getQuery("network")) // Google: g, d, s, ytv, video, p, pmax
	gclid := getQuery("gclid")
	wbraid := getQuery("wbraid")
	gbraid := getQuery("gbraid")
	ttclid := getQuery("ttclid") // TikTok
	zaloSource := strings.ToLower(getQuery("zalo_source"))
	zarsrc := getQuery("zarsrc")

	// ============================================================
	// A. NHẬN DIỆN META ADS (FACEBOOK / INSTAGRAM / AUDIENCE NETWORK / MESSENGER)
	// ============================================================
	isMeta := fbclid != "" || igshid != "" || siteSource != "" || fbPlacement != "" ||
		strings.Contains(srcLower, "fb") || strings.Contains(srcLower, "facebook") || strings.Contains(srcLower, "meta") ||
		strings.Contains(srcLower, "instagram") || strings.Contains(srcLower, "insta") || strings.Contains(srcLower, "ig") ||
		strings.Contains(refHost, "facebook.com") || strings.Contains(refHost, "instagram.com") ||
		strings.Contains(refHost, "messenger.com") || strings.Contains(refHost, "fb.me")

	if isMeta {
		info.Platform = "Meta Ads"

		// 1. Phân loại Instagram (ig)
		if siteSource == "ig" || igshid != "" || strings.Contains(placementParam, "instagram") ||
			strings.Contains(fbPlacement, "instagram") || strings.Contains(srcLower, "instagram") ||
			strings.Contains(srcLower, "ig") || strings.Contains(refHost, "instagram.com") {

			info.SourceTag = "ig"
			if strings.Contains(placementParam, "stories") || strings.Contains(fbPlacement, "stories") {
				info.Placement = "Instagram Stories"
			} else if strings.Contains(placementParam, "reels") || strings.Contains(fbPlacement, "reels") {
				info.Placement = "Instagram Reels"
			} else if strings.Contains(placementParam, "explore") || strings.Contains(fbPlacement, "explore") {
				info.Placement = "Instagram Explore"
			} else {
				info.Placement = "Instagram Feed"
			}
			return info
		}

		// 2. Phân loại Audience Network (an)
		if siteSource == "an" || strings.Contains(placementParam, "an_") || strings.Contains(placementParam, "audience") ||
			strings.Contains(fbPlacement, "an_") || strings.Contains(refHost, "an.facebook.com") ||
			strings.Contains(srcLower, "audience_network") || strings.Contains(srcLower, "an") {
			info.SourceTag = "an"
			info.Placement = "Audience Network (AN)"
			return info
		}

		// 3. Phân loại Messenger (msg)
		if siteSource == "msg" || strings.Contains(placementParam, "messenger") || strings.Contains(fbPlacement, "messenger") ||
			strings.Contains(refHost, "messenger.com") || strings.Contains(srcLower, "messenger") {
			info.SourceTag = "msg"
			info.Placement = "Facebook Messenger"
			return info
		}

		// 4. Phân loại Facebook Feed / Watch / Marketplace (fb)
		info.SourceTag = "fb"
		if strings.Contains(placementParam, "feed") || strings.Contains(fbPlacement, "feed") {
			info.Placement = "Facebook Feed"
		} else if strings.Contains(placementParam, "video") || strings.Contains(placementParam, "watch") || strings.Contains(fbPlacement, "video") {
			info.Placement = "Facebook Video / Watch"
		} else if strings.Contains(placementParam, "stories") || strings.Contains(fbPlacement, "stories") {
			info.Placement = "Facebook Stories"
		} else if strings.Contains(placementParam, "marketplace") || strings.Contains(fbPlacement, "marketplace") {
			info.Placement = "Facebook Marketplace"
		} else if strings.Contains(placementParam, "right_column") {
			info.Placement = "Facebook Cột Phải"
		} else {
			info.Placement = "Facebook Feed / Tin"
		}
		return info
	}

	// ============================================================
	// B. NHẬN DIỆN GOOGLE ADS (SEARCH / GDN / YOUTUBE / PMAX)
	// ============================================================
	isGoogle := gclid != "" || wbraid != "" || gbraid != "" || network != "" ||
		strings.Contains(srcLower, "google") || strings.Contains(srcLower, "adwords") ||
		strings.Contains(medLower, "cpc") || strings.Contains(medLower, "gdn") ||
		strings.Contains(refHost, "googleads") || strings.Contains(refHost, "doubleclick") ||
		strings.Contains(refHost, "youtube.com") || strings.Contains(refHost, "googlesyndication.com")

	if isGoogle {
		info.Platform = "Google Ads"

		// 1. YouTube Video Ads
		if network == "ytv" || network == "video" || strings.Contains(refHost, "youtube.com") ||
			strings.Contains(srcLower, "youtube") || strings.Contains(campLower, "youtube") {
			info.SourceTag = "ytv"
			info.Placement = "YouTube Video Ads"
			return info
		}

		// 2. Google Display Network (GDN / Banner)
		if network == "d" || strings.Contains(medLower, "gdn") || strings.Contains(medLower, "display") ||
			strings.Contains(refHost, "doubleclick.net") || strings.Contains(refHost, "googlesyndication.com") {
			info.SourceTag = "d"
			info.Placement = "Google Display Network (GDN)"
			return info
		}

		// 3. Performance Max (PMax)
		if network == "p" || network == "pmax" || strings.Contains(campLower, "pmax") || strings.Contains(campLower, "performance_max") {
			info.SourceTag = "pmax"
			info.Placement = "Google Performance Max (PMax)"
			return info
		}

		// 4. Search Partners
		if network == "s" {
			info.SourceTag = "s"
			info.Placement = "Google Search Partners"
			return info
		}

		// 5. Google Search
		if network == "g" || gclid != "" || strings.Contains(medLower, "cpc") || strings.Contains(campLower, "search") {
			info.SourceTag = "g"
			info.Placement = "Google Tìm Kiếm (Search)"
			return info
		}

		info.SourceTag = "google"
		info.Placement = "Google Ads (Chung)"
		return info
	}

	// ============================================================
	// C. NHẬN DIỆN TIKTOK ADS (IN-FEED VS PANGLE)
	// ============================================================
	isTikTok := ttclid != "" || strings.Contains(srcLower, "tiktok") || strings.Contains(refHost, "tiktok.com") ||
		strings.Contains(srcLower, "bytedance")

	if isTikTok {
		info.Platform = "TikTok Ads"
		if placementParam == "placement_v2" || strings.Contains(placementParam, "pangle") || strings.Contains(srcLower, "pangle") {
			info.SourceTag = "pangle"
			info.Placement = "TikTok Pangle (App Network)"
		} else {
			info.SourceTag = "in_feed"
			info.Placement = "TikTok In-Feed Ads"
		}
		return info
	}

	// ============================================================
	// D. NHẬN DIỆN ZALO ADS (FEED VS BÁO MỚI / ZING)
	// ============================================================
	isZalo := zarsrc != "" || zaloSource != "" || strings.Contains(srcLower, "zalo") ||
		strings.Contains(refHost, "zalo.me") || strings.Contains(refHost, "baomoi.com") ||
		strings.Contains(refHost, "zingnews.vn") || strings.Contains(refHost, "znews.vn")

	if isZalo {
		info.Platform = "Zalo Ads"
		if zaloSource == "article" || strings.Contains(refHost, "baomoi.com") || strings.Contains(refHost, "zingnews.vn") || strings.Contains(refHost, "znews.vn") {
			info.SourceTag = "article"
			info.Placement = "Zalo Network (Báo Mới / Zing)"
		} else {
			info.SourceTag = "feed"
			info.Placement = "Zalo Feed Ads"
		}
		return info
	}

	// ============================================================
	// E. TÌM KIẾM TỰ NHIÊN (ORGANIC SEARCH)
	// ============================================================
	if strings.Contains(refHost, "google.") {
		info.Platform = "Tìm Kiếm Tự Nhiên"
		info.Placement = "Google Search Organic"
		info.SourceTag = "google_organic"
		return info
	}
	if strings.Contains(refHost, "coccoc.com") {
		info.Platform = "Tìm Kiếm Tự Nhiên"
		info.Placement = "Cốc Cốc Search"
		info.SourceTag = "coccoc_organic"
		return info
	}

	// ============================================================
	// F. MẠNG XÃ HỘI KHÁC HOẶC GIỚI THIỆU (REFERRAL)
	// ============================================================
	if refHost != "" && !strings.Contains(refHost, urlHost) {
		info.Platform = "Liên Kết Ngoài (Referral)"
		info.Placement = refHost
		info.SourceTag = "referral"
		return info
	}

	if srcLower != "" && srcLower != "direct" {
		info.Platform = "Chiến Dịch Khác"
		info.Placement = srcLower
		info.SourceTag = srcLower
		return info
	}

	// G. Trực tiếp
	return info
}
