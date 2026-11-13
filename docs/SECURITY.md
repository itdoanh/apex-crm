# Hệ thống Bảo mật & Chống Spam

Hệ thống xử lý traffic quảng cáo tài chính có **CPA cao** → mỗi lead hợp lệ có giá trị lớn, nhưng bot spam cũng nhắm vào đúng form này. Bảo mật được thiết kế **nhiều lớp**, không dựa vào một biện pháp duy nhất.

---

## 1. Kiến trúc phòng thủ nhiều lớp

```
                    Request POST /api/leads
                              │
        ┌─────────────────────▼─────────────────────┐
        │ 1. Rate Limit (theo IP)                   │──► 429, không ghi DB
        └─────────────────────┬─────────────────────┘
                              │
        ┌─────────────────────▼─────────────────────┐
        │ 2. Proof-of-Work (SHA-256 challenge)      │──► 403 → spam_quarantine
        │    + HMAC signature + replay protection    │    (trừ duplicate_submit →
        │                                            │     idempotent 200 OK)
        └─────────────────────┬─────────────────────┘
                              │
        ┌─────────────────────▼─────────────────────┐
        │ 3. Honeypot (field ẩn bất khả kiểm)       │──► 403 → spam_quarantine
        └─────────────────────┬─────────────────────┘
                              │
        ┌─────────────────────▼─────────────────────┐
        │ 4. Behavioral metrics                      │──► đánh dấu is_bot
        │    time_on_page / mouse / scroll / clicks  │
        └─────────────────────┬─────────────────────┘
                              │
        ┌─────────────────────▼─────────────────────┐
        │ 5. Phone validation (VN 10 số)             │──► 400
        │    + carrier detection                     │
        └─────────────────────┬─────────────────────┘
                              │
        ┌─────────────────────▼─────────────────────┐
        │ 6. GeoIP enrich + ad placement parse       │
        └─────────────────────┬─────────────────────┘
                              │
                     ┌────────▼────────┐
                     │  SAVE LEAD ✅   │──► Google Sheets
                     └─────────────────┘
```

---

## 2. Proof-of-Work Challenge

**File:** `backend/security/challenge.go`

### Cách hoạt động

**Server** (`GET /api/auth/challenge?difficulty=3`):
```go
challengeID := "ch_" + hex(random 16 bytes)
salt        := hex(random 16 bytes)
expiresAt   := now + 10 phút          // challengeTTL
message     := challengeID + ":" + salt + ":" + difficulty + ":" + expiresAt
signature   := HMAC-SHA256(message, challengeSecretKey)
```
Server trả về `{ challenge_id, salt, difficulty, expires_at, signature }`.

**Client** (`tracker.js`) brute-force:
```javascript
// tìm solution sao cho SHA256(challenge_id + ":" + salt + ":" + solution)
// bắt đầu bằng "000" (difficulty=3)
```

**Server verify** (`VerifyChallenge`) — 4 bước:
1. **Hết hạn?** `now + 60s > expiresAt` → fail. (Dung sai lệch đồng hồ 60 giây — `challengeClockSkewGrace`)
2. **Chữ ký HMAC hợp lệ?** So sánh bằng `hmac.Equal` (constant-time) → fail nếu client sửa difficulty/expiresAt.
3. **Đúng bài toán?** `SHA256(challenge_id + ":" + salt + ":" + solution)` phải bắt đầu bằng `difficulty` chữ `"0"`.
4. **Replay?** Key = `challenge_id:solution` lưu trong `sync.Map` (`usedNonces`), TTL 10 phút → đã dùng thì fail.

> ⚠️ **Thứ tự cố ý: kiểm tra hash TRƯỚC khi đánh dấu nonce đã dùng.**
> Nhờ vậy lời giải sai không bao giờ ghi vào `usedNonces` ⇒ lần gửi hợp lệ sau đó
> vẫn dùng được nonce cũ, không cần xoá thủ công (tránh race với request song song).

### Mã lý do (PowCode) và phân loại "gửi trùng do client"

`VerifyChallenge` trả về `PowResult{OK, Code, Reason}` với các mã:

