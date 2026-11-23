# Meta Conversions API + Pixel — Tài liệu kỹ thuật chi tiết

> **Dataset chính:** `1468058901923737` · **Graph API:** `v27.0` · **Test event code:** `TEST90946`
> **Phạm vi:** toàn bộ luồng Meta Pixel (browser) + Conversions API (server) trong Apex CRM.
> Cập nhật: 2026-11-23. Đọc kèm `docs/API.md` (endpoint `/api/facebook-ads/*`), `docs/DATABASE.md` (bảng `facebook_capi_*`), `docs/LANDING.md` (tracker.js + Pixel), `docs/SECURITY.md` (gate Facebook traffic).

---

## 1. Tổng quan kiến trúc

```
Browser (fbq Pixel) ──┐
                       ├─► Meta Events Manager (gộp theo event_name + event_id, cửa sổ 48h)
Server (CAPI, Go)   ──┘
```

| Kênh | Nguồn phát | File chính | Khi nào bắn |
|---|---|---|---|
| **Pixel (browser)** | `fbq('track', …)` trong HTML landing + `frontend/admin/js/tracker.js` | `backend/handlers/landing_handler.go` (`injectPageViewEventIDSnippet`, `rewritePageViewWithCurrencyValue`, `rewritePageViewWithEventID`) | Mỗi lượt xem / mỗi submit form — chạy trên trình duyệt khách |
| **CAPI PageView (server)** | `AnalyticsHandler.dispatchCAPIPageView` → `Dispatcher.SendPageViewEvent` | `backend/handlers/analytics_handler.go` ~L440-560, `backend/integrations/facebook/dispatcher.go` ~L310 | Ping `POST /api/analytics/track` (goroutine nền) — chỉ khi landing **bật CAPI** và có `event_id` chung với Pixel |
| **CAPI Lead (server)** | `LeadHandler.dispatchCAPILead` → `Dispatcher.SendLeadEvent` | `backend/handlers/lead_handler.go` ~L873, `dispatcher.go` ~L268 | `POST /api/leads` thành công + lead **không phải test** + **đến từ Facebook** |
| **CAPI CRM sync (server)** | `LeadHandler.dispatchCAPICRMSync` → `Dispatcher.SendCRMSyncEvent` | `backend/handlers/lead_handler.go` ~L1035, `dispatcher.go` ~L356 | Sales đổi trạng thái CRM sang trạng thái có ánh xạ Meta |
| **Test event (thủ công)** | `FacebookAdsHandler.HandleTestEvent` → `Dispatcher.SendTestEvent` | `backend/handlers/facebook_ads_handler.go` ~L273, `dispatcher.go` ~L480 | Admin bấm "Gửi test" ở tab Facebook Ads (kèm `test_event_code`) |
| **Dataset Quality (đọc)** | `FacebookAdsHandler.HandleDatasetQuality` → `Client.FetchDatasetQuality` | `backend/handlers/facebook_ads_handler.go` ~L324, `backend/integrations/facebook/quality.go` | Admin bấm "Làm mới" (thủ công — Meta trễ vài giờ, cần ≥ 20 phút sau khi gửi) |

**Nguyên tắc bất biến (không được phá):**

1. CAPI **không bao giờ chặn luồng chính** — lỗi Meta chỉ ghi `facebook_capi_logs` (`status = failed/skipped`), không fail `POST /api/leads` / `POST /api/leads/update`.
2. `landing_cu` + `landing_lznoew` **bị cấm CAPI tầng server tuyệt đối** (`serverCAPIDenyList` + env `META_CAPI_BLOCKED_LANDINGS` → `resolveConfig()` trả `nil` trước khi query DB).
3. Lead test (tên chứa `test`, không phân biệt hoa thường) **không bắn CAPI** (`is_test = TRUE` → skip ở `lead_handler.go`), nhưng vẫn hiện CRM + vẫn đồng bộ Google Sheets.
4. `only_facebook_traffic = TRUE` (mặc định): chỉ gửi khi lead/session **đến từ Facebook** (`DetectFacebookTraffic`). Organic / Google Ads / TikTok / truy cập thẳng → `skipped`.

---

## 2. Graph API v27.0 — 2 endpoint đang dùng

### 2.1. Gửi sự kiện: `POST https://graph.facebook.com/v27.0/{pixel_id}/events`

```go
// backend/integrations/facebook/capi.go
func BuildEventURL(pixelID, apiVersion string, gatewayEnabled bool, gatewayEndpoint, gatewayAccessKey string) string
func (c *Client) SendEvents(ctx, endpoint, accessToken string, req *CAPIRequest) (resp, httpStatus, rawBody, err)
```

