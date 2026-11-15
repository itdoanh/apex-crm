package facebook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ─── Dataset Quality API (Meta v27.0) ───────────────────────────────────
//
// Endpoint: GET https://graph.facebook.com/{version}/dataset_quality
//
// ⛔ THAM SỐ BẮT BUỘC:
//   - dataset_id   : ID tập dữ liệu (Pixel) cần truy vấn
//   - access_token : token có quyền `ads_read` (hoặc `ads_management`)
//   - fields       : danh sách trường cần lấy
//
// 📌 CẤU TRÚC PHẢN HỒI (rất khác với "1 số liệu tổng"):
//
//	{
//	  "web": [                                  ← MẢNG, mỗi phần tử = 1 event_name
//	    {
//	      "event_name": "Purchase",
//	      "event_match_quality": {
//	        "composite_score": 6.2,             ← điểm EMQ 0..10
//	        "match_key_feedback": [
//	          { "identifier": "email",
//	            "coverage": { "percentage": 100 },
//	            "potential_aly_acr_increase": { "percentage": 58.96, "description": "…" } }
//	        ],
//	        "diagnostics": [                    ← 6 lỗi EMQ mới từ 09/2026
//	          { "name": "…", "description": "…", "solution": "…",
//	            "percentage": 59.5, "affected_event_count": 18930, "total_event_count": 31830 }
//	        ]
//	      },
//	      "acr": { "percentage": 37.9, "description": "…" },
//	      "event_potential_aly_acr_increase": { "percentage": 32.9, "description": "…" },
//	      "event_coverage": { "percentage": 34.1, "goal_percentage": 75, "description": "…" },
//	      "dedupe_key_feedback": [
//	        { "dedupe_key": "event_id",
//	          "browser_events_with_dedupe_key": { "percentage": 100, "description": "…" },
//	          "server_events_with_dedupe_key":  { "percentage": 100, "description": "…" },
//	          "overall_browser_coverage_from_dedupe_key": { "percentage": 14.8, "description": "…" } }
//	      ],
//	      "data_freshness": { "upload_frequency": "real_time", "description": "…" },
//	      "event_spend": { "amount": 108947.38, "currency_code": "MXN" },
//	      "event_ad_sets": { "data": [ { "ad_set_id": "…", "ad_set_name": "…",
//	          "objective": "Sales", "spend_amount": 20619.35,
//	          "spend_currency_code": "USD", "event_use": "Optimization" } ] },
//	      "value_diagnostics": [ { "issue_type": "MISSING_OR_INVALID_VALUE",
//	          "issue_count": 250, "event_count": 1000, "issue_rate": 0.25, "issue_example": "null" } ],
//	      "event_delay": { "events_delayed_over_2h": 100, "events_delayed_over_24h": 10,
//	          "total_events": 1000, "pct_delayed_over_2h": 10.0, "pct_delayed_over_24h": 1.0 }
//	    }
//	  ]
//	}
//
// ⚠️ Đây là lý do bản cũ (parse `web` như 1 object) luôn hiển thị "—":
//    Meta trả `web` là MẢNG ⇒ mọi trường đọc dạng map đều rỗng.

// DatasetQualityFields là tập trường Meta yêu cầu khi đọc Dataset Quality API.
//
// Bao gồm đủ: EMQ (composite_score + match_key_feedback + diagnostics), ACR,
// event_coverage, dedupe_key_feedback, data_freshness, event_spend, event_ad_sets,
// value_diagnostics, event_delay.
const DatasetQualityFields = "web{" +
	"event_name," +
	"event_match_quality{composite_score,match_key_feedback{identifier,coverage{percentage},potential_aly_acr_increase{percentage,description}},diagnostics{name,description,solution,percentage,affected_event_count,total_event_count}}," +
	"acr{percentage,description}," +
	"event_potential_aly_acr_increase{percentage,description}," +
	"event_coverage{percentage,goal_percentage,description}," +
	"dedupe_key_feedback{dedupe_key,browser_events_with_dedupe_key{percentage,description},server_events_with_dedupe_key{percentage,description},overall_browser_coverage_from_dedupe_key{percentage,description}}," +
	"data_freshness{upload_frequency,description}," +
	"event_spend{amount,currency_code}," +
	"event_ad_sets{ad_set_id,ad_set_name,objective,spend_amount,spend_currency_code,event_use}," +
	"value_diagnostics{issue_type,issue_count,event_count,issue_rate,issue_example}," +
	"event_delay{events_delayed_over_2h,events_delayed_over_24h,total_events,pct_delayed_over_2h,pct_delayed_over_24h}" +
	"}"

