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
        │    + HMAC signature + replay protection    │
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
expiresAt   := now + 5 phút
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
1. **Hết hạn?** `now > expiresAt` → fail.
2. **Chữ ký HMAC hợp lệ?** So sánh bằng `hmac.Equal` (constant-time) → fail nếu client sửa difficulty/expiresAt.
3. **Replay?** Key = `challenge_id:solution` lưu trong `sync.Map`, TTL 10 phút → đã dùng thì fail.
4. **Đúng bài toán?** `SHA256(challenge_id + ":" + salt + ":" + solution)` phải bắt đầu bằng `difficulty` chữ `"0"`.

### Cấu hình

| Tham số | Giá trị | Vị trí |
|---|---|---|
| Difficulty mặc định | `3` (SHA-256 bắt đầu bằng `"000"`, client mất ~15–30ms) | `challenge.go` |
| Difficulty tối đa | `6` | `ParseDifficulty` |
| TTL challenge | 5 phút | `GenerateChallenge` |
| TTL nonce cache | 10 phút, dọn mỗi 5 phút | `usedNonces` |
| Secret HMAC | biến môi trường `CHALLENGE_SECRET`, fallback `defaultChallengeSecretKey` | `challenge.go` |

> ✅ **Secret đã hỗ trợ biến môi trường.** Đặt `CHALLENGE_SECRET` trong `.env` (production: `C:\apex\.env`) để rotate secret mà không cần sửa code. Nếu để trống → dùng giá trị mặc định trong `backend/security/challenge.go` (`defaultChallengeSecretKey`).
>
> ⚠️ **Lưu ý:** secret mặc định vẫn nằm trong source code → ai đọc repo cũng thấy. Nên đặt `CHALLENGE_SECRET` ở mọi môi trường thật.


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
