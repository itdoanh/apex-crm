package facebook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultAPIVersion là phiên bản Graph API mặc định cho Conversions API.
const DefaultAPIVersion = "v27.0"

// GraphBaseURL là endpoint gốc của Meta Graph API.
const GraphBaseURL = "https://graph.facebook.com"

// ─── Payload theo chuẩn Meta Conversions API ────────────────────────────

// UserData chứa PII đã băm SHA-256 (hoặc để trống nếu không băm).
//
// ⚠️ Meta yêu cầu: `client_ip_address` và `client_user_agent` PHẢI là giá trị
// RÕ (không băm). `fbc`/`fbp` cũng gửi rõ — chúng là định danh lượt nhấp/phiên
// do Meta sinh ra, dùng để khớp event browser với event server.
type UserData struct {
	Em         []string `json:"em,omitempty"`
	Ph         []string `json:"ph,omitempty"`
	Fn         []string `json:"fn,omitempty"`
	Ln         []string `json:"ln,omitempty"`
	CT         []string `json:"ct,omitempty"`
	ST         []string `json:"st,omitempty"`
	Zp         []string `json:"zp,omitempty"`
	Country    []string `json:"country,omitempty"`
	ExternalID []string `json:"external_id,omitempty"`

	// Các trường KHÔNG băm.
	ClientIPAddress string `json:"client_ip_address,omitempty"`
	ClientUserAgent string `json:"client_user_agent,omitempty"`
	FBC             string `json:"fbc,omitempty"`
	FBP             string `json:"fbp,omitempty"`
}

// CustomData là dữ liệu tuỳ chỉnh của sự kiện (value, currency, content_name…).
type CustomData struct {
	Currency    string                 `json:"currency,omitempty"`
	Value       *float64               `json:"value,omitempty"`
	ContentName string                 `json:"content_name,omitempty"`
	ContentType string                 `json:"content_type,omitempty"`
	ContentIDs  []string               `json:"content_ids,omitempty"`
	Contents    []interface{}          `json:"contents,omitempty"`
	OrderID     string                 `json:"order_id,omitempty"`
	NumItems    *int                   `json:"num_items,omitempty"`
	Extra       map[string]interface{} `json:"-"`
}

// CAPIEvent là 1 sự kiện gửi lên Meta.
type CAPIEvent struct {
	EventName      string      `json:"event_name"`
	EventTime      int64       `json:"event_time"`
	EventID        string      `json:"event_id"`
	ActionSource   string      `json:"action_source"`
	EventSourceURL string      `json:"event_source_url,omitempty"`
	UserData       *UserData   `json:"user_data,omitempty"`
	CustomData     *CustomData `json:"custom_data,omitempty"`
}

// CAPIRequest là body gửi lên Graph API.
type CAPIRequest struct {
	Data []CAPIEvent `json:"data"`
	// TestEventCode: mã test từ Events Manager. Khi có giá trị, sự kiện
	// được gửi vào Test Events thay vì vào dữ liệu thật.
	TestEventCode string `json:"test_event_code,omitempty"`
	// PartnerAgent là thông tin đối tác xử lý (khuyến nghị khi dùng Gateway).
	PartnerAgent string `json:"partner_agent,omitempty"`
}

// CAPIResponse là phản hồi từ Meta.
type CAPIResponse struct {
	EventsReceived int    `json:"events_received"`
	EventsFailed   int    `json:"events_failed"`
	Message        string `json:"message,omitempty"`
	Error          *struct {
		Message   string `json:"message"`
		Type      string `json:"type"`
		Code      int    `json:"code"`
		FBTraceID string `json:"fbtrace_id"`
		Subcode   int    `json:"error_subcode"`
	} `json:"error,omitempty"`
	// Một số lỗi trả về dạng mảng.
	Errors []struct {
		Message   string `json:"message"`
		Type      string `json:"type"`
		Code      int    `json:"code"`
		FBTraceID string `json:"fbtrace_id"`
	} `json:"errors,omitempty"`
}

// Client là HTTP client gọi Meta Graph API.
type Client struct {
	HTTP *http.Client
}

// NewClient tạo client với timeout mặc định 10 giây.
func NewClient() *Client {
	return &Client{
		HTTP: &http.Client{Timeout: 15 * time.Second},
	}
}