// DatasetQualityResponse là phản hồi thô từ endpoint dataset_quality.
type DatasetQualityResponse struct {
	Data  []map[string]interface{} `json:"data"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    int    `json:"code"`
	} `json:"error,omitempty"`
}

// ─── Struct chuẩn hoá (frontend dùng) ────────────────────────────────────

// DQPercentDesc là cặp (percentage, description) dùng chung của Meta.
type DQPercentDesc struct {
	Percentage  float64 `json:"percentage,omitempty"`
	Description string  `json:"description,omitempty"`
}

// DQCoverage là tỷ lệ sự kiện phù hợp (event_coverage).
type DQCoverage struct {
	Percentage     float64 `json:"percentage,omitempty"`
	GoalPercentage float64 `json:"goal_percentage,omitempty"`
	Description    string  `json:"description,omitempty"`
}

// DQMatchKey là 1 khoá so khớp (email, ip_address, fbc, external_id…).
type DQMatchKey struct {
	Identifier string       `json:"identifier"`
	Coverage   DQPercentDesc `json:"coverage"`
	PotentialACR DQPercentDesc `json:"potential_aly_acr_increase,omitempty"`
}

// DQDiagnostic là 1 lỗi EMQ do Meta chẩn đoán (6 nhóm từ 09/2026).
type DQDiagnostic struct {
	Name               string  `json:"name"`
	Description        string  `json:"description,omitempty"`
	Solution           string  `json:"solution,omitempty"`
	Percentage         float64 `json:"percentage,omitempty"`
	AffectedEventCount int     `json:"affected_event_count,omitempty"`
	TotalEventCount    int     `json:"total_event_count,omitempty"`
}

// DQEventMatchQuality là chất lượng so khớp sự kiện (EMQ) của 1 event.
type DQEventMatchQuality struct {
	CompositeScore   float64       `json:"composite_score,omitempty"`
	MatchKeyFeedback []DQMatchKey  `json:"match_key_feedback,omitempty"`
	Diagnostics      []DQDiagnostic `json:"diagnostics,omitempty"`
}

// DQDedupeKey là phản hồi khoá bỏ trùng lặp cho 1 dedupe_key.
type DQDedupeKey struct {
	DedupeKey                    string        `json:"dedupe_key"`
	BrowserEventsWithDedupeKey   DQPercentDesc `json:"browser_events_with_dedupe_key"`
	ServerEventsWithDedupeKey    DQPercentDesc `json:"server_events_with_dedupe_key"`
	OverallBrowserCoverage       DQPercentDesc `json:"overall_browser_coverage_from_dedupe_key"`
}

// DQFreshness là độ mới dữ liệu.
type DQFreshness struct {
	UploadFrequency string `json:"upload_frequency,omitempty"`
	Description     string `json:"description,omitempty"`
}

// DQSpend là chi tiêu cho sự kiện.
type DQSpend struct {
	Amount       float64 `json:"amount,omitempty"`
	CurrencyCode string  `json:"currency_code,omitempty"`
}

// DQAdSet là 1 nhóm quảng cáo gắn với sự kiện.
type DQAdSet struct {
	AdSetID           string  `json:"ad_set_id,omitempty"`
	AdSetName         string  `json:"ad_set_name,omitempty"`
	Objective         string  `json:"objective,omitempty"`
	SpendAmount       float64 `json:"spend_amount,omitempty"`
	SpendCurrencyCode string  `json:"spend_currency_code,omitempty"`
	EventUse          string  `json:"event_use,omitempty"`
}

// DQValueDiagnostic là 1 lỗi chất lượng value/currency.
type DQValueDiagnostic struct {
	IssueType    string  `json:"issue_type,omitempty"`
	IssueCount   int     `json:"issue_count,omitempty"`
	EventCount   int     `json:"event_count,omitempty"`
	IssueRate    float64 `json:"issue_rate,omitempty"`
	IssueExample string  `json:"issue_example,omitempty"`
}

// DQEventDelay là độ trễ tải lên sự kiện (7 ngày gần nhất).
type DQEventDelay struct {
	EventsDelayedOver2H  int     `json:"events_delayed_over_2h,omitempty"`
	EventsDelayedOver24H int     `json:"events_delayed_over_24h,omitempty"`
	TotalEvents          int     `json:"total_events,omitempty"`
	PctDelayedOver2H     float64 `json:"pct_delayed_over_2h,omitempty"`
	PctDelayedOver24H    float64 `json:"pct_delayed_over_24h,omitempty"`
}

// DQEvent là 1 sự kiện (event_name) kèm toàn bộ chỉ số chất lượng.
type DQEvent struct {
	EventName      string                 `json:"event_name"`
	EMQ            *DQEventMatchQuality   `json:"event_match_quality,omitempty"`
	ACR            *DQPercentDesc         `json:"acr,omitempty"`
	PotentialACR   *DQPercentDesc         `json:"event_potential_aly_acr_increase,omitempty"`
	Coverage       *DQCoverage            `json:"event_coverage,omitempty"`
	DedupeKeys     []DQDedupeKey          `json:"dedupe_key_feedback,omitempty"`
	Freshness      *DQFreshness           `json:"data_freshness,omitempty"`
	Spend          *DQSpend               `json:"event_spend,omitempty"`
	AdSets         []DQAdSet              `json:"event_ad_sets,omitempty"`
	ValueDiags     []DQValueDiagnostic    `json:"value_diagnostics,omitempty"`
	Delay          *DQEventDelay          `json:"event_delay,omitempty"`
}

// DQSummary là kết quả đã chuẩn hoá, frontend dùng để vẽ thẻ chỉ số.
type DQSummary struct {
	PixelID   string    `json:"pixel_id"`
	FetchedAt string    `json:"fetched_at"`
	Events    []DQEvent `json:"events"`
	// Aggregate là số liệu gộp cho toàn bộ dataset (dùng cho KPI nhanh).
	Aggregate DQAggregate `json:"aggregate"`
}

// DQAggregate gộp nhanh các chỉ số quan trọng nhất.
type DQAggregate struct {
	EventCount        int     `json:"event_count"`
	AvgEMQ            float64 `json:"avg_emq"`
	AvgCoverage       float64 `json:"avg_coverage"`
	TotalSpend        float64 `json:"total_spend"`
	SpendCurrencyCode string  `json:"spend_currency_code,omitempty"`
	CriticalIssues    int     `json:"critical_issues"`
}

// FetchDatasetQuality đọc chỉ số chất lượng dữ liệu của 1 pixel từ Meta.
//
// Endpoint: GET https://graph.facebook.com/{version}/dataset_quality
//           ?dataset_id=<PIXEL_ID>&agent_name=<NAME>&access_token=<TOKEN>&fields=...
//
// ⏱️ Meta cần ≥ 20 phút sau khi gửi sự kiện mới có số liệu, và dữ liệu thường
// trễ vài giờ ⇒ chỉ gọi thủ công từ Admin UI (nút "Làm mới"), KHÔNG gọi tự
// động khi submit lead.
func (c *Client) FetchDatasetQuality(ctx context.Context, apiVersion, pixelID, accessToken, agentName string) (map[string]interface{}, int, []byte, error) {
	version := strings.TrimSpace(apiVersion)
	if version == "" {
		version = DefaultAPIVersion
	}
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	if strings.TrimSpace(accessToken) == "" {
		return nil, 0, nil, fmt.Errorf("thiếu access token Meta")
	}
	// ⛔ `dataset_id` là tham số BẮT BUỘC — thiếu nó Meta trả lỗi 100.
	if strings.TrimSpace(pixelID) == "" {
		return nil, 0, nil, fmt.Errorf("thiếu dataset_id (Pixel ID)")
	}

	q := url.Values{}
	q.Set("dataset_id", pixelID)
	q.Set("fields", DatasetQualityFields)
	if a := strings.TrimSpace(agentName); a != "" {
		q.Set("agent_name", a)
	}
	q.Set("access_token", accessToken)
	endpoint := fmt.Sprintf("%s/%s/dataset_quality?%s", GraphBaseURL, version, q.Encode())

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("không tạo được HTTP request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("lỗi gọi Dataset Quality API: %w", err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, resp.StatusCode, nil, fmt.Errorf("không đọc được phản hồi: %w", readErr)
	}

	var parsed DatasetQualityResponse
	if len(body) > 0 {
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, resp.StatusCode, body, fmt.Errorf("phản hồi Dataset Quality không phải JSON (HTTP %d): %s",
				resp.StatusCode, truncate(string(body), 400))
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := ""
		if parsed.Error != nil {
			msg = parsed.Error.Message
		}
		if msg == "" {
			msg = truncate(string(body), 300)
		}
		return nil, resp.StatusCode, body, fmt.Errorf("Meta trả HTTP %d: %s", resp.StatusCode, msg)
	}

	if len(parsed.Data) == 0 {
		return map[string]interface{}{}, resp.StatusCode, body, nil
	}
	return parsed.Data[0], resp.StatusCode, body, nil
}

// ParseDatasetQuality chuẩn hoá phản hồi `dataset_quality` thành cấu trúc
// dễ hiển thị.
//
// ⛔ Điểm mấu chốt: `web` là MẢNG (mỗi phần tử = 1 event_name). Bản cũ parse
// nó như 1 object ⇒ mọi chỉ số đều rỗng. Hàm này duyệt MẢNG và gom theo event.
func ParseDatasetQuality(raw map[string]interface{}, pixelID string) DQSummary {
	sum := DQSummary{
		PixelID:   pixelID,
		FetchedAt: time.Now().Format("2006-01-02 15:04:05"),
		Events:    []DQEvent{},
	}
	if raw == nil {
		return sum
	}

	// `web` có thể là mảng (đúng chuẩn v27.0) hoặc 1 object (fallback).
	var webItems []map[string]interface{}
	switch v := raw["web"].(type) {
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				webItems = append(webItems, m)
			}
		}
	case map[string]interface{}:
		webItems = append(webItems, v)
	}

	for _, w := range webItems {
		ev := DQEvent{EventName: str(w["event_name"])}
		ev.EMQ = parseEMQ(w["event_match_quality"])
		ev.ACR = parsePercentDesc(w["acr"])
		ev.PotentialACR = parsePercentDesc(w["event_potential_aly_acr_increase"])
		ev.Coverage = parseCoverage(w["event_coverage"])
		ev.DedupeKeys = parseDedupeKeys(w["dedupe_key_feedback"])
		ev.Freshness = parseFreshness(w["data_freshness"])
		ev.Spend = parseSpend(w["event_spend"])
		ev.AdSets = parseAdSets(w["event_ad_sets"])
		ev.ValueDiags = parseValueDiags(w["value_diagnostics"])
		ev.Delay = parseDelay(w["event_delay"])
		sum.Events = append(sum.Events, ev)
	}

	sum.Aggregate = aggregateEvents(sum.Events)
	return sum
}

// aggregateEvents tính chỉ số gộp cho toàn dataset.
func aggregateEvents(events []DQEvent) DQAggregate {
	agg := DQAggregate{EventCount: len(events)}
	if len(events) == 0 {
		return agg
	}
	var emqSum, covSum float64
	var emqCnt, covCnt int
	for _, ev := range events {
		if ev.EMQ != nil && ev.EMQ.CompositeScore > 0 {
			emqSum += ev.EMQ.CompositeScore
			emqCnt++
		}
		if ev.Coverage != nil && ev.Coverage.Percentage > 0 {
			covSum += ev.Coverage.Percentage
			covCnt++
		}
		if ev.Spend != nil && ev.Spend.Amount > 0 {
			agg.TotalSpend += ev.Spend.Amount
			if agg.SpendCurrencyCode == "" {
				agg.SpendCurrencyCode = ev.Spend.CurrencyCode
			}
		}
		agg.CriticalIssues += len(ev.ValueDiags)
		if ev.EMQ != nil {
			agg.CriticalIssues += len(ev.EMQ.Diagnostics)
		}
	}
	if emqCnt > 0 {
		agg.AvgEMQ = emqSum / float64(emqCnt)
	}
	if covCnt > 0 {
		agg.AvgCoverage = covSum / float64(covCnt)
	}
	return agg
}

// ─── Helper parse từng khối ─────────────────────────────────────────────

func str(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func num(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	}
	return 0
}

func i64(v interface{}) int {
	return int(num(v))
}

func parsePercentDesc(v interface{}) *DQPercentDesc {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	return &DQPercentDesc{
		Percentage:  num(m["percentage"]),
		Description: str(m["description"]),
	}
}

func parseCoverage(v interface{}) *DQCoverage {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	return &DQCoverage{
		Percentage:     num(m["percentage"]),
		GoalPercentage: num(m["goal_percentage"]),
		Description:    str(m["description"]),
	}
}

func parseEMQ(v interface{}) *DQEventMatchQuality {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	emq := &DQEventMatchQuality{CompositeScore: num(m["composite_score"])}

	if arr, ok := m["match_key_feedback"].([]interface{}); ok {
		for _, item := range arr {
			km, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			key := DQMatchKey{Identifier: str(km["identifier"])}
			if c := parsePercentDesc(km["coverage"]); c != nil {
				key.Coverage = *c
			}
			if p := parsePercentDesc(km["potential_aly_acr_increase"]); p != nil {
				key.PotentialACR = *p
			}
			emq.MatchKeyFeedback = append(emq.MatchKeyFeedback, key)
		}
	}

	if arr, ok := m["diagnostics"].([]interface{}); ok {
		for _, item := range arr {
			dm, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			emq.Diagnostics = append(emq.Diagnostics, DQDiagnostic{
				Name:               str(dm["name"]),
				Description:        str(dm["description"]),
				Solution:           str(dm["solution"]),
				Percentage:         num(dm["percentage"]),
				AffectedEventCount: i64(dm["affected_event_count"]),
				TotalEventCount:    i64(dm["total_event_count"]),
			})
		}
	}
	return emq
}

func parseDedupeKeys(v interface{}) []DQDedupeKey {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]DQDedupeKey, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		d := DQDedupeKey{DedupeKey: str(m["dedupe_key"])}
		if c := parsePercentDesc(m["browser_events_with_dedupe_key"]); c != nil {
			d.BrowserEventsWithDedupeKey = *c
		}
		if c := parsePercentDesc(m["server_events_with_dedupe_key"]); c != nil {
			d.ServerEventsWithDedupeKey = *c
		}
		if c := parsePercentDesc(m["overall_browser_coverage_from_dedupe_key"]); c != nil {
			d.OverallBrowserCoverage = *c
		}
		out = append(out, d)
	}
	return out
}

func parseFreshness(v interface{}) *DQFreshness {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	return &DQFreshness{
		UploadFrequency: str(m["upload_frequency"]),
		Description:     str(m["description"]),
	}
}

func parseSpend(v interface{}) *DQSpend {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	return &DQSpend{
		Amount:       num(m["amount"]),
		CurrencyCode: str(m["currency_code"]),
	}
}

func parseAdSets(v interface{}) []DQAdSet {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	arr, ok := m["data"].([]interface{})
	if !ok {
		return nil
	}
	out := make([]DQAdSet, 0, len(arr))
	for _, item := range arr {
		am, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		out = append(out, DQAdSet{
			AdSetID:           str(am["ad_set_id"]),
			AdSetName:         str(am["ad_set_name"]),
			Objective:         str(am["objective"]),
			SpendAmount:       num(am["spend_amount"]),
			SpendCurrencyCode: str(am["spend_currency_code"]),
			EventUse:          str(am["event_use"]),
		})
	}
	return out
}

func parseValueDiags(v interface{}) []DQValueDiagnostic {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]DQValueDiagnostic, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		out = append(out, DQValueDiagnostic{
			IssueType:    str(m["issue_type"]),
			IssueCount:   i64(m["issue_count"]),
			EventCount:   i64(m["event_count"]),
			IssueRate:    num(m["issue_rate"]),
			IssueExample: str(m["issue_example"]),
		})
	}
	return out
}

func parseDelay(v interface{}) *DQEventDelay {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	return &DQEventDelay{
		EventsDelayedOver2H:  i64(m["events_delayed_over_2h"]),
		EventsDelayedOver24H: i64(m["events_delayed_over_24h"]),
		TotalEvents:          i64(m["total_events"]),
		PctDelayedOver2H:     num(m["pct_delayed_over_2h"]),
		PctDelayedOver24H:    num(m["pct_delayed_over_24h"]),
	}
}
