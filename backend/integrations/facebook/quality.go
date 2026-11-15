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

// DatasetQualityFields là các trường Meta yêu cầu khi đọc Dataset Quality API.
const DatasetQualityFields = "web{event_match_quality,event_coverage,dedupe_key_feedback,data_freshness,value_diagnostics,event_delay,event_spend,event_ad_sets}"

// DatasetQualityResponse là phản hồi thô từ endpoint dataset_quality.
type DatasetQualityResponse struct {
	Data  []map[string]interface{} `json:"data"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    int    `json:"code"`
	} `json:"error,omitempty"`
}

// FetchDatasetQuality đọc chỉ số chất lượng dữ liệu (EMQ, coverage, dedupe,
// freshness, delay, value) của 1 pixel từ Meta.
//
// Endpoint: GET https://graph.facebook.com/{version}/dataset_quality?access_token=..&fields=web{...}
//
// ⏱️ Meta yêu cầu chờ ≥ 20 phút sau khi gửi sự kiện mới có số liệu cập nhật,
// và dữ liệu thường bị trễ vài giờ. Đây là lý do hàm này chỉ được gọi
// từ Admin UI (nút "Làm mới"), không gọi tự động khi submit lead.
func (c *Client) FetchDatasetQuality(ctx context.Context, apiVersion, accessToken string) (map[string]interface{}, int, []byte, error) {
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

	q := url.Values{}
	q.Set("access_token", accessToken)
	q.Set("fields", DatasetQualityFields)
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

// ParseDatasetQuality tách phần `web` ra thành cấu trúc dễ hiển thị.
func ParseDatasetQuality(raw map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{
		"event_match_quality": map[string]interface{}{},
		"event_coverage":      map[string]interface{}{},
		"dedupe_key_feedback": map[string]interface{}{},
		"data_freshness":      map[string]interface{}{},
		"value_diagnostics":   map[string]interface{}{},
		"event_delay":         map[string]interface{}{},
		"event_spend":         map[string]interface{}{},
		"event_ad_sets":       map[string]interface{}{},
	}
	if raw == nil {
		return out
	}
	web, ok := raw["web"].(map[string]interface{})
	if !ok {
		return out
	}
	for _, k := range []string{
		"event_match_quality", "event_coverage", "dedupe_key_feedback",
		"data_freshness", "value_diagnostics", "event_delay",
		"event_spend", "event_ad_sets",
	} {
		if v, ok := web[k]; ok && v != nil {
			out[k] = v
		}
	}
	return out
}
