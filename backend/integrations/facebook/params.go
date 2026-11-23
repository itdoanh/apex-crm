package facebook

import (
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ─── Phân tích URL quảng cáo Meta theo "URL Parameters" của Ads Manager ──
//
// Mẫu URL đang dùng trong chiến dịch:
//
//	utm_medium=paid
//	&utm_source={{site_source_name}}
//	&utm_placement={{placement}}
//	&utm_campaign={{campaign.id}}
//	&utm_content={{adset.id}}
//	&utm_term={{ad.id}}
//	&campaign_name={{campaign.name}}
//	&adset_name={{adset.name}}
//	&ad_name={{ad.name}}
//	&fbclid=...
//	&utm_id={{campaign.id}}
//
// Lưu ý ánh xạ KHÔNG theo quy ước Google Ads:
//   - utm_campaign  = CAMPAIGN ID  (không phải tên chiến dịch)
//   - utm_content   = ADSET ID     (không phải tên adset)
//   - utm_term      = AD ID        (không phải tên ad)
//   - campaign_name / adset_name / ad_name là TÊN hiển thị
//
// Meta Ads chỉ đẩy tên chiến dịch/adset/ad khi bật "Include names in URL".
// Nếu không bật, các trường tên sẽ rỗng → báo cáo sẽ hiển thị ID.

const (
	UTMSource      = "utm_source"
	UTMMedium      = "utm_medium"
	UTMCampaign    = "utm_campaign"
	UTMContent     = "utm_content"
	UTMTerm        = "utm_term"
	UTMPlacement   = "utm_placement"
	UTMID          = "utm_id"
	CampaignName   = "campaign_name"
	AdSetName      = "adset_name"
	AdName         = "ad_name"
	Fbclid         = "fbclid"
	Igshid         = "igshid"
	FBC            = "fbc"
	FBP            = "fbp"
	SiteSourceName = "site_source_name"
)

// MetaAdsParams là bộ thông số Meta Ads đã phân tích từ URL.
type MetaAdsParams struct {
	Source         string `json:"utm_source"`
	Medium         string `json:"utm_medium"`
	CampaignID     string `json:"campaign_id"`
	AdSetID        string `json:"adset_id"`
	AdID           string `json:"ad_id"`
	Placement      string `json:"placement"`
	UTMID          string `json:"utm_id"`
	CampaignName   string `json:"campaign_name"`
	AdSetName      string `json:"adset_name"`
	AdName         string `json:"ad_name"`
	Fbclid         string `json:"fbclid"`
	Igshid         string `json:"igshid"`
	SiteSourceName string `json:"site_source_name"`
}

// ParseMetaAdsURL phân tích URL landing (kèm query string) thành bộ thông số Meta Ads.
//
// Hàm nhận cả URL đầy đủ lẫn chỉ query string.
func ParseMetaAdsURL(rawURL string) MetaAdsParams {
	p := MetaAdsParams{}
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return p
	}

	q := url.Values{}
	if u, err := url.Parse(rawURL); err == nil && u.RawQuery != "" {
		q = u.Query()
	} else if strings.Contains(rawURL, "=") {
		// Chỉ có query string (không có scheme/host) → parse thủ công.
		if parsed, err := url.ParseQuery(strings.TrimPrefix(rawURL, "?")); err == nil {
			q = parsed
		}
	}

	get := func(keys ...string) string {
		for _, k := range keys {
			if v := strings.TrimSpace(q.Get(k)); v != "" {
				return v
			}
		}
		return ""
	}

	p.Source = get(UTMSource)
	p.Medium = get(UTMMedium)
	p.Placement = get(UTMPlacement)
	p.UTMID = get(UTMID)
	p.CampaignName = get(CampaignName)
	p.AdSetName = get(AdSetName)
	p.AdName = get(AdName)
	p.Fbclid = get(Fbclid)
	p.Igshid = get(Igshid)
	p.SiteSourceName = get(SiteSourceName)

	// utm_campaign = campaign.id ; utm_content = adset.id ; utm_term = ad.id
	p.CampaignID = get(UTMCampaign)
	p.AdSetID = get(UTMContent)
	p.AdID = get(UTMTerm)

	// utm_id thường trùng campaign.id → dùng làm fallback.
	if p.CampaignID == "" {
		p.CampaignID = p.UTMID
	}
	// site_source_name là nguồn thật (facebook/instagram/audience_network...).
	if p.SiteSourceName == "" {
		p.SiteSourceName = p.Source
	}
	return p
}

// DisplayCampaign trả về tên chiến dịch để hiển thị, fallback sang ID.
func (p MetaAdsParams) DisplayCampaign() string {
	if p.CampaignName != "" {
		return p.CampaignName
	}
	return p.CampaignID
}

// DisplayAdSet trả về tên adset để hiển thị, fallback sang ID.
func (p MetaAdsParams) DisplayAdSet() string {
	if p.AdSetName != "" {
		return p.AdSetName
	}
	return p.AdSetID
}

// DisplayAd trả về tên ad để hiển thị, fallback sang ID.
func (p MetaAdsParams) DisplayAd() string {
	if p.AdName != "" {
		return p.AdName
	}
	return p.AdID
}

// HasCampaign trả về true nếu URL có thông tin chiến dịch Meta.
func (p MetaAdsParams) HasCampaign() bool {
	return p.CampaignID != "" || p.CampaignName != ""
}

// ClickID trả về định danh lượt nhấp ưu tiên (fbclid > igshid).
func (p MetaAdsParams) ClickID() string {
	if p.Fbclid != "" {
		return p.Fbclid
	}
	return p.Igshid
}

// NormalizeFBC chuẩn hoá `_fbc` theo định dạng Meta: fb.1.<timestamp>.<fbclid>.
//
// Meta chỉ chấp nhận `_fbc` đúng định dạng. Nếu client gửi `fbclid` thô
// (không phải `_fbc`), ta dựng lại theo chuẩn.
//
// ⚠️ Fix 2026-11-23: trước đây timestamp dựng lại luôn là `"0"`
// (`fb.1.0.<fbclid>`) — Meta dùng `creation_time` để tính độ tươi click
// và chấm EMQ. Nay dùng `time.Now().UnixMilli()` ⇒ khớp thời điểm click
// thực tế hơn, tăng điểm `fbc` trong Dataset Quality.
func NormalizeFBC(fbc, fbclid string) string {
	fbc = strings.TrimSpace(fbc)
	if fbc != "" {
		// Đã đúng định dạng: fb.1.<ts>.<fbclid>
		if strings.HasPrefix(fbc, "fb.") && len(strings.Split(fbc, ".")) >= 4 {
			return fbc
		}
		// Ngắn quá → coi như fbclid và dựng lại.
		fbclid = fbc
	}
	fbclid = strings.TrimSpace(fbclid)
	if fbclid == "" {
		return ""
	}
	if strings.HasPrefix(fbclid, "fb.") && len(strings.Split(fbclid, ".")) >= 4 {
		return fbclid
	}
	return "fb.1." + strconv.FormatInt(time.Now().UnixMilli(), 10) + "." + fbclid
}

// NormalizeFBP chuẩn hoá `_fbp` theo định dạng Meta: fb.1.<timestamp>.<random>.
func NormalizeFBP(fbp string) string {
	fbp = strings.TrimSpace(fbp)
	if fbp == "" {
		return ""
	}
	if strings.HasPrefix(fbp, "fb.") && len(strings.Split(fbp, ".")) >= 4 {
		return fbp
	}
	return "fb.1.0." + fbp
}