| Mã | Ý nghĩa | Handler xử lý |
|---|---|---|
| `ok` | PoW hợp lệ | Tạo lead + đẩy Google Sheets |
| `missing_params` | Thiếu tham số challenge | → `spam_quarantine` (shadow ban) |
| `expired` | Challenge hết hạn | → `spam_quarantine` |
| `bad_signature` | HMAC sai (client sửa tham số) | → `spam_quarantine` |
| `invalid_solution` | Hash không thỏa điều kiện | → `spam_quarantine` |
| `replay` | Nonce đã dùng, **fingerprint KHÁC** ⇒ bot chơi lại payload | → `spam_quarantine` |
| `duplicate_submit` | Nonce đã dùng, **fingerprint GIỐNG** ⇒ client gửi trùng | **KHÔNG ghi cách ly**, trả 200 OK idempotent |

### Fingerprint phiên đăng ký — chống gắn nhầm lead thật là spam

**BỐI CẢNH (bug 2026-11-05):** lead thật #32 (`Maivanrot` / `0345833454`) về CRM + Google Sheets
đúng, nhưng đồng thời bị ghi vào `spam_quarantine` 2 lần với lý do **"Lỗi Proof-of-Work"**
(`pow_invalid: Thử thách này đã được sử dụng (Replay attack detected)`).

**Nguyên nhân:** landing có **3 handler submit chạy song song cho 1 lần bấm**:

1. capture listener trong HTML (`document.addEventListener('submit', ..., true)`)
2. inline `onsubmit="handleFormSubmit(event)"` trong HTML
3. interceptor của `tracker.js` (`initAutoFormInterceptor`)

Cả 3 đều gọi `ApexTracker.sendLeadToBackend()` ⇒ **cùng 1 payload** (kể cả cùng
`challenge_id` + `challenge_solution`) được POST lên `/api/leads` 3 lần. Request #1 tạo lead
đúng; request #2, #3 dùng lại nonce PoW đã tiêu thụ ⇒ bị coi là replay ⇒ ghi vào vùng cách ly
⇒ **lead thật bị hiển thị sai là "Lỗi Proof-of-Work"** trong tab Bảo mật & Bot.

**Cách sửa (3 lớp):**

1. **Server — fingerprint** (`backend/handlers/lead_handler.go` → `buildChallengeFingerprint`):
   server tính fingerprint = `session_id | visitor_id | SĐT đã chuẩn hóa | IP`.
   Nonce được lưu kèm fingerprint (nguyên tử qua `sync.Map.LoadOrStore`).
   - Nonce đã dùng + **fingerprint giống** → `duplicate_submit` ⇒ **bỏ qua, KHÔNG ghi cách ly**,
     trả `200 OK` idempotent (lead đã được tạo bởi request gốc).
   - Nonce đã dùng + **fingerprint khác** → `replay` ⇒ ghi vào vùng cách ly (chống bot còn nguyên hiệu lực).
   - Cửa sổ `duplicateSubmitWindow` = **60 giây** (`security/challenge.go`).

2. **Client — khoá gửi theo SĐT** (`frontend/landing/tracker.js` → `LEAD_DEDUP_WINDOW_MS = 30000`):
   `sendLeadToBackend()` bỏ qua lời gọi thứ 2 trở đi với cùng SĐT trong 30 giây.
   Khoá được **mở lại** nếu request lỗi mạng / server trả lỗi để khách bấm lại được.

3. **Client — bỏ handler trùng** (`frontend/landing/index.html`, `frontend/landing/ladi.html`):
   - `handleFormSubmit()` **không** gọi `sendLeadToBackend()` nữa (capture listener đã gửi).
   - Set `window.__REGISTRATION_FORM_HANDLED__ = true` ⇒ interceptor của `tracker.js`
     tự bỏ qua form `registrationForm`.