Payload chuẩn (ví dụ `Lead`):

```json
{
  "data": [{
    "event_name": "Lead",
    "event_time": 1763900000,
    "event_id": "pv-xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
    "action_source": "website",
    "event_source_url": "https://lophoc.hanghoaphaisinh.net/?fbclid=...",
    "user_data": {
      "em": ["<sha256(lower(trim(email)))>"],
      "ph": ["<sha256(E.164 +84…)>"],
      "fn": ["<sha256>"], "ln": ["<sha256>"],
      "ct": ["<sha256(ha noi)>"], "st": ["<sha256(ha noi)>"],
      "country": ["<sha256(vn)>"],
      "external_id": ["<sha256(sđt E.164)>"],
      "client_ip_address": "113.160.x.x",
      "client_user_agent": "Mozilla/5.0 …",
      "fbc": "fb.1.1763900000000.IwZXh0bgNhZW0…",
      "fbp": "fb.1.1763900000000.123456789"
    },
    "custom_data": { "currency": "VND", "value": 0, "content_name": "lead" }
  }],
  "test_event_code": "TEST90946",
  "partner_agent": "apex_crm"
}
```

| Trường | Chuẩn Meta (v27.0) | Code thực thi |
|---|---|---|
| `event_name` | `PageView` · `Lead` · `Contact` · `StartTrial` · `Subscribe` · `Purchase` | `event.go` (`EventPageView`, …) |
| `event_time` | Unix giây, trong 7 ngày gần nhất (khuyến nghị < 24h) | `BuildEvent()` — mặc định `time.Now().Unix()` |
| `event_id` | **Khóa dedup** — xem mục 3 | `GenerateEventID()` (UUID v4) hoặc giữ nguyên id client gửi lên |
| `action_source` | Luôn `"website"` | hardcode trong `BuildEvent()` |
| `event_source_url` | URL landing khách đang xem (kèm query) | `lc.URL` / `EventSourceURL` |
| `test_event_code` | Chỉ khi test — mã do Events Manager sinh ra | `SendTestEvent()` — `TEST90946` cho dataset `1468058901923737` |
| `partner_agent` | Định danh nền tảng, lowercase | `PartnerAgentName = "apex_crm"` (`capi.go`) — dùng chung cho cả gửi event và đọc Dataset Quality |

**Gateway (tùy chọn):** nếu bật, URL base là `https://<region>.gateway.facebook.com`, access key truyền bằng query `?access_key=<KEY>` (không phải `access_token`). Payload vẫn là `{ "data": […] }`. Xem `BuildEventURL()`.

### 2.2. Chẩn đoán: `GET https://graph.facebook.com/v27.0/dataset_quality`

```go
// backend/integrations/facebook/quality.go
func (c *Client) FetchDatasetQuality(ctx, apiVersion, pixelID, accessToken, agentName string) (raw, httpStatus, body, err)
func ParseDatasetQuality(raw map[string]interface{}, pixelID string) DQSummary
```

Query (bắt buộc + tùy chọn):

```
dataset_id=<PIXEL_ID>          ⛔ bắt buộc — thiếu → Meta trả lỗi 100
access_token=<TOKEN>           ⛔ bắt buộc — cần quyền ads_read / ads_management
agent_name=apex_crm            tùy chọn — PHẢI khớp partner_agent lúc gửi (lowercase)
fields=web{event_name,event_match_quality{…},acr{…},event_coverage{…},
  dedupe_key_feedback{…},data_freshness{…},event_spend{…},
  event_ad_sets{…},value_diagnostics{…},event_delay{…}}
```

**⚠️ Điểm mấu chốt v27.0: `web` là MẢNG** — mỗi phần tử = 1 `event_name` kèm toàn bộ chỉ số riêng. Bản cũ parse `web` như 1 object ⇒ mọi chỉ số rỗng. `ParseDatasetQuality()` duyệt mảng và gom theo event (có fallback object đơn để tương thích).

Mỗi phần tử `web[]` gồm:

| Khối | Ý nghĩa | Ngưỡng / lưu ý |
|---|---|---|
| `event_match_quality.composite_score` | Điểm EMQ 0–10 | Càng cao càng tốt; xem mục 5 để nâng |
| `event_match_quality.match_key_feedback[]` | Coverage từng khóa (`email`, `phone`, `fbc`, `ip_address`, `external_id`…) + `potential_aly_acr_increase` | Thêm khóa thiếu có coverage cao để tăng EMQ |
| `event_match_quality.diagnostics[]` | 6 nhóm lỗi EMQ mới (từ 09/2026), gồm `"Server sending mismatched IP addresses"` | Xem mục 5.3 — đã fix bằng `isPublicIP()` |
| `acr` | Tỷ lệ gán (attribution credit rate) | — |
| `event_coverage` | % sự kiện có đủ dữ liệu | **Mục tiêu 75%** (`goal_percentage`) |
| `dedupe_key_feedback[]` | `dedupe_key` = `event_id` / `external_id` / `fbp`; `% browser/server có khóa`; `overall_browser_coverage_from_dedupe_key` | `event_id` phải 100% cả 2 phía (mục 3) |
| `data_freshness` | `upload_frequency` — kỳ vọng `"real_time"` | Gửi trong goroutine ngay khi lead về / ping analytics |
| `event_spend` | `{ amount, currency_code }` | — |
| `event_ad_sets.data[]` | Chi tiêu theo ad set | — |
| `value_diagnostics[]` | `MISSING_OR_INVALID_VALUE` + `issue_rate` | Xem mục 6 — `currency` + `value` bắt buộc |
| `event_delay` | `events_delayed_over_2h/24h`, `pct_delayed_over_24h` | Gửi real-time ⇒ delay ≈ 0 |

`DQSummary.Aggregate` gộp nhanh: `event_count`, `avg_emq`, `avg_coverage`, `total_spend`, `critical_issues` (= số `value_diagnostics` + số `diagnostics` EMQ).

---

## 3. Dedup 2 nguồn (Pixel browser ↔ CAPI server) — `event_id` dùng chung

Meta chỉ gộp browser ↔ server khi **cùng `(event_name, event_id)`** trong cửa sổ **48 giờ**. Không đồng bộ ⇒ 1 lượt xem bị đếm 2 lần PageView (đúng lỗi trong CSV audit — xem mục 4).

### 3.1. PageView — `pv-*`

```
landing HTML (<head>)                    tracker.js (browser)              server (Go)
─────────────────────                    ──────────────────              ─────────────
injectPageViewEventIDSnippet()           ensurePageViewEventId()          dispatchCAPIPageView()
 sinh window.APEX_PV_EVENT_ID ──đọc──► fbq('track','PageView',{},        đọc p.EventID (payload)
 (pv-<uuid>, 1/phiên)                    {eventID: pvEventId})  ──gửi──► hoặc fallback
      │                                   │   ▲ eventID là THAM SỐ        analytics_sessions
      └─ rewritePageViewWithEventID() ────┘   THỨ 4 của fbq               .pv_event_id
         (ghi đè mọi fbq('track',            (KHÔNG phải field            rồi truyền NGUYÊN
          'PageView') hardcode)               trong object tham số 3)     VĂN vào CAPI event_id
```

Chi tiết code:

- **Snippet đầu `<head>`** (`landing_handler.go` → `injectPageViewEventIDSnippet()`): sinh 1 `window.APEX_PV_EVENT_ID = "pv-<uuid>"` duy nhất mỗi lượt tải trang.
- **Ghi đè hardcode** (`rewritePageViewWithEventID()`): mọi `fbq('track','PageView')` trong HTML export từ LadiPage được viết lại thành `fbq('track','PageView',{},{eventID: window.APEX_PV_EVENT_ID})`. Block Meta Pixel auto-inject cũng kèm eventID.
- **Client** (`tracker.js`): `ensurePageViewEventId()` đọc `window.APEX_PV_EVENT_ID`; `buildAnalyticsPayload()` gửi `event_id` lên `POST /api/analytics/track`.
- **Server** (`analytics_handler.go` → `dispatchCAPIPageView()`): ưu tiên `p.EventID` (client gửi kèm) → fallback `GetSessionPvEventID(sessionID)` (đọc `analytics_sessions.pv_event_id` — giữ giá trị ĐẦU TIÊN của phiên). **Landing có Pixel nhưng không có `event_id` chung ⇒ KHÔNG bắn server PageView** (chấp nhận thiếu 1 lượt thay vì thừa 1 lượt).
- **Chỉ gắn `eventID` khi landing thực sự bật CAPI** (`isCAPIActive(landing_id)`): landing không bật CAPI (vd `landing_cu` — chỉ dùng Pixel) giữ nguyên block Meta Pixel gốc.

### 3.2. Lead — dùng lại `event_id` gốc

