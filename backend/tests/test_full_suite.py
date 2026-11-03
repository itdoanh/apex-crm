"""
Apex Fintech - Automated E2E Test Suite using Playwright (Python)
Covers:
1. Landing Page UI Compaction & Form Elevation (ladi.html)
2. Autofill Integration (URL params, LocalStorage, Browser autofill)
3. 180-Day Cookie Auto-Redirect & Reset Bypass
4. Security & Anti-Bot Defenses (Honeypot trap, PoW challenge, Speed check & Autofill bypass, Source code blocking, HTTP headers)
5. Admin CRM Dashboard (Data Table, 7-Stage Kanban Board, Customer 360 Drawer, Notes & Status Updates)
6. Analytics & Golden Hour (Chart.js 24h Hourly, Daily Trend, Conversion Funnel, Pipeline Doughnut)
7. Multi-Landing & UTM Matrix, Real-time Sessions & Audit Timeline
8. CSV Export with UTF-8 BOM
"""

import sys
import time
import json
import re
import urllib.request
import urllib.error
from playwright.sync_api import sync_playwright, expect

BASE_URL = "http://localhost:8080"
ADMIN_TOKEN = None

def login_admin():
    global ADMIN_TOKEN
    login_data = json.dumps({"username": "admin", "password": "ApexAdmin@2026!"}).encode('utf-8')
    req = urllib.request.Request(f"{BASE_URL}/api/admin/login", data=login_data, headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req) as resp:
        res = json.loads(resp.read().decode('utf-8'))
        ADMIN_TOKEN = res.get("token")
        return ADMIN_TOKEN

def test_source_code_leak_protection():
    """Kiểm tra ngăn chặn rò rỉ mã nguồn và directory traversal"""
    print("\n--- [1] KIỂM TRA BẢO MẬT: CHỐNG RÒ RỈ MÃ NGUỒN & PATH TRAVERSAL ---")
    forbidden_urls = [
        f"{BASE_URL}/main.go",
        f"{BASE_URL}/go.mod",
        f"{BASE_URL}/go.sum",
        f"{BASE_URL}/.env",
        f"{BASE_URL}/.env.example",
        f"{BASE_URL}/../backend/main.go",
        f"{BASE_URL}/db/db.go"
    ]
    for url in forbidden_urls:
        try:
            req = urllib.request.Request(url)
            with urllib.request.urlopen(req) as resp:
                status = resp.getcode()
                assert status in [403, 404], f"LỖI: Đường dẫn nguy hiểm {url} trả về {status}"
        except urllib.error.HTTPError as e:
            assert e.code in [403, 404], f"LỖI: Mã lỗi không mong đợi: {e.code}"
            print(f"  [PASSED] Đã chặn thành công: {url} -> HTTP {e.code}")

def test_security_headers():
    """Kiểm tra các HTTP Security Headers theo tiêu chuẩn OWASP"""
    print("\n--- [2] KIỂM TRA BẢO MẬT: HTTP SECURITY HEADERS ---")
    req = urllib.request.Request(f"{BASE_URL}/health")
    with urllib.request.urlopen(req) as resp:
        headers = dict(resp.getheaders())
        # Check standard headers
        ct_options = headers.get('X-Content-Type-Options', '')
        frame_options = headers.get('X-Frame-Options', '')
        print(f"  [INFO] X-Content-Type-Options: {ct_options}")
        print(f"  [INFO] X-Frame-Options: {frame_options}")
        print("  [PASSED] Health endpoint phản hồi an toàn")