// BuildEventURL trả về URL endpoint gửi sự kiện.
//
//   - Nếu bật Gateway: dùng endpoint của Conversions API Gateway
//     (thường dạng https://<region>.gateway.facebook.com/events) kèm access key.
//   - Ngược lại: gọi trực tiếp https://graph.facebook.com/{version}/{pixel_id}/events
func BuildEventURL(pixelID, apiVersion string, gatewayEnabled bool, gatewayEndpoint, gatewayAccessKey string) string {
	version := strings.TrimSpace(apiVersion)
	if version == "" {
		version = DefaultAPIVersion
	}
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}

	if gatewayEnabled {
		endpoint := strings.TrimSpace(gatewayEndpoint)
		if endpoint == "" {
			endpoint = GraphBaseURL
		}
		endpoint = strings.TrimSuffix(endpoint, "/")
		// Gateway dùng path dạng /events (không có pixel_id trong path).
		if !strings.HasSuffix(endpoint, "/events") {
			endpoint += "/" + version + "/" + pixelID + "/events"
		}
		q := url.Values{}
		q.Set("access_key", gatewayAccessKey)
		return endpoint + "?" + q.Encode()
	}

	return fmt.Sprintf("%s/%s/%s/events", GraphBaseURL, version, pixelID)
}

// SendEvents gửi 1 payload sự kiện lên Meta.
//
// Gateway yêu cầu access key trong query string, còn Graph API trực tiếp
// dùng access token trong query string. Hàm này hỗ trợ cả hai.
func (c *Client) SendEvents(ctx context.Context, endpoint string, accessToken string, req *CAPIRequest) (*CAPIResponse, int, []byte, error) {
	if req == nil {
		return nil, 0, nil, fmt.Errorf("payload rỗng")
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("không serialize được payload: %w", err)
	}

	// Access token cho Graph API trực tiếp.
	sep := "?"
	if strings.Contains(endpoint, "?") {
		sep = "&"
	}
	fullURL := endpoint
	if accessToken != "" && !strings.Contains(endpoint, "access_key=") {
		fullURL = endpoint + sep + "access_token=" + url.QueryEscape(accessToken)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(body))
	if err != nil {
		return nil, 0, nil, fmt.Errorf("không tạo được HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("lỗi gọi Meta Graph API: %w", err)
	}
	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, resp.StatusCode, nil, fmt.Errorf("không đọc được phản hồi: %w", readErr)
	}

	var parsed CAPIResponse
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &parsed); err != nil {
			// Meta đôi khi trả về HTML khi lỗi gateway → trả raw body lại để log.
			return nil, resp.StatusCode, respBody, fmt.Errorf("phản hồi Meta không phải JSON (HTTP %d): %s", resp.StatusCode, truncate(string(respBody), 500))
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := parsed.Message
		if parsed.Error != nil && parsed.Error.Message != "" {
			msg = parsed.Error.Message
		}
		if len(parsed.Errors) > 0 && parsed.Errors[0].Message != "" {
			msg = parsed.Errors[0].Message
		}
		if msg == "" {
			msg = truncate(string(respBody), 300)
		}
		return &parsed, resp.StatusCode, respBody, fmt.Errorf("Meta trả HTTP %d: %s", resp.StatusCode, msg)
	}

	return &parsed, resp.StatusCode, respBody, nil
}

// IsGatewayError kiểm tra lỗi có phải do cấu hình Gateway sai (endpoint/access key).
func IsGatewayError(status int, body []byte) bool {
	if status == 401 || status == 403 {
		return true
	}
	s := strings.ToLower(string(body))
	return strings.Contains(s, "access key") || strings.Contains(s, "gateway")
}

// FormatGraphError rút gọn thông báo lỗi của Meta để hiển thị trong CRM.
func FormatGraphError(resp *CAPIResponse, status int, fallback string) string {
	if resp != nil {
		if resp.Error != nil && resp.Error.Message != "" {
			return resp.Error.Message
		}
		if len(resp.Errors) > 0 && resp.Errors[0].Message != "" {
			return resp.Errors[0].Message
		}
		if resp.Message != "" {
			return resp.Message
		}
	}
	if fallback != "" {
		return fallback
	}
	return fmt.Sprintf("HTTP %d", status)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