- Submit form: `tracker.js` sinh event_id cho `fbq('track','Lead',{…},{eventID})` rồi gửi kèm trong `POST /api/leads` (`tracking.event_id` / `eventId` / `fb_event_id`).
- Server (`lead_handler.go` → `buildCAPILeadContext()`): đọc 3 key trên, truyền nguyên văn vào `LeadContext.EventID` → `SendLeadEvent()` → CAPI `event_id`.
- CRM sync trạng thái `moi` → event `Lead`: **dùng lại event_id gốc** (`lcForEvent = lc`) ⇒ Meta gộp với sự kiện browser/server lúc submit ⇒ không thổi phồng số Lead.
- CRM sync trạng thái khác (`Contact` / `StartTrial` / `Subscribe` / `Purchase`): **sinh event_id mới** (clone `lc` + xóa `EventID`) ⇒ không bị Meta gộp nhầm vào event `Lead` ⇒ không mất sự kiện chuyển đổi.

---

## 4. Dedup 1 nguồn (server bắn trùng) — root cause từ CSV + 3 chốt chặn

### 4.1. Bằng chứng: `1468058901923737_PageView_EventDeduplicationSamples_ALL_DOMAINS.csv`

File 100 mẫu dedup PageView của Events Manager cho `https://lophoc.hanghoaphaisinh.net/` (`event_id` dạng `pv-*`):

| Mẫu | Ý nghĩa | Số dòng |
|---|---|---|
| browser 1 + server 1 | ✅ Chuẩn (Pixel + CAPI gộp thành 1) | 72 |
| browser 1 + server 2 | ❌ Server bắn trùng 2 lần cùng `event_id` | 22 |
| browser 0 + server 3 | ❌ Server bắn trùng 3 lần, Pixel không thấy | 2 |
| browser 0 + server 2 | ❌ Server bắn trùng 2 lần, Pixel không thấy | 4 |

⇒ **28% event bị server bắn 2–3 lần** cho cùng 1 `event_id`. Pixel (browser) chỉ bắn 0–1 lần ⇒ lỗi nằm ở **tầng server**.

**Root cause (race condition):** `tracker.js` gửi nhiều ping `POST /api/analytics/track` trong 1 phiên (`pageview`, `heartbeat`, `exit`, `scroll…`). Mỗi ping chạy `dispatchCAPIPageView()` trên 1 goroutine riêng. Hai ping gần nhau cùng qua check DB `HasFacebookCAPISessionEvent = false` **trước khi log kịp ghi** ⇒ cả hai cùng gọi Meta ⇒ 2–3 server events cho cùng 1 `event_id`.

### 4.2. Chốt chặn 1 — flight-lock trong bộ nhớ (chặn trùng SONG SONG)

```go
// backend/handlers/analytics_handler.go — AnalyticsHandler struct + dispatchCAPIPageView()
pageViewFlightMu sync.Mutex
pageViewFlight   map[string]bool  // key = session_id (fallback visitor_id|landing_id), TTL 30s
```

Ping đầu đánh dấu phiên đang gửi; ping song song thấy cờ thì bỏ qua. Flag tự xóa sau 30s (chống kẹt nếu goroutine crash). Chỉ chặn được trùng **đồng thời** — 2 ping cách nhau > 30s hoặc khác process vẫn lọt.

### 4.3. Chốt chặn 2 — dedup theo session trong DB (chặn trùng KHÁC event_id, 24h)

```go
// backend/integrations/facebook/dispatcher.go — SendPageViewEvent()
sent, _ := d.store.HasFacebookCAPISessionEvent(lc.SessionID, EventPageView) // 24h, status IN (success, skipped)
if sent { return skipped("PageView đã gửi cho session này (dedup)") }
```

```sql
-- backend/db/facebook_ads.go
SELECT EXISTS (
  SELECT 1 FROM facebook_capi_logs
  WHERE session_id = $1 AND event_name = $2
    AND status IN ('success','skipped')
    AND created_at >= NOW() - INTERVAL '24 hours');
```

1 phiên chỉ gửi 1 PageView/ngày — chặn được ping `heartbeat`/`exit` gửi `event_id` khác nhau.

### 4.4. Chốt chặn 3 (CUỐI) — dedup theo `(event_name, event_id)` trong DB (48h, khớp cửa sổ Meta)

```go
// backend/integrations/facebook/dispatcher.go — send() (áp dụng MỌI sự kiện, không chỉ PageView)
if d.store != nil && ev.EventID != "" {
    if dup, _ := d.store.HasFacebookCAPIEventID(ev.EventID, eventName); dup {
        return skipped("event_id đã gửi trong 48h (dedup)");
    }
}
```

```sql
-- backend/db/facebook_ads.go — HasFacebookCAPIEventID()
SELECT EXISTS (
  SELECT 1 FROM facebook_capi_logs
  WHERE event_id = $1 AND event_name = $2
    AND status = 'success'
    AND created_at >= NOW() - INTERVAL '48 hours');
```