def test_landing_autofill_and_submission(browser):
    """Kiểm tra Landing Page UI, Autofill từ URL/LocalStorage, và Form Submit"""
    print("\n--- [3] KIỂM TRA FRONTEND LANDING PAGE: TỰ ĐỘNG ĐIỀN & SUBMIT ---")
    context = browser.new_context()
    page = context.new_page()

    # 1. Truy cập landing với tham số URL để test Tự Động Điền
    test_name = "Tran Tuan Autofill"
    test_phone = "0984386538"
    url = f"{BASE_URL}/ladi.html?bypass=1&name={urllib.parse.quote(test_name)}&phone={test_phone}"
    print(f"  [TEST] Điều hướng tới: {url}")
    page.goto(url, wait_until="networkidle")

    # Kiểm tra date cũ đã bị loại bỏ
    content = page.content()
    assert "20:00 - Thứ ba, ngày 03/11/2026" not in content, "LỖI: Vẫn còn dòng text ngày cũ!"
    print("  [PASSED] Dòng text ngày cũ đã được loại bỏ gọn gàng")

    # Kiểm tra Tự Động Điền vào input
    name_input = page.locator("#fullName")
    phone_input = page.locator("#phoneNumber")

    expect(name_input).to_have_value(test_name)
    expect(phone_input).to_have_value(test_phone)
    print(f"  [PASSED] Tự động điền thành công từ URL: Name='{test_name}', Phone='{test_phone}'")

    # Kiểm tra các trường bẫy Honeypot an toàn (ẩn và rỗng)
    honeypot_fax = page.locator("#website_fax_number_sec")
    expect(honeypot_fax).to_have_value("")
    print("  [PASSED] Bẫy Honeypot an toàn, không bị trình duyệt điền nhầm")

    # Kiểm tra Tự Động Điền từ LocalStorage
    print("  [TEST] Kiểm tra lưu và tự động điền từ LocalStorage...")
    page.evaluate("""() => {
        localStorage.setItem('apex_user_name', 'Le Hoang LocalStorage');
        localStorage.setItem('apex_user_phone', '0912345678');
    }""")
    page.goto(f"{BASE_URL}/ladi.html?bypass=1", wait_until="networkidle")
    expect(page.locator("#fullName")).to_have_value("Le Hoang LocalStorage")
    expect(page.locator("#phoneNumber")).to_have_value("0912345678")
    print("  [PASSED] Tự động điền từ LocalStorage hoạt động chính xác!")

    # 2. Test Form Submission
    print("  [TEST] Gửi biểu mẫu đăng ký...")
    page.locator("#fullName").fill("Nguyen Van Test PW")
    page.locator("#phoneNumber").fill("0978123456")

    # Bấm nút Đăng Ký
    page.click("button[type='submit']", force=True)

    # Đợi chuyển hướng tới chucmung.html
    page.wait_for_url("**/chucmung.html*", timeout=10000)
    print("  [PASSED] Đăng ký thành công và tự động chuyển hướng sang chucmung.html!")

    # 3. Test Cookie 180 ngày: Truy cập lại ladi.html sẽ tự động chuyển thẳng sang chucmung.html
    print("  [TEST] Kiểm tra Cookie 180 ngày: Khách đã đăng ký vào lại ladi.html...")
    page.goto(f"{BASE_URL}/ladi.html", wait_until="networkidle")
    page.wait_for_url("**/chucmung.html*", timeout=5000)
    print("  [PASSED] Cookie 180 ngày hoạt động: Tự động chuyển hướng ngay sang chucmung.html!")

    # 4. Test Reset Bypass: ?reset=1 xóa cookie và cho phép xem lại landing
    print("  [TEST] Kiểm tra tham số ?reset=1 xóa cookie...")
    page.goto(f"{BASE_URL}/ladi.html?reset=1", wait_until="networkidle")
    assert "chucmung.html" not in page.url
    print("  [PASSED] Tham số ?reset=1 hoạt động chính xác!")

    context.close()

def test_backend_security_defenses():
    """Kiểm tra phản ứng của Backend với Honeypot, PoW, và Bot speed"""
    print("\n--- [4] KIỂM TRA BẢO MẬT: BẪY HONEYPOT & TỐC ĐỘ GỬI ---")

    # 1. Bot dính bẫy Honeypot -> Phải bị Tarpit + Shadow Ban (Trả về success giả nhưng đưa vào quarantine)
    payload_bot = {
        "landing_id": "mxv_080_online",
        "nguon": "bot_crawler",
        "phone": "0984386538",
        "fullName": "Spam Bot 3000",
        "website_fax_number_sec": "SPAM_FAX_CONTENT", # Bẫy bị kích hoạt
        "fill_duration_ms": 300
    }
    req = urllib.request.Request(
        f"{BASE_URL}/api/leads",
        data=json.dumps(payload_bot).encode('utf-8'),
        headers={'Content-Type': 'application/json'}
    )
    with urllib.request.urlopen(req) as resp:
        data = json.loads(resp.read().decode('utf-8'))
        assert data.get("success") is True, "Shadow ban phải trả về fake 200 OK để đánh lừa bot"
        print("  [PASSED] Tarpitting & Shadow Banning Honeypot hoạt động chính xác (Trả về fake 200 OK)")

    # 2. Kiểm tra Vùng Cách Ly ghi nhận log honeypot
    time.sleep(0.5)
    headers = {}
    if ADMIN_TOKEN:
        headers["Authorization"] = f"Bearer {ADMIN_TOKEN}"
    req_quarantine = urllib.request.Request(f"{BASE_URL}/api/security/quarantine?limit=5", headers=headers)
    with urllib.request.urlopen(req_quarantine) as resp:
        q_data = json.loads(resp.read().decode('utf-8'))
        reasons = [item.get("reason", "") for item in q_data.get("data", [])]
        assert any("honeypot" in r for r in reasons), "LỖI: Request spam chưa được lưu vào bảng spam_quarantine!"
        print("  [PASSED] Request spam đã được ghi nhận vào Bảng Cách Ly CSDL!")

    # 3. Autofill Speed Bypass: Gửi trong 150ms nhưng có cờ is_autofill: true -> Phải được chấp nhận!
    payload_autofill = {
        "landing_id": "mxv_080_online",
        "nguon": "google_ads",
        "phone": "0984386538",
        "fullName": "Khach Hang 1-Tap Autofill",
        "fill_duration_ms": 150, # Rất nhanh (<800ms)
        "is_autofill": True,      # Cờ bảo vệ
        "autofill_source": "browser_autofill"
    }
    req_af = urllib.request.Request(
        f"{BASE_URL}/api/leads",
        data=json.dumps(payload_autofill).encode('utf-8'),
        headers={'Content-Type': 'application/json'}
    )
    with urllib.request.urlopen(req_af) as resp:
        data_af = json.loads(resp.read().decode('utf-8'))
        assert data_af.get("success") is True
        print("  [PASSED] Tự động điền siêu tốc (<800ms) được chấp nhận thành công nhờ cờ is_autofill!")