4. **Server — bảo đảm client thực sự NHẬN được code mới** (fix 2026-11-05):

   3 lớp bảo vệ phía client ở trên chỉ có tác dụng nếu trình duyệt nạp được
   `tracker.js` **bản mới**. Trên production đã xảy ra 3 sự cố khiến client chạy
   code cũ hoặc không chạy:

   | # | Sự cố | Hậu quả | Fix |
   |---|---|---|---|
   | a | `GetLandingFile()` không `ORDER BY` | Serve bản `domain=''` cũ (thiếu `LEAD_DEDUP_WINDOW_MS`) | `backend/db/landings.go` — ưu tiên domain khớp chính xác, mới nhất |
   | b | `SyncSharedStaticFilesToDB()` bỏ sót cặp `(domain, page_id)` | Một số landing giữ `tracker.js` cũ | `backend/db/landings.go` — `UNION` `landing_pages` + `landing_files` |
   | c | `<base href>` đặt sau `<script src="tracker.js">` | Trình duyệt gọi `/p/tracker.js` ⇒ **404** ⇒ `window.ApexTracker` không tồn tại ⇒ lớp bảo vệ #2, #3 vô hiệu | `backend/handlers/landing_handler.go` — `rewriteRelativeAssetPaths()` ghi đè đường dẫn tài nguyên thành **tuyệt đối** |

   > ⚠️ Lớp bảo vệ #1 (fingerprint server) là lớp **duy nhất không phụ thuộc client**
   > ⇒ vẫn giữ được tín hiệu "gửi trùng do client" và không ghi lead thật vào cách ly.
   > Nhưng nếu `tracker.js` không nạp được ⇒ landing tự gửi lead cũng không chạy ⇒
   > **lead thật không tới server** ⇒ phải sửa cả 3 nguyên nhân a, b, c.

> ⚠️ **Không được phá autofill-safe** khi sửa `tracker.js` (xem mục 4).

### 🩹 4 nguyên nhân gốc gây lỗi "Proof-of-Work" (fix 2026-11-12)

| # | Nguyên nhân | Dấu hiệu | Fix đã áp dụng |
|---|---|---|---|
| 1 | **Challenge hết hạn** — khách ở trên trang > TTL rồi mới bấm gửi | `challenge_expires_at` nhỏ hơn `time_on_page` | Server TTL 5′ → **10′**; client `ensureChallengeReady()` tự lấy challenge mới + giải lại **tại thời điểm submit** nếu `expires_at <= now + 90s` |
| 2 | **Sai thuật toán băm / sai format chuỗi** | Hash không bắt đầu `"000"` dù solution đúng | Công thức **bắt buộc**: `SHA256(challenge_id + ":" + salt + ":" + solution)`, SHA-256 (không SHA-1/MD5) — phải **giống hệt** giữa `tracker.js` và `challenge.go` |
| 3 | **Replay / double-click** | `challenge_signature` bị từ chối lần gửi thứ 2 | Client `disableSubmitButtons(true)` **ngay khi click** (trước khi gọi mạng) + khoá idempotency theo SĐT; server `LoadOrStore` nonce ⇒ challenge dùng 1 lần |
| 4 | **Lệch đồng hồ hệ thống** | Challenge vừa tạo đã báo hết hạn | Server chấp nhận sai số **60 giây** (`challengeClockSkewGrace`) |

> ✅ **Đã kiểm tra server production (2026-11-11)**: `date /T` trên `WS2025` = `Thu 11/12/2026`,
> khớp với giờ local ⇒ **không** có clock skew. Nguyên nhân chính là #1 (hết hạn) và #3 (double-click).

> ⚠️ **Lưu ý thứ tự ưu tiên khi sửa PoW**: đừng đụng vào `usedNonces`/`duplicateSubmitWindow`.
> Lớp bảo vệ #1 (fingerprint server) là lớp duy nhất **không phụ thuộc client** ⇒
> vẫn giữ được tín hiệu "gửi trùng do client" mà không ghi lead thật vào vùng cách ly.

### Cấu hình

| Tham số | Giá trị | Vị trí |
|---|---|---|
| Difficulty mặc định | `3` (SHA-256 bắt đầu bằng `"000"`, client mất ~15–30ms) | `challenge.go` |
| Difficulty tối đa | `6` | `ParseDifficulty` |
| TTL challenge | **10 phút** (`challengeTTL`) — nâng từ 5 phút 2026-11-12 vì khách đọc/điền form lâu | `GenerateChallenge` |
| Dung sai lệch đồng hồ | **60 giây** (`challengeClockSkewGrace`) | `challenge.go` |
| Biên an toàn client | **90 giây** — client tự lấy challenge mới nếu `expires_at <= now + 90` | `tracker.js` → `ensureChallengeReady()` |
| TTL nonce cache | 10 phút, dọn mỗi 5 phút | `usedNonces` |
| Cửa sổ "gửi trùng do client" | 60 giây | `duplicateSubmitWindow` |
| Khoá gửi trùng phía client | 30 giây | `tracker.js` → `LEAD_DEDUP_WINDOW_MS` |
| Secret HMAC | biến môi trường `CHALLENGE_SECRET`, fallback `defaultChallengeSecretKey` | `challenge.go` |