Khớp đúng cửa sổ dedup 48h của Meta ⇒ server cũng chỉ gửi 1 lần cho mỗi cặp `(event_name, event_id)` trong 48h. 2 ping cùng session + cùng `event_id` cùng lọt qua chốt 1+2 vẫn bị chặn ở đây (ping thứ 2 thấy log `success` của ping thứ 1).

---

## 5. EMQ (Event Match Quality) — chuẩn hóa + hashing PII

### 5.1. Bảng chuẩn hóa (thực thi trong `backend/integrations/facebook/*.go`)

| Khóa Meta | Chuẩn | Code | Ví dụ |
|---|---|---|---|
| `em` | trim + lowercase → SHA-256 | `NormalizeEmail` → `HashEmail` (`hashing.go`) | `"  NGUYEN@Example.COM "` → `nguyen@example.com` → hash |
| `ph` | **E.164** → SHA-256 (Meta chỉ khớp E.164) | `NormalizePhone` → `HashPhone` | `0912345678` → `+84912345678` → hash; `+84 912-345-678` → `+84912345678` |
| `fn` / `ln` | lowercase + gộp khoảng trắng → SHA-256 | `SplitFullName` + `NormalizeText` → `HashText` (`event.go`, `hashing.go`) | `"Nguyễn Văn A"` → fn=`"nguyễn văn"`, ln=`"a"` |
| `ct` | **bỏ dấu** + lowercase → SHA-256 | `BuildCity` → `NormalizeCity` (`attribution.go`, `hashing.go`) | `"Hà Nội"` → `ha noi` → hash |
| `st` | bỏ dấu + lowercase; **BỎ QUA vùng kinh tế** | `BuildState` (`attribution.go`) | `"Miền Bắc"` → `""` (không gửi — Meta không có khái niệm vùng); `"Hà Nội"` → `ha noi` |
| `zp` | trim + lowercase → SHA-256 | `HashText` | `"100000"` |
| `country` | **ISO-2** lowercase → SHA-256 | `BuildCountryISO` → `NormalizeCountry` | `"Việt Nam"` / `"Vietnam"` / `"VN"` → `vn` |
| `external_id` | SĐT E.164 (fallback text chuẩn hóa) → SHA-256 | `HashExternalID` | `0912345678` → `+84912345678` → hash |

Tất cả hash đều là **SHA-256 hex 64 ký tự**, không hash thì không gửi (tránh khối rỗng — `BuildUserData()` trả `nil` khi không có PII nào).

Bỏ dấu tiếng Việt (`StripVietnameseDiacritics`) dùng bảng tra rune thủ công (không phụ thuộc `unicode/norm` — Go trên server không có package đó).

### 5.2. Trường KHÔNG băm (gửi rõ)

| Trường | Chuẩn | Code |
|---|---|---|
| `client_ip_address` | IPv4/IPv6 **public** (mục 5.3) | `withIP()` trong `send()` + `SendTestEvent()` |
| `client_user_agent` | UA gốc của trình duyệt | `withUA()` |
| `fbc` | `fb.1.<creation_time_ms>.<fbclid>` | `NormalizeFBC()` — xem 5.4 |
| `fbp` | `fb.1.<creation_time_ms>.<random>` | `NormalizeFBP()` |

IP/UA gốc của sự kiện CRM sync được fallback từ `tracking.server_ip` / `tracking.server_user_agent` đã lưu lúc tạo lead (`buildCAPILeadContext()`), vì lúc sales đổi trạng thái không còn request của khách.

### 5.3. Lọc IP private — fix diagnostic `"Server sending mismatched IP addresses"`

```go
// backend/integrations/facebook/dispatcher.go
func isPublicIP(ip string) bool {
    parsed := net.ParseIP(strings.TrimSpace(ip))
    if parsed == nil { return false }
    if parsed.IsLoopback() || parsed.IsPrivate() || parsed.IsUnspecified() || parsed.IsLinkLocalUnicast() { return false }
    return true
}
// Trong send() + SendTestEvent(): if isPublicIP(lc.ClientIP) { ev.UserData = withIP(…) }
```

IP local (`127.0.0.1`, `::1`, `10/8`, `172.16/12`, `192.168/16`, link-local…) không bao giờ khớp IP browser mà Meta thấy ⇒ gửi lên chỉ làm nhiễu diagnostic + hạ EMQ ⇒ bỏ qua. Áp dụng cho cả gửi thật và gửi test.

### 5.4. `_fbc` — timestamp dựng lại phải là thời gian thật (ms)

```go
// backend/integrations/facebook/params.go
func NormalizeFBC(fbc, fbclid string) string {
    // đã đúng fb.1.<ts>.<fbclid> (≥ 4 phần split ".") → giữ nguyên
    // fbclid thô → "fb.1." + time.Now().UnixMilli() + "." + fbclid
}
```