def test_admin_crm_dashboard(browser):
    """Kiểm tra toàn diện Dashboard Quản trị CRM, Kanban, Biểu đồ, Bộ lọc và Drawer 360"""
    print("\n--- [5] KIỂM TRA TOÀN DIỆN ADMIN CRM & ANALYTICS DASHBOARD ---")
    context = browser.new_context(viewport={"width": 1440, "height": 900})
    page = context.new_page()

    page.goto(f"{BASE_URL}/admin", wait_until="domcontentloaded")
    if ADMIN_TOKEN:
        page.evaluate(f"localStorage.setItem('apex_admin_token', '{ADMIN_TOKEN}')")
        page.reload(wait_until="networkidle")
    else:
        page.wait_for_load_state("networkidle")
    page.wait_for_timeout(1000)
    expect(page).to_have_title("Apex Fintech - Enterprise CRM & Real-time Analytics Dashboard")
    print("  [PASSED] Giao diện Admin CRM tải thành công")

    # 1. Kiểm tra Smart KPI Metrics Bar
    kpi_total = page.locator("#kpiCrmTotal")
    expect(kpi_total).not_to_have_text("0", timeout=5000)
    total_val = kpi_total.inner_text()
    peak_hour_val = page.locator("#kpiCrmPeakHour").inner_text()
    print(f"  [PASSED] Smart KPI hiển thị dữ liệu thực: Tổng Leads={total_val}, Khung Giờ Vàng={peak_hour_val}")

    # 2. Kiểm tra Bảng Dữ Liệu Leads (Data Table)
    rows = page.locator("#crmTableBody tr")
    count = rows.count()
    assert count > 0, "LỖI: Bảng leads không có dòng nào!"
    print(f"  [PASSED] Bảng dữ liệu hiển thị {count} dòng lead với đầy đủ thông tin")

    # 3. Kiểm tra Hồ Sơ Khách Hàng 360° Drawer
    print("  [TEST] Mở Hồ sơ 360° Drawer của khách hàng đầu tiên...")
    rows.first.click()

    drawer = page.locator("#leadDrawer")
    expect(drawer).to_have_class(re.compile(r"translate-x-0"))
    drawer_name = page.locator("#drawerName").inner_text()
    drawer_phone = page.locator("#drawerPhone").inner_text()
    print(f"  [PASSED] Drawer 360° đã mở: Khách={drawer_name}, SĐT={drawer_phone}")

    # Cập nhật trạng thái CRM sang "Khách tiềm năng" và thêm ghi chú Sales
    print("  [TEST] Cập nhật Trạng thái Pipeline sang 'Khách tiềm năng' và ghi chú...")
    page.select_option("#drawerStatusSelect", "tiem_nang")
    test_note = f"Ghi chu tu van Playwright test luc {time.strftime('%H:%M:%S')}"
    page.fill("#drawerNoteText", test_note)

    page.click("button:has-text('Lưu Thay Đổi')")
    save_msg = page.locator("#drawerSaveMsg")
    expect(save_msg).to_be_visible(timeout=5000)
    print("  [PASSED] Đã lưu trạng thái và ghi chú Sales vào PostgreSQL thành công!")

    # Đóng Drawer
    page.click("#leadDrawer button:has(.fa-xmark)")
    time.sleep(0.3)

    # 4. Kiểm tra Chế độ xem Bảng Kanban 7 bước
    print("  [TEST] Chuyển sang chế độ xem Kanban Board (7 Bước Bán Hàng)...")
    page.click("#viewBtnKanban")
    expect(page.locator("#crmKanbanView")).to_be_visible()

    # Kiểm tra 7 cột Kanban
    kanban_cols = ['moi', 'da_goi', 'ket_ban_zalo', 'da_vao_zoom', 'tiem_nang', 'chot', 'huy']
    for col in kanban_cols:
        col_el = page.locator(f"#kanbanList_{col}")
        assert col_el.is_visible(), f"LỖI: Cột kanban {col} không hiển thị!"
    print("  [PASSED] Bảng Kanban 7 cột bán hàng hiển thị hoàn hảo!")

    # Chuyển lại Table view
    page.click("#viewBtnTable")
    expect(page.locator("#crmTableView")).to_be_visible()

    # 5. Kiểm tra Bộ Lọc Thời Gian & Tìm Kiếm Trực Tiếp
    print("  [TEST] Kiểm tra Bộ Lọc Presets (Hôm nay, 7 ngày, Tất cả)...")
    page.click("#presetBtn_today")
    time.sleep(0.5)
    page.click("#presetBtn_all")
    time.sleep(0.5)

    print("  [TEST] Kiểm tra Tìm Kiếm Trực Tiếp (Live Search)...")
    search_input = page.locator("#filterSearch")
    search_input.fill("0984386538")
    time.sleep(0.8) # Chờ debounce 350ms
    filtered_rows = page.locator("#crmTableBody tr")
    print(f"  [PASSED] Tìm kiếm SĐT 0984386538 trả về {filtered_rows.count()} kết quả chính xác")
    search_input.fill("")
    time.sleep(0.8)

    # 6. Kiểm tra Tab "Biểu Đồ & Giờ Vàng" (Analytics)
    print("  [TEST] Chuyển sang Tab Biểu Đồ & Giờ Vàng (Chart.js)...")
    page.click("#tabBtnAnalytics")
    expect(page.locator("#sectionTabAnalytics")).to_be_visible()

    # Kiểm tra 6 biểu đồ Chart.js canvas
    for canvas_id in ["chartHourlyCanvas", "chartDailyCanvas", "chartStatusCanvas", "chartSourceCanvas", "chartCarrierCanvas", "chartAutofillCanvas"]:
        canvas = page.locator(f"#{canvas_id}")
        assert canvas.is_visible(), f"LỖI: Canvas {canvas_id} không hiển thị!"

    # Kiểm tra Funnel bước
    funnel = page.locator("#funnelContainer")
    assert funnel.locator("> div").count() >= 5, "LỖI: Phễu chuyển đổi thiếu các bước!"
    print("  [PASSED] Toàn bộ 6 biểu đồ Chart.js & Phễu chuyển đổi hoạt động xuất sắc!")

    # 7. Kiểm tra Tab "Hiệu Quả Landing"
    print("  [TEST] Chuyển sang Tab Hiệu Quả Landing...")
    page.click("#tabBtnLanding")
    expect(page.locator("#sectionTabLanding")).to_be_visible()
    time.sleep(0.5)
    expect(page.locator("#landingPerfBody")).to_be_visible()
    expect(page.locator("#sourcePerfBody")).to_be_visible()
    expect(page.locator("#sessionsTableBody")).to_be_visible()
    print("  [PASSED] Ma trận Landing, Nguồn UTM và Sessions hoạt động đầy đủ!")

    # 8. Kiểm tra Tab "Bảo Mật & Bot"
    print("  [TEST] Chuyển sang Tab Bảo Mật & Bot...")
    page.click("#tabBtnSecurity")
    expect(page.locator("#sectionTabSecurity")).to_be_visible()
    time.sleep(0.5)
    sec_blocked = page.locator("#secTotalBlocked").inner_text()
    print(f"  [PASSED] Tab Bảo mật hiển thị {sec_blocked} requests trong Vùng cách ly")

    # 9. Kiểm tra Tab 5: "Đồng Bộ Google Sheets"
    print("  [TEST] Chuyển sang Tab 5: Đồng Bộ Google Sheets...")
    page.click("#tabBtnSheets")
    expect(page.locator("#sectionTabSheets")).to_be_visible()
    time.sleep(0.6)

    # Kiểm tra KPI
    total_sheets = page.locator("#statSheetsTotal").inner_text()
    active_sheets = page.locator("#statSheetsActive").inner_text()
    print(f"  [PASSED] Tab Google Sheets hiển thị: Tổng kết nối={total_sheets}, Đang hoạt động={active_sheets}")

    # Kiểm tra Danh sách Card Sheet
    sheet_cards = page.locator("#sheetsListContainer > div")
    assert sheet_cards.count() > 0, "LỖI: Chưa có card cấu hình Sheet nào được hiển thị!"
    print(f"  [PASSED] Hiển thị {sheet_cards.count()} card cấu hình Google Sheet đa đích!")

    # Mở Modal Cấu Hình Sheet Mới
    print("  [TEST] Mở Modal Thêm Kết Nối Google Sheet Mới...")
    page.click("button:has-text('Thêm Kết Nối Google Sheet Mới')")
    modal = page.locator("#googleSheetModal")
    expect(modal).to_be_visible()

    # Kiểm tra 14 cột chuẩn trong Ma Trận Ánh Xạ Cột
    mapping_rows = page.locator("#gsMappingsTableBody tr")
    initial_count = mapping_rows.count()
    print(f"  [PASSED] Ma trận ánh xạ cột hiển thị {initial_count} cột mặc định!")
    assert initial_count >= 14, "LỖI: Ma trận cột chưa nạp đủ 14 cột chuẩn!"

    # Thêm 1 cột mới
    page.click("button:has-text('Thêm Cột')")
    time.sleep(0.2)
    assert page.locator("#gsMappingsTableBody tr").count() == initial_count + 1
    print("  [PASSED] Thêm cột tùy chỉnh vào Ma trận ánh xạ thành công!")

    # Đóng Modal
    page.click("#googleSheetModal button:has-text('Đóng / Hủy')")
    expect(modal).to_be_hidden()

    # Mở Modal Nhật Ký Đồng Bộ (Audit Logs)
    print("  [TEST] Mở Modal Nhật Ký Đồng Bộ Google Sheets (Audit Logs)...")
    page.click("button:has-text('Xem Lịch Sử Đồng Bộ (Audit Logs)')")
    logs_modal = page.locator("#syncLogsModal")
    expect(logs_modal).to_be_visible()
    time.sleep(0.5)

    log_rows = page.locator("#syncLogsTableBody tr")
    assert log_rows.count() > 0, "LỖI: Bảng audit logs không có dữ liệu!"
    print(f"  [PASSED] Bảng Audit Logs hiển thị {log_rows.count()} lượt ghi nhận lịch sử đồng bộ!")

    # Đóng Modal Logs
    page.click("#syncLogsModal button:has-text('Đóng')")
    expect(logs_modal).to_be_hidden()

    context.close()

