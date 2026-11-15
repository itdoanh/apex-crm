package facebook

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"apex-backend/models"
)

// ─── Sự kiện Meta ───────────────────────────────────────────────────────

const (
	EventPageView   = "PageView"
	EventLead       = "Lead"
	EventPurchase   = "Purchase"
	EventSubscribe  = "Subscribe"
	EventStartTrial = "StartTrial"
)

// GenerateEventID tạo event_id duy nhất (UUID v4 rút gọn).
//
// ⚠️ event_id là khoá để Meta gộp sự kiện browser + server (deduplication).
// Nếu browser (fbq) và server (CAPI) dùng chung event_id thì Meta chỉ tính 1 lần.
// Vì vậy: nếu client đã gửi `event_id` lên server, server PHẢI dùng lại đúng
// event_id đó. Nếu không có, server tự sinh.
func GenerateEventID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback: dùng timestamp + hash. Vẫn đủ độc lập cho dedup.
		return fmt.Sprintf("apex-%d", time.Now().UnixNano())
	}
	// Dạng UUID v4 (8-4-4-4-12)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32])
}

// BuildUserData dựng khối `user_data` từ thông tin lead + tracking.
//
// Meta yêu cầu PII băm SHA-256 (ngoại trừ `client_ip_address` và
// `client_user_agent` — hai trường này PHẢI giữ nguyên bản rõ).
func BuildUserData(cfg *models.FacebookCAPIConfig, input UserDataInput) *UserData {
	if !cfg.SendUserData {
		return nil
	}
	ud := &UserData{}

	if cfg.HashEmail {
		if h := HashEmail(input.Email); h != "" {
			ud.Em = []string{h}
		}
	}
	if cfg.HashPhone {
		if h := HashPhone(input.Phone); h != "" {
			ud.Ph = []string{h}
		}
	}
	if cfg.HashName {
		if h := HashText(input.FirstName); h != "" {
			ud.Fn = []string{h}
		}
		if h := HashText(input.LastName); h != "" {
			ud.Ln = []string{h}
		}
	}
	if cfg.HashAddress {
		if h := HashText(input.City); h != "" {
			ud.CT = []string{h}
		}
	}
	if cfg.HashState {
		if h := HashText(input.State); h != "" {
			ud.ST = []string{h}
		}
	}
	if cfg.HashZip {
		if h := HashText(input.Zip); h != "" {
			ud.Zp = []string{h}
		}
	}
	if cfg.HashCountry {
		if h := HashCountry(input.Country); h != "" {
			ud.Country = []string{h}
		}
	}
	if cfg.HashExternalID {
		if h := HashExternalID(input.ExternalID); h != "" {
			ud.ExternalID = []string{h}
		}
	}

	// Không có PII nào → trả nil để không gửi khối rỗng.
	if len(ud.Em) == 0 && len(ud.Ph) == 0 && len(ud.Fn) == 0 && len(ud.Ln) == 0 &&
		len(ud.CT) == 0 && len(ud.ST) == 0 && len(ud.Zp) == 0 && len(ud.Country) == 0 &&
		len(ud.ExternalID) == 0 {
		return nil
	}
	return ud
}

// UserDataInput là dữ liệu thô đầu vào để dựng user_data.
type UserDataInput struct {
	Email      string
	Phone      string
	FirstName  string
	LastName   string
	City       string
	State      string
	Zip        string
	Country    string
	ExternalID string
}

// EventOptions là tuỳ chọn khi dựng 1 sự kiện.
type EventOptions struct {
	EventName      string
	EventID        string
	EventTime      int64
	EventSourceURL string
	UserData       *UserData
	CustomData     *CustomData
}

// BuildEvent dựng 1 sự kiện CAPI hoàn chỉnh.
//
// `event_time` là UNIX timestamp — Meta chỉ nhận sự kiện trong 7 ngày gần nhất
// (server event window), và khuyến nghị gửi trong 24h.
func BuildEvent(opts EventOptions) CAPIEvent {
	eventID := strings.TrimSpace(opts.EventID)
	if eventID == "" {
		eventID = GenerateEventID()
	}
	eventTime := opts.EventTime
	if eventTime <= 0 {
		eventTime = time.Now().Unix()
	}
	eventName := opts.EventName
	if eventName == "" {
		eventName = EventLead
	}
	return CAPIEvent{
		EventName:      eventName,
		EventTime:      eventTime,
		EventID:        eventID,
		ActionSource:   "website",
		EventSourceURL: opts.EventSourceURL,
		UserData:       opts.UserData,
		CustomData:     opts.CustomData,
	}
}

// BuildValueCustomData dựng custom_data có giá trị (Purchase/Subscribe).
//
// Meta quy ước `value` là số thực đại diện giá trị. Với VND, 1 đơn vị = 1 VND
// là quy ước phổ biến nhất khi dùng chung dataset với các đơn vị khác.
func BuildValueCustomData(value float64, currency, contentName, contentType string) *CustomData {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		currency = "VND"
	}
	v := value
	return &CustomData{
		Currency:    currency,
		Value:       &v,
		ContentName: contentName,
		ContentType: contentType,
	}
}

// SplitFullName tách họ tên tiếng Việt thành tên và tên đệm.
//
// Tiếng Việt viết liền không có dấu cách rõ ràng giữa họ và tên, nên ta dùng
// quy ước: từ cuối = tên, từ đầu = họ + tên đệm. Đây cũng là cách Meta Pixel
// thường xử lý với dữ liệu tiếng Việt.
func SplitFullName(full string) (first string, last string) {
	full = strings.TrimSpace(full)
	if full == "" {
		return "", ""
	}
	parts := strings.Fields(full)
	if len(parts) == 1 {
		return parts[0], ""
	}
	last = parts[len(parts)-1]
	rest := strings.Join(parts[:len(parts)-1], " ")
	return rest, last
}