Trước 2026-11-23 timestamp dựng lại luôn là `"0"` (`fb.1.0.<fbclid>`). Meta dùng `creation_time` để tính độ tươi click và chấm EMQ khóa `fbc` ⇒ nay dùng `time.Now().UnixMilli()`.

---

## 6. `custom_data` — `currency` + `value` bắt buộc (fix cảnh báo 100% Events Manager)

LadiPage hardcode `fbq('track','Lead')` không kèm `custom_data` ⇒ Events Manager báo "100% sự kiện Lead thiếu thông số giá trị và đơn vị tiền tệ". Sửa 3 tầng (loại trừ `landing_cu` + `landing_lznoew`):

1. **Client** (`tracker.js`): `fbq('track','Lead',{content_name, currency:'VND', value:0},{eventID})`.
2. **Server lúc serve HTML** (`landing_handler.go` → `rewriteLeadWithCurrencyValue()`): ghi đè lệnh Lead trong HTML ⇒ `fbq('track','Lead',{currency:'VND',value:0})`.
3. **Server lúc boot** (`db/landing_pixel_repair.go` → `RepairMetaPixelLeadCurrency()`): sửa dữ liệu DB.

Quy tắc `value` cho CAPI (thực thi trong `dispatcher.go` — `SendLeadEvent`, `SendCRMSyncEvent`, `SendTestEvent` + chốt chặn cuối ép `StartTrial/Subscribe/Purchase > 0`):

| Sự kiện | `currency` | `value` |
|---|---|---|
| `Lead` (mới + test) | `VND` | `0` (khách quan tâm — chuyển đổi thật nhập tay ở CRM sau) |
| `PageView`, `Contact` | `VND` | `0` |
| `StartTrial` (`da_hoc_buoi_chia_se`) | `VND` | `1` symbolic — Meta **từ chối `value = 0`** ("phải là số lớn hơn 0"); dùng 1 VND để qua diagnostic mà không phồng ROAS |
| `Subscribe` (`da_hoc_chuyen_sau`) | `VND` | tiền nhập tay CRM → fallback `course_value` cấu hình → cuối cùng `6000000` (PHẢI > 0) |
| `Purchase` (`da_mua_hang`) | `VND` | như `Subscribe` |

CRM nhập tay: `leads.conversion_value` (VND) + `leads.lead_score` (thang Meta 0–10) trong drawer Customer 360° ⇒ đồng bộ CAPI + Sheets + CSV. `Send*` nào cũng kèm `custom_data { currency:"VND", value }` + `lead_score` tùy chọn. `CustomData.Extra` (`json:"-"` + `MarshalJSON()` merge) giữ lại mọi field tùy biến do Admin UI gửi lên khi test (trước đây bị lặng lẽ loại bỏ).

---

## 7. Ánh xạ CRM → Meta + gate Facebook traffic

### 7.1. Pipeline 10 trạng thái (`backend/models/lead_status.go`)

`moi` → `da_goi` → `ket_ban_zalo` → `da_hoc_buoi_chia_se` (miễn phí) → `da_hoc_chuyen_sau` (≈6.000.000đ) → `da_mua_hang`, kèm kết thúc không mua: `da_hoc_truoc_do` · `khong_quan_tam` · `spam_khong_lien_lac` · `huy`. Tương thích ngược: `tiem_ngan`/`da_vao_zoom` → `da_hoc_buoi_chia_se`, `chot` → `da_mua_hang`. UI render động từ `GET /api/facebook-ads/lead-statuses`.

| Trạng thái CRM | Event Meta | `hasValue` | Không ánh xạ (không bắn) |
|---|---|---|---|
| `moi` | `Lead` (dùng lại event_id gốc ⇒ dedup với `fbq`) | — | `ket_ban_zalo`, `da_hoc_truoc_do`, `khong_quan_tam`, `spam_khong_lien_lac`, `huy` |
| `da_goi` | `Contact` | — | |
| `da_hoc_buoi_chia_se` | `StartTrial` (+`value` = 1) | — | |
| `da_hoc_chuyen_sau` | `Subscribe` (+`value` = tiền khóa học) | ✅ | |
| `da_mua_hang` | `Purchase` (+`value`) | ✅ | |

### 7.2. Gate `only_facebook_traffic` (mặc định TRUE)