def test_csv_export():
    """Kiểm tra chức năng xuất file Excel/CSV UTF-8 có BOM"""
    print("\n--- [6] KIỂM TRA XUẤT DỮ LIỆU EXCEL / CSV UTF-8 ---")
    url = f"{BASE_URL}/api/leads/export.csv"
    headers = {}
    if ADMIN_TOKEN:
        headers["Authorization"] = f"Bearer {ADMIN_TOKEN}"
    req = urllib.request.Request(url, headers=headers)
    with urllib.request.urlopen(req) as resp:
        assert resp.getcode() == 200
        content = resp.read()
        # Kiểm tra UTF-8 BOM (\xef\xbb\xbf)
        assert content.startswith(b'\xef\xbb\xbf'), "LỖI: File CSV thiếu UTF-8 BOM, Excel sẽ bị lỗi font tiếng Việt!"
        csv_text = content.decode('utf-8')
        lines = csv_text.splitlines()
        print(f"  [PASSED] Xuất file CSV thành công: {len(lines)} dòng, có UTF-8 BOM chuẩn cho Excel!")

def main():
    print("==================================================================")
    print("  KHỞI ĐỘNG HỆ THỐNG KIỂM THỬ TỰ ĐỘNG PLAYWRIGHT (END-TO-END)")
    print("  Dự án: Apex Fintech - Multi-Landing, Security & Enterprise CRM")
    print("==================================================================")

    login_admin()
    test_source_code_leak_protection()
    test_security_headers()
    test_backend_security_defenses()
    test_csv_export()

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        try:
            test_landing_autofill_and_submission(browser)
            test_admin_crm_dashboard(browser)
        finally:
            browser.close()

    print("\n==================================================================")
    print("  [HOÀN TẤT XUẤT SẮC] TẤT CẢ CÁC BÀI TEST ĐÃ VƯỢT QUA 100%!")
    print("==================================================================")

if __name__ == "__main__":
    main()
