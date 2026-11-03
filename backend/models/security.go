package models

import "encoding/json"

// SpamQuarantineItem đại diện cho một request rác bị hệ thống bảo mật tóm gọn và cách ly
type SpamQuarantineItem struct {
	ID        uint64          `json:"id"`
	CreatedAt string          `json:"created_at"`
	IPAddress string          `json:"ip_address"`
	UserAgent string          `json:"user_agent"`
	LandingID string          `json:"landing_id"`
	Nguon     string          `json:"nguon"`
	Phone     string          `json:"phone"`
	Reason    string          `json:"reason"`
	Payload   json.RawMessage `json:"payload"`
}

// SecurityStats thống kê số lượng spam đã chặn và phân loại vi phạm
type SecurityStats struct {
	TotalSpamBlocked int64            `json:"total_spam_blocked"`
	TodaySpamBlocked int64            `json:"today_spam_blocked"`
	ReasonsBreakdown map[string]int64 `json:"reasons_breakdown"`
}