`DetectFacebookTraffic()` (`attribution.go`) — thứ tự ưu tiên bằng chứng (mạnh → yếu): `fbc` → `fbclid` → `site_source_name`/`utm_source` ∈ `{fb, ig, msg, an, facebook, instagram, messenger, …}` → referrer Meta + `utm_medium=paid` → `fbp` (yếu nhất). Không có bằng chứng ⇒ `skipped("lead không đến từ Facebook")`.

Ánh xạ URL Meta Ads (**không** theo Google Ads): `utm_campaign` = `campaign.id`, `utm_content` = `adset.id`, `utm_term` = `ad.id`; `campaign_name`/`adset_name`/`ad_name` là tên hiển thị. Parse trong `params.go` → lưu vào `leads.tracking` → báo cáo `GET /api/facebook-ads/campaigns`.

---

## 8. Partner Agent — `apex_crm` (đồng nhất 2 đầu)

```go
// backend/integrations/facebook/capi.go
const PartnerAgentName = "apex_crm"
```

Theo tài liệu Meta "Thư viện công cụ tạo thông số": `partner_agent` trong `POST /{pixel_id}/events` và `agent_name` trong `GET dataset_quality` phải là **cùng 1 giá trị lowercase**. Code dùng chung hằng số ở cả `send()` + `SendTestEvent()` (`partner_agent`) và `HandleDatasetQuality` (`agent_name`) ⇒ Dataset Quality lọc đúng sự kiện do hệ thống gửi. (Trước đây gửi `"apex-crm"` có gạch ngang ⇒ không khớp chuẩn lowercase của Meta.)

---

## 9. Test với `TEST90946` trên dataset `1468058901923737`

### 9.1. Chuẩn bị

1. Mở Events Manager → dataset `1468058901923737` → tab **Test Events** → server tạo mã `TEST90946` (mã hết hạn theo phiên — nếu Meta xoay mã mới thì dùng mã mới).
2. CRM → tab **Facebook Ads** → chọn landing (nên tạo 1 cấu hình riêng cho landing test, `is_active = TRUE`, `only_facebook_traffic` có thể tắt tạm để test không cần traffic FB thật).
3. ⚠️ Môi trường `test` (`APEX_DISABLE_EXTERNAL_SYNC=true`) **không gửi ra ngoài** — muốn thấy event trong Test Events phải test ở LOCAL trỏ token thật hoặc PRODUCTION (cân nhắc — test event không vào dữ liệu thật nên an toàn).

### 9.2. Gửi test từ Admin UI

`POST /api/facebook-ads/test-event` (`RequireAdminAuth`):

```json
{
  "landing_id": "<landing test>",
  "event_name": "Lead",
  "event_id": "test-0001",
  "email": "nguyen.van.a@example.com",
  "phone": "0912345678",
  "first_name": "Nguyễn Văn",
  "last_name": "A",
  "city": "Hà Nội",
  "country": "VN",
  "value": 0,
  "currency": "VND",
  "event_source_url": "https://lophoc.hanghoaphaisinh.net/?fbclid=TEST",
  "test_event_code": "TEST90946",
  "custom_data": { "content_name": "lead" }
}
```

Kiểm tra trong Test Events của Meta: event hiện trong vài giây, mở rộng để xem `user_data` đã hash (64 hex), `client_ip_address` là IP public thật (không phải `127.0.0.1`), `fbc` có timestamp ms hiện tại, `custom_data` có `currency: VND`.

### 9.3. Gửi test bằng cURL (trực tiếp Meta, bỏ qua backend)

```bash
curl -X POST "https://graph.facebook.com/v27.0/1468058901923737/events?access_token=<ACCESS_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{
    "data": [{
      "event_name": "Lead",
      "event_time": 1763900000,
      "event_id": "test-0001",
      "action_source": "website",
      "event_source_url": "https://lophoc.hanghoaphaisinh.net/",
      "user_data": {
        "em": ["<sha256 của nguyen.van.a@example.com>"],
        "ph": ["<sha256 của +84912345678>"],
        "client_ip_address": "<IP public thật>",
        "client_user_agent": "Mozilla/5.0"
      },
      "custom_data": { "currency": "VND", "value": 0 }
    }],
    "test_event_code": "TEST90946",
    "partner_agent": "apex_crm"
  }'
```

### 9.4. Đọc Dataset Quality từ CRM

`GET /api/facebook-ads/dataset-quality?landing_id=<id>` → backend gọi Meta với `agent_name=apex_crm` + `fields=<DatasetQualityFields>`, parse `web[]` thành `DQSummary` (UI vẽ EMQ, coverage, dedupe, freshness, spend, value diagnostics, delay). **Chỉ gọi thủ công** (nút "Làm mới") — Meta cần ≥ 20 phút sau khi gửi và số liệu thường trễ vài giờ.