> ✅ **Secret đã hỗ trợ biến môi trường.** Đặt `CHALLENGE_SECRET` trong `.env` (production: `C:\apex\.env`) để rotate secret mà không cần sửa code. Nếu để trống → dùng giá trị mặc định trong `backend/security/challenge.go` (`defaultChallengeSecretKey`).
>
> ⚠️ **Lưu ý:** secret mặc định vẫn nằm trong source code → ai đọc repo cũng thấy. Nên đặt `CHALLENGE_SECRET` ở mọi môi trường thật.

### Kiểm thử

```bash
# Unit test PoW (10 ca: happy path, duplicate, replay, song song, invalid, expired...)
cd backend && go test ./security/ -run "TestVerifyChallenge" -v

# E2E (cần backend chạy local + DB local)
node backend/tests/e2e_pow_duplicate_submit_test.js
```


### Hiệu quả

Script tự động (cURL, Python Requests, botnet) không có JS runtime → không giải được PoW. Người dùng thật chỉ mất vài chục ms, không nhận ra.

---

## 3. Honeypot

**File:** `backend/security/honeypot.go`

Field ẩn bằng CSS (`display:none`, `visibility:hidden`, `opacity:0`, `position:absolute; left:-9999px`) mà **mắt người thật không thấy** nhưng bot tự động điền hết.

Tên field thường giống hợp lệ: `website`, `fax_company_prefix`, `user_email_verify`, `company_website_url`...

**Bất biến:** Honeypot phải **luôn nằm trong DOM** (không dùng `display:none` bằng JS sau load) và form phải `autocomplete="off"` cho các field này.

---

## 4. Behavioral Metrics

**File:** `backend/security/` + `backend/static/js/tracker.js`

Tracker đo:
- `time_on_page` — thời gian từ load → submit
- `max_scroll` — % scroll depth
- `clicks_count` — số click
- `mouse movement` — có di chuyển chuột không
- `fill_duration_ms` — thời gian điền form

Ngưỡng tham chiếu: `time_on_page > 3 giây` là bắt buộc cho người thật.

### Điểm quan trọng: Autofill-safe

Người dùng Chrome autofill hoặc Apple Keychain điền tức thì → `fill_duration_ms` rất ngắn, không có mouse movement. Thuật toán **không** coi đây là bot nếu:

- `is_autofill === true` và `autofill_source === "browser_autofill"`
- Form có hợp lệ (đủ trường, SĐT đúng định dạng)

**Không được** siết quy tắc này — nếu chặn autofill, tỷ lệ chuyển đổi giảm mạnh.

---

## 5. Rate Limit

**File:** `backend/security/ratelimit.go`

Giới hạn theo IP, sliding window. Vượt ngưỡng → `429`.

Lead spam từ 1 IP sẽ bị chặn trước khi chạm DB — tiết kiệm tài nguyên và giữ CSDL sạch.

---

## 6. Phone Validation & Carrier Detection

**File:** `backend/security/phone_validator.go`

- Chuẩn E.164: `+84XXXXXXXXX`
- 10 số, prefix hợp lệ: `03` (Vinaphone/Mobifone/Vietnamobile), `05` (Viettel), `07` (Vietnamobile), `08` (FPT/GTEL), `09` (Viettel/Vinaphone/Wintel)
- Phát hiện nhà mạng: **Viettel, Vinaphone, Mobifone, Vietnamobile, Wintel**
- Loại bỏ prefix `+84`, `84`, `0` để normalize

---

## 7. GeoIP & Ad Placement

**File:** `backend/security/geoip.go`, `backend/security/ad_placement.go`

- GeoIP: IP → `city`, `region` (Miền Bắc/Trung/Nam/Quốc Tế), `isp`
- Ad placement parse từ UTM + referrer:
  - Facebook: `fb_feed`, `fb_stories`, `fb_reels`, `fb_audience_network`
  - Instagram: `ig_stories`, `ig_reels`, `ig_feed`
  - Google: `google_search`, `gdn`, `youtube`, `pmax`
  - TikTok, Zalo, Direct

Dữ liệu này dùng cho báo cáo "Hiệu Quả CR% Từng Vị Trí QC".

---

## 8. Auth — Argon2id & Session

**File:** `backend/security/argon2.go`, `backend/handlers/auth_handler.go`

