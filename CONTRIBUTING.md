# Đóng góp cho dự án Apex Fintech

Cảm ơn bạn đã đóng góp! Tài liệu này mô tả quy trình chuẩn cho **contributor là con người**.

> Nếu bạn là **AI agent**, hãy đọc `AGENTS.md` + `.cursor/rules/` thay vì file này.

---

## 1. Trước khi bắt đầu

```bash
git clone https://github.com/itdoanh/apex-crm.git
cd apex-crm
cp .env.example .env
```

Đọc theo thứ tự:
1. [`AGENTS.md`](AGENTS.md) — bản đồ repo + bất biến
2. [`docs/ONBOARDING.md`](docs/ONBOARDING.md) — setup môi trường
3. Tài liệu theo phần bạn sẽ sửa (bảng trong `AGENTS.md` mục 2)

---

## 2. Bất biến kỹ thuật

Xem [`AGENTS.md`](AGENTS.md) mục 5. Tóm tắt:

| # | Bất biến |
|---|---|
| 1 | CORS mặc định `ALLOW_ORIGINS=*` |
| 2 | SĐT Việt Nam 10 số (`03x/05x/07x/08x/09x`) |
| 3 | Mọi query ngày/giờ dùng `AT TIME ZONE 'Asia/Ho_Chi_Minh'` |
| 4 | `landing_pages` PK composite `(domain, page_id)` |
| 5 | Ghi DB → phải `Reload()` cache |
| 6 | Backend chạy với working directory `C:\apex\backend` |
| 7 | `/api/internal/check-domain` trả `200`/`403` đúng quy ước |
| 8 | `backend/static/` là mirror của `frontend/admin/` |

---

## 3. Branch naming

```bash
git checkout -b feat/ten-task
git checkout -b fix/ten-loi
git checkout -b chore/ten-viec
```

---

## 4. Commit message

```
feat(scope): mô tả ngắn bằng tiếng Việt
fix(scope): mô tả ngắn bằng tiếng Việt
chore(scope): mô tả ngắn bằng tiếng Việt
```

**Scope hợp lệ:** `studio` `landings` `domains` `security` `leads` `sheets` `admin` `deploy`

### Ví dụ
```
fix(studio): fix preview syntax error, persistent data-ve-id sync, domain deploy 400
feat(landings): add search-replace widget, live attribute sync, multi-file domain deploy
fix(security): support CHALLENGE_SECRET env var for secret rotation
docs(deploy): document Task Scheduler instead of NSSM
```

### Ràng buộc
- Một commit = một thay đổi logic.
- Viết bằng tiếng Việt, không mập mờ.
- Nêu rõ **vấn đề** khi là `fix`, không chỉ nêu **hiện tượng**.
- **Commit là công sức chung của `itdoanh` + AI agent** — kết thúc message bằng:
  ```
  Co-authored-by: itdoanh <itdoanh@users.noreply.github.com>
  Co-authored-by: Cursor <cursoragent@cursor.com>
  ```

---

## 4.1. Quy luật cho AI agent

### Đọc trước — sửa sau — đồng bộ

1. **Trước khi làm gì cũng phải đọc các file liên quan** (bảng mục 1 + `AGENTS.md` mục 2).
2. **Làm xong phải cập nhật lại và đồng bộ với các file liên quan** — `docs/`, `AGENTS.md`,
   `CLAUDE.md`, `.cursorrules`, `.cursor/rules/*.mdc`, `CONTRIBUTING.md`, `.env.example`.
   Đồng bộ tài liệu là **một phần của task**, không phải việc làm thêm cho sau.

### AI không tự test vận hành

- **Được**: check lỗi code (`go build ./...`, `go vet ./...`, parse PowerShell), khởi động
  lại service để production sẵn sàng, báo cáo trạng thái.
- **Không được** (trừ khi `itdoanh` nói rõ): submit lead thật, test luồng end-user
  (đăng nhập CRM, tạo landing, deploy domain, đổi mật khẩu), sửa/xoá dữ liệu nghiệp vụ
  chỉ để "kiểm tra xem có chạy không".