Checklist sau khi gửi test + chờ đủ lâu:

- [ ] `dedupe_key_feedback[event_id]`: browser 100% + server 100%.
- [ ] Không còn dòng `server_events ≥ 2` cho cùng `event_id` (đối chiếu `facebook_capi_logs`: `SELECT event_id, event_name, COUNT(*) FROM facebook_capi_logs WHERE status='success' AND created_at >= NOW() - INTERVAL '48 hours' GROUP BY 1,2 HAVING COUNT(*) > 1;` phải rỗng).
- [ ] Hết diagnostic `"Server sending mismatched IP addresses"`.
- [ ] Hết `value_diagnostics[MISSING_OR_INVALID_VALUE]` (mọi event đều có `currency` + `value`).
- [ ] `data_freshness.upload_frequency = real_time`, `event_delay ≈ 0`.

---

## 10. File tham chiếu nhanh

| File | Vai trò |
|---|---|
| `backend/integrations/facebook/capi.go` | Struct payload, `PartnerAgentName`, `Client.SendEvents`, `CustomData.MarshalJSON` |
| `backend/integrations/facebook/event.go` | `BuildEvent`, `BuildUserData`, `BuildValueCustomData`, `GenerateEventID`, `SplitFullName` |
| `backend/integrations/facebook/hashing.go` | Chuẩn hóa + SHA-256 PII, `StripVietnameseDiacritics` |
| `backend/integrations/facebook/attribution.go` | `BuildCountryISO/City/State`, `DetectFacebookTraffic` |
| `backend/integrations/facebook/params.go` | Parse URL Meta Ads, `NormalizeFBC/FBP` |
| `backend/integrations/facebook/quality.go` | `FetchDatasetQuality`, `ParseDatasetQuality`, `DatasetQualityFields` |
| `backend/integrations/facebook/dispatcher.go` | `SendLeadEvent/PageViewEvent/CRMSyncEvent/TestEvent`, `send()` + 3 chốt dedup, `isPublicIP`, deny-list |
| `backend/handlers/facebook_ads_handler.go` | CRUD config, test event, dataset quality, logs, stats, campaigns, parse-url, lead-statuses |
| `backend/handlers/lead_handler.go` | `dispatchCAPILead`, `buildCAPILeadContext`, `dispatchCAPICRMSync` |
| `backend/handlers/analytics_handler.go` | `dispatchCAPIPageView` (flight-lock + event_id chung), `landingHasMetaPixel` |
| `backend/handlers/landing_handler.go` | `injectPageViewEventIDSnippet`, `rewritePageViewWithEventID`, `rewriteLeadWithCurrencyValue` |
| `backend/db/facebook_ads.go` | `HasFacebookCAPISessionEvent` (24h), `HasFacebookCAPIEventID` (48h), `GetSessionPvEventID`, logs/stats |
| `backend/models/facebook_ads.go` | `FacebookCAPIConfig`, `FacebookCAPIEvent`, `FacebookCAPIEventRequest` |
| `backend/models/lead_status.go` | Pipeline 10 trạng thái, `LeadStatusCAPIEvent` |
| `frontend/admin/js/tracker.js` | `ensurePageViewEventId`, `buildAnalyticsPayload`, `fbq(...{eventID})`, `currency/value` cho Lead |

---

## 11. Lịch sử fix liên quan (để tra cứu)

- **2026-11-15:** gate `only_facebook_traffic` + ánh xạ CRM→Meta + chuẩn PII (ISO-2, bỏ dấu, E.164).
- **2026-11-16:** dedup PageView Pixel↔CAPI bằng `event_id` chung (`window.APEX_PV_EVENT_ID`, tham số thứ 4 của `fbq`); lead test (`is_test`) không bắn CAPI.
- **2026-11-18:** deny-list `landing_cu`/`landing_lznoew` (chỉ Pixel, tắt CAPI server); `isCAPIActive` quyết định có gắn `eventID`; flight-lock `pageViewFlight` (chốt 1).
- **2026-11-19:** `currency:VND + value` bắt buộc cho Lead (3 tầng: tracker + rewrite HTML + repair DB); `conversion_value` + `lead_score` nhập tay ở CRM.
- **2026-11-21:** `StartTrial value = 1` (Meta từ chối `value = 0`).
- **2026-11-23 (đợt này):** `partner_agent`/`agent_name` đồng nhất `apex_crm`; `isPublicIP()` lọc IP local; `NormalizeFBC` dùng timestamp ms thật; `CustomData.Extra` merge khi serialize; chốt dedup cuối `(event_name, event_id)` 48h trong `send()` — xử lý triệt để 28% server-event trùng trong CSV audit.