### Argon2id parameters
| Tham số | Giá trị |
|---|---|
| Memory | 64 MB |
| Iterations | 3 |
| Parallelism | 2 |
| Salt | 16 byte ngẫu nhiên |

**Không dùng** MD5/SHA1/bcrypt cho mật khẩu.

### Session
- Token sinh ngẫu nhiên → lưu **SHA-256 hash** trong `admin_sessions.token_hash` (không lưu plaintext)
- Set cookie `apex_session` (HttpOnly) + trả token để dùng `Authorization: Bearer`
- `expires_at` dài hạn → có thể revoke từ xa qua UI "Nhật Ký & Phiên Đăng Nhập"
- Đổi mật khẩu → revoke **mọi** session khác

### Audit log
Mọi hành động admin được ghi vào `admin_audit_logs`: action, resource, details JSONB, IP, user agent, device, browser, OS.

---

## 9. Bảo vệ file tĩnh

**File:** `backend/main.go` (edge router)

- Chặn path traversal: chuỗi chứa `..` hoặc `//` → `403 Forbidden`
- Chặn file nhạy cảm: tên chứa `service-account`, `.env`, đuôi `.key`/`.sql`/`.exe` → `404`
- Chặn file ẩn: basename bắt đầu bằng `.` → `404`
- Whitelist đuôi file được phục vụ công khai (`.html .js .css .json .png .jpg .svg .webp .woff2 .mp4 .pdf`...)

---

## 9.1 Panic recovery middleware

**File:** `backend/main.go` (`recoveryMiddleware`)

Server bọc toàn bộ router bằng `recoveryMiddleware`. Mục đích: **một lỗi parse dữ liệu
(UA lạ, header thiếu) không được làm chết request** — trả `500` với JSON chuẩn và
ghi stack trace vào `server.log`.

Bối cảnh: `security.ParseDeviceInfo()` đọc User-Agent bằng regex. Trước khi có
middleware, một regex chỉ có **1 capture group** nhưng code truy cận `m[2]`
(case `Vivo Browser` / `OPPO Browser`) → `panic: index out of range [2] with length 2`
→ **mất toàn bộ event tracking của phiên đó** và log server bị ngập stack trace.

> ⚠️ **Khi thêm regex mới trong `security/device.go`:**
> 1. Đếm số capture group của regex.
> 2. Chỉ truy cận `m[N]` với `N <= số group` (điều kiện `len(m) > N` bảo đảm an toàn).
> 3. Test với UA thật của trình duyệt đó (Vivo Browser, OPPO Browser, Huawei Browser…).

---

## 10. Bảo vệ tài nguyên SSL

`GET /api/internal/check-domain?domain=X`:
- Domain đã đăng ký trong `domains` → `200 OK`
- Domain lạ → `403 Forbidden`

Caddy dùng endpoint này để quyết định có cấp chứng chỉ SSL hay không → chống lạm dụng Let's Encrypt rate-limit.

> ⚠️ **Endpoint này không có authentication.** Bất kỳ ai cũng gọi được. Nên giới hạn theo IP (`127.0.0.1` / `::1`) ở tầng Caddy hoặc firewall.

---

## 11. Cookie chuyển đổi 180 ngày

**File:** `backend/static/js/tracker.js`

Sau khi gửi form thành công → set cookie (TTL 180 ngày). Lần sau vào lại → tự redirect sang `chucmung.html`.

Mục đích: tránh tạo lead trùng, tối ưu chuyển đổi và trải nghiệm người dùng.

---

## 12. Checklist bảo mật trước khi deploy

- [ ] `service-account.json`, `.env`, `.env.prod` **không** nằm trong git
- [ ] Mật khẩu admin đã đổi khỏi `ApexAdmin@2026!`
- [ ] Mật khẩu PostgreSQL đã đổi khỏi `postgres`
- [ ] `ALLOW_ORIGINS` thu hẹp về domain thật (nếu không cần wildcard)
- [ ] Port `8080` **đã chặn** từ internet (chỉ Caddy local được truy cập)
- [ ] Port `5432` chỉ listen `127.0.0.1`
- [ ] `/api/internal/check-domain` giới hạn IP local
- [ ] `challengeSecretKey` đã chuyển sang biến môi trường `CHALLENGE_SECRET` trong `.env`
- [ ] Caddy log rotation hoạt động
- [ ] Backup PostgreSQL định kỳ

Xem `SECURITY_PLAN.md` ở root repo cho kế hoạch bảo mật đầy đủ.