> Mỗi lần test lead là một lần ghi vào DB production + Google Sheets + KPI doanh thu.
> AI test "cho có" chỉ làm nhiễu dữ liệu thật mà không đem lại giá trị.

Báo cáo phải kết thúc bằng:
`🧪 CHƯA TEST vận hành — anh itdoanh vào kiểm tra giúp em`

### Không tự ý sửa lan sang

- Vấn đề ngoài phạm vi task → **báo cáo**, không tự sửa.
- Không xoá code/dữ liệu production mà không hỏi `itdoanh`.
- Deploy / restart production → **báo trước khi làm**.

---

## 5. Pull Request

### Trước khi mở PR

```
□ cd backend && build.bat            # build thành công
□ go vet ./...                        # không có lỗi
□ Test E2E nếu có thay đổi UI/backend
□ Không còn console.log debug
□ Output từ dữ liệu người dùng đã escape (XSS)
□ Honeypot + autofill-safe còn nguyên (nếu đụng tracker.js)
□ Ghi DB → đã gọi Reload() cache
□ frontend/admin/ đã đồng bộ sang backend/static/
□ docs liên quan đã cập nhật
□ git diff không chứa .env / service-account.json
```

### Mô tả PR

```markdown
## Vấn đề
<!-- Điều gì đang hỏng, ai bị ảnh hưởng -->

## Thay đổi
<!-- Những gì đã sửa, theo bullet -->

## Cách kiểm tra
<!-- Cách reviewer tự verify -->

## Ảnh hưởng
<!-- Ảnh hưởng tới production, DB, schema, security -->
```

---

## 6. Trước khi merge

- [ ] CI pass
- [ ] Có ít nhất 1 review approve
- [ ] Không có conflict với `main`
- [ ] Đã test trên môi trường staging/local với DB thật
- [ ] Nếu đổi schema → đã cập nhật `docs/DATABASE.md`
- [ ] Nếu đổi API → đã cập nhật `docs/API.md`

---

## 7. Deploy

Chỉ người có quyền mới deploy production. Quy trình: [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md) mục 1.

```
local → test → commit → push → git pull server → build → restart task → verify /health
```

🚨 **Không sửa file trực tiếp trên production.**

---

## 8. Báo cáo lỗi

Báo lỗi qua GitHub Issues với:

```markdown
**Mô tả:** Điều gì xảy ra
**Tái hiện:** Các bước
**Mong đợi:** Điều nên xảy ra
**Môi trường:** local / staging / production
**Trình duyệt/OS:** 
**Console log:**
**Network tab (request/response):**
```

### Nếu lỗi production → dùng runbook

[`docs/RUNBOOK.md`](docs/RUNBOOK.md) có checklist chẩn đoán theo triệu chứng.

---

## 9. Câu hỏi thường gặp

**Sửa UI ở đâu?**
→ `frontend/admin/`, rồi copy sang `backend/static/`. Xem `docs/FRONTEND.md`.

**Thêm API endpoint?**
→ 5 bước: model → store → handler → route → docs. Xem `docs/API.md`.

**Đổi schema DB?**
→ `backend/db/db.go` (`initSchema`) + `backend/models/` + mọi `Scan()` + `docs/DATABASE.md`.

**Làm sao để landing hiện thay đổi ngay?**
→ Handler ghi DB phải gọi `landingCache.Reload()` / `domainCache.Reload()`.

**Lead không vào CRM?**
→ [`docs/RUNBOOK.md`](docs/RUNBOOK.md) mục "Không nhận được lead".

**Domain mới không lên SSL?**
→ [`docs/RUNBOOK.md`](docs/RUNBOOK.md) mục "On-Demand TLS".

---

## 10. Code of Conduct

- Tôn trọng mọi người, review mang tính xây dựng.
- Trao đổi bằng tiếng Việt trong issue/PR/commit.
- Không commit dữ liệu production (lead, SĐT khách hàng) vào repo.
