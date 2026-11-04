---
name: manage-domains-and-landings
description: Quản lý tên miền, định tuyến path, và landing page trong dự án Apex Fintech — deploy route, DNS/SSL check, cache reload, feature flags. Dùng khi người dùng yêu cầu thêm domain, deploy landing lên domain, kiểm tra SSL, hoặc cấu hình định tuyến multi-domain.
---

# Quản lý Domain & Landing — Apex Fintech

---

## Mô hình dữ liệu

```
domains (1) ──< (N) domain_routes ──> landing_pages
   domain              path_prefix         page_id
```

- `landing_pages` PK = **`(domain, page_id)`** — composite, không phải `page_id` đơn lẻ.
- `domain_routes` UNIQUE `(domain_id, path_prefix)`.
- Lookup runtime = **Longest Prefix Match**: `/landing` ưu tiên hơn `/`, `/` luôn ở cuối.

---

## Thêm domain mới

### Bước 1 — DNS (ngoài hệ thống)
```
A  domain.com  → 103.124.95.232
```
hoặc đổi NS sang `ns1.hanghoaphaisinh.net` / `ns2.hanghoaphaisinh.net`.

### Bước 2 — Đăng ký trong DB

Qua UI CRM (`/ads` → Quản Lý Tên Miền) hoặc SQL:

```sql
INSERT INTO domains (domain, ns1, ns2, target_ip, dns_status, ssl_status, default_landing_id)
VALUES ('domain.com', 'ns1.hanghoaphaisinh.net', 'ns2.hanghoaphaisinh.net',
        '103.124.95.232', 'pending', 'active', 'mxv_080_online')
RETURNING id;
```

### Bước 3 — Deploy route

```sql
INSERT INTO domain_routes (domain_id, path_prefix, landing_id, is_active)
VALUES (<domain_id>, '/', 'mxv_080_online', TRUE);
```

### Bước 4 — ⚠️ BẮT BUỘC reload cache

Nếu sửa trực tiếp bằng SQL → **restart backend**:
```cmd
schtasks /End /TN "Apex_Backend_Server" && schtasks /Run /TN "Apex_Backend_Server"
```

Nếu qua UI CRM / API handler → handler đã gọi `domainCache.Reload()`.

### Bước 5 — Kiểm tra SSL tự cấp

```cmd
curl "http://127.0.0.1:8080/api/internal/check-domain?domain=domain.com"
```
→ `200` = đã đăng ký. Truy cập `https://domain.com` → Caddy cấp SSL trong ~2s.

---

## Kiểm tra DNS

```powershell
Resolve-DnsName domain.com -Type A
Resolve-DnsName ns1.hanghoaphaisinh.net
```

| Kết quả | Nghĩa là |
|---|---|
| `103.124.95.232` | ✅ OK |
| IP khác | ❌ DNS chưa trỏ đúng — chưa đụng server được |
| `nxdomain` | ❌ Domain chưa có trong hệ thống DNS |

---

## Kiểm tra SSL

```cmd
powershell -NoProfile -Command "Get-ChildItem 'C:\Users\Administrator\AppData\Roaming\Caddy\acme' -Recurse -Filter '*.json' | Select-Object Name"
```

Log Caddy:
```cmd
powershell -NoProfile -Command "Select-String -Path C:\caddy\logs\caddy.log -Pattern 'on-demand' | Select-Object -Last 10"
```

| Lỗi | Nguyên nhân | Xử lý |
|---|---|---|
| `failed to get permission for on-demand certificate` | Domain chưa đăng ký | Đăng ký trong CRM → backend trả 403 |
| `ask endpoint unreachable` | Backend chết | Restart `Apex_Backend_Server` |
| `no such host` | DNS sai | Sửa A record |
| Certificate cũ | Chưa renew | Truy cập domain → Caddy renew |

---

## Deploy landing lên domain/path

### Qua UI CRM
`/ads` → **Quản Lý Tên Miền** → **Triển Khai Landing Page Lên Tên Miền**
→ chọn domain + path prefix + landing → xác nhận.

### Qua API
```http
POST /api/domains/deploy
Authorization: Bearer <token>
Content-Type: application/json

{ "domain_id": 3, "landing_id": "landing_cu", "path_prefix": "/landing", "redirect_url": "" }
```

### Lưu ý
- Một `(domain_id, path_prefix)` chỉ có **1** route (UNIQUE constraint) → deploy lại sẽ UPDATE.
- `redirect_url` khác rỗng → server trả `302` thay vì render landing.
- Sau khi deploy → `domainCache.Reload()` đã được handler gọi.

---

## Feature flags (`is_active`)

```sql
UPDATE landing_pages SET is_active = FALSE, redirect_url = 'chucmung.html'
WHERE page_id = 'mxv_080_online' AND domain = '';
```

- `is_active = TRUE` → render landing.
- `is_active = FALSE` → trả trang bảo trì, hoặc `302` nếu có `redirect_url`.
- Sửa trực tiếp bằng SQL → **restart backend** (cache).

---

## Xoá domain / route

```sql
-- Xoá route (domain giữ nguyên)
DELETE FROM domain_routes WHERE domain_id = 3 AND path_prefix = '/old';

-- Xoá domain (cascade xoá routes)
DELETE FROM domains WHERE domain = 'domain.com';
```

⚠️ Xoá domain → Caddy sẽ **không cấp được SSL mới** cho domain đó nữa (check-domain trả 403).

---

## Kiểm tra chẩn đoán

```sql
-- Toàn bộ route
SELECT d.domain, r.path_prefix, r.landing_id, r.is_active, r.redirect_url
FROM domain_routes r JOIN domains d ON d.id = r.domain_id ORDER BY d.domain, r.path_prefix;

-- Landing theo domain
SELECT page_id, domain, name, is_active, status, nguon_default, updated_at
FROM landing_pages ORDER BY updated_at DESC;

-- Trùng page_id (⚠️ gây rối cache)
SELECT page_id, COUNT(*), array_agg(domain) FROM landing_pages GROUP BY page_id HAVING COUNT(*) > 1;
```

### Test routing
```cmd
curl -I -H "Host: ads.giaodichhanghoatuonglai.com" http://127.0.0.1:8080/landing
curl -I -H "Host: domain-moi.com" http://127.0.0.1:8080/
```

---

## Checklist deploy landing mới

```
□ DNS trỏ 103.124.95.232
□ Domain có trong bảng domains
□ domain_routes có (domain, path_prefix) → landing_id
□ domainCache.Reload() đã chạy (restart nếu sửa SQL)
□ /api/internal/check-domain?domain=X → 200
□ https://domain.com/path → 200, render đúng landing
□ SSL hợp lệ (không cảnh báo)
□ Form submit → lead vào CRM
□ Landing ID trong lead khớp page_id
□ Test mobile viewport
```

---

## Tài liệu liên quan

- `docs/DATABASE.md` — schema `domains`, `domain_routes`, `landing_pages`
- `docs/ARCHITECTURE.md` — edge router, Longest Prefix Match
- `docs/DEPLOYMENT.md` — DNS, Caddy On-Demand TLS
- `docs/LANDING.md` — vòng đời landing
