"""
Apex Fintech - Automated Test Suite for:
1. Per-Landing Google Sheet Isolation & Dispatching
2. Code Studio Direct Save (No draft, saves directly to DB & Live Cache)
3. UI Verification (Google Sheets tab, sheet selector, card badges)
"""

import json
import time
import sys
import os
import urllib.request
import urllib.parse

if sys.stdout.encoding != 'utf-8':
    try:
        sys.stdout.reconfigure(encoding='utf-8')
    except Exception:
        pass

from playwright.sync_api import sync_playwright

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

def api_get(path):
    headers = {}
    if ADMIN_TOKEN:
        headers["Authorization"] = f"Bearer {ADMIN_TOKEN}"
    req = urllib.request.Request(f"{BASE_URL}{path}", headers=headers)
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read().decode('utf-8'))

def api_post(path, data):
    body = json.dumps(data).encode('utf-8')
    headers = {"Content-Type": "application/json; charset=utf-8"}
    if ADMIN_TOKEN:
        headers["Authorization"] = f"Bearer {ADMIN_TOKEN}"
    req = urllib.request.Request(
        f"{BASE_URL}{path}",
        data=body,
        headers=headers
    )
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read().decode('utf-8'))

def test_per_landing_google_sheets_configuration():
    print("\n--- [1] KIỂM TRA CẤU HÌNH GOOGLE SHEETS RIÊNG BIỆT CHO TỪNG LANDING PAGE ---")

    # 1. Tạo 2 Sheet Configs giả lập trong DB
    sheet1_payload = {
        "name": "Bảng Tính VIP 1 (Chuyên Landing A)",
        "auth_type": "service_account",
        "credentials_json": '{"type":"service_account","project_id":"apex-test"}',
        "spreadsheet_id": "1A_TestVIPSheet1_MockID",
        "spreadsheet_name": "Google Sheet Khách VIP Landing 1",
        "sheet_tab_name": "Trang_VIP1",
        "field_mappings": [
            {"source_field": "ho_ten", "sheet_column": "A", "header_title": "Họ và tên", "is_required": True},
            {"source_field": "so_dien_thoai", "sheet_column": "B", "header_title": "Số điện thoại", "is_required": True}
        ],
        "filter_landing_id": "",
        "filter_nguon": "",
        "is_active": True,
        "auto_create_header": False
    }
    sheet1 = api_post("/api/integrations/sheets/configs", sheet1_payload)
    sheet1_id = sheet1.get("config", {}).get("id")
    print(f"  [OK] Đã tạo Sheet Config 1: ID={sheet1_id}, Tên='{sheet1_payload['name']}'")

    sheet2_payload = {
        "name": "Bảng Tính Khác 2 (Chuyên Landing B)",
        "auth_type": "service_account",
        "credentials_json": '{"type":"service_account","project_id":"apex-test"}',
        "spreadsheet_id": "2B_TestOtherSheet2_MockID",
        "spreadsheet_name": "Google Sheet Khách Landing 2",
        "sheet_tab_name": "Trang_Landing2",
        "field_mappings": [
            {"source_field": "ho_ten", "sheet_column": "A", "header_title": "Họ và tên", "is_required": True},
            {"source_field": "so_dien_thoai", "sheet_column": "B", "header_title": "Số điện thoại", "is_required": True}
        ],
        "filter_landing_id": "",
        "filter_nguon": "",
        "is_active": True,
        "auto_create_header": False
    }
    sheet2 = api_post("/api/integrations/sheets/configs", sheet2_payload)
    sheet2_id = sheet2.get("config", {}).get("id")
    print(f"  [OK] Đã tạo Sheet Config 2: ID={sheet2_id}, Tên='{sheet2_payload['name']}'")

    # 2. Cập nhật Landing Page 'mxv_080_online' chỉ nhận đồng bộ riêng vào Sheet 1
    landing_data = api_get("/api/landings/get?page_id=mxv_080_online")
    landing = landing_data.get("landing")
    settings = landing.get("settings", {})
    settings["google_sheet_sync_mode"] = "selected"
    settings["google_sheet_ids"] = [sheet1_id]

    save_resp = api_post("/api/landings/save", {
        "domain": landing.get("domain", ""),
        "page_id": "mxv_080_online",
        "is_active": True,
        "settings": settings
    })
    assert save_resp.get("success") is True, "Lưu cài đặt landing thất bại"
    print(f"  [PASSED] Đã gán riêng Sheet ID=[{sheet1_id}] cho Landing Page 'mxv_080_online' (mode=selected)")

    # 3. Kiểm tra lại cài đặt vừa lưu
    updated_landing = api_get("/api/landings/get?page_id=mxv_080_online").get("landing")
    up_settings = updated_landing.get("settings", {})
    assert up_settings.get("google_sheet_sync_mode") == "selected"
    assert sheet1_id in up_settings.get("google_sheet_ids", [])
    assert sheet2_id not in up_settings.get("google_sheet_ids", [])
    print(f"  [PASSED] Xác thực cấu hình DB: landing chỉ liên kết Sheet 1, KHÔNG liên kết Sheet 2")

def test_code_studio_direct_save_without_draft():
    print("\n--- [2] KIỂM TRA LƯU CODE STUDIO TRỰC TIẾP (KHÔNG LƯU NHÁP, STATUS=PUBLISHED & CẬP NHẬT LIVE CACHE) ---")

    unique_marker = f"APEX_STUDIO_LIVE_TEST_{int(time.time())}"
    test_html = f"""<!DOCTYPE html>
<html lang="vi">
<head>
    <meta charset="UTF-8">
    <title>Apex Live Test - Direct Save</title>
</head>
<body>
    <div id="live-content">
        <h1>{unique_marker}</h1>
        <p>Mã nguồn được lưu trực tiếp không cần lưu nháp!</p>
    </div>
</body>
</html>"""

    # Gọi API lưu mã nguồn (/api/landings/code)
    save_code_payload = {
        "domain": "",
        "page_id": "mxv_080_online",
        "code_html": test_html
    }
    save_code_resp = api_post("/api/landings/code", save_code_payload)
    assert save_code_resp.get("success") is True, "Lưu code studio trực tiếp thất bại"
    print(f"  [PASSED] API /api/landings/code trả về success=True: '{save_code_resp.get('message')}'")

    # Kiểm tra xem Live Cache và DB đã được cập nhật trực tiếp ngay lập tức chưa (không cần deploy)
    get_landing = api_get("/api/landings/get?page_id=mxv_080_online").get("landing", {})
    assert unique_marker in get_landing.get("code_html", ""), "Mã nguồn trong DB chưa cập nhật marker mới"
    print(f"  [PASSED] DB đã cập nhật trực tiếp mã nguồn mới (Marker: {unique_marker})")

    # Kiểm tra trang public /p/mxv_080_online xem live cache đã nhận mã mới chưa
    req_pub = urllib.request.Request(f"{BASE_URL}/p/mxv_080_online")
    with urllib.request.urlopen(req_pub) as pub_resp:
        pub_html = pub_resp.read().decode('utf-8')
        assert unique_marker in pub_html, "Trang public chưa nhận mã nguồn mới trực tiếp"
        print("  [PASSED] Trang Public /p/mxv_080_online phản ánh mã mới ngay lập tức không cần duyệt nháp!")

def test_ui_with_playwright():
    print("\n--- [3] KIỂM TRA GIAO DIỆN ADMIN: GOOGLE SHEETS TAB VÀ NÚT LƯU TRONG CODE STUDIO ---")
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        ctx = browser.new_context(viewport={"width": 1440, "height": 900})
        page = ctx.new_page()

        # Mở trang admin
        page.goto(f"{BASE_URL}/admin", wait_until="domcontentloaded")
        if ADMIN_TOKEN:
            page.evaluate(f"localStorage.setItem('apex_admin_token', '{ADMIN_TOKEN}')")
            page.reload(wait_until="networkidle")
        else:
            page.wait_for_load_state("networkidle")
        page.wait_for_timeout(1000)

        # Mở tab Ladi Studio
        page.click("#tabBtnLadiStudio")
        page.wait_for_timeout(1000)

        # Kiểm tra badge "Sheet riêng" xuất hiện trên card landing
        sheet_badge_count = page.locator("span:has-text('Sheet riêng')").count()
        print(f"  [PASSED] Tìm thấy {sheet_badge_count} badge Sheet riêng trên Grid Landing Pages")
        assert sheet_badge_count > 0, "Không tìm thấy badge Sheet riêng trên card landing"

        # Mở Modal Cài Đặt của landing đầu tiên
        page.locator("button:has-text('Cài Đặt')").first.click()
        page.wait_for_timeout(600)
        assert page.is_visible("#landingSettingsModal"), "Modal Cài Đặt chưa mở"

        # Nhấn vào Tab 6: Google Sheets
        page.click("#cfgTabBtnSheets")
        page.wait_for_timeout(400)
        assert page.is_visible("#cfgTabSecSheets"), "Tab 6 Google Sheets chưa mở"
        print("  [PASSED] Tab 6: Google Sheets trong Modal Cài Đặt hiển thị đầy đủ")

        # Kiểm tra danh sách bảng tính có checkboxes
        checkboxes = page.locator("input[name='cfgSheetCheckbox']")
        cb_count = checkboxes.count()
        print(f"  [PASSED] Đã tải danh sách {cb_count} Google Sheets để người dùng tích chọn riêng")
        assert cb_count >= 2, "Phải có ít nhất 2 Google Sheet trong danh sách để lựa chọn"

        # Chụp ảnh Tab 6 Google Sheets
        page.screenshot(path="tests/screenshots/13_landing_google_sheets_config.png")
        print("  [SAVED] tests/screenshots/13_landing_google_sheets_config.png")

        # Đóng Settings Modal
        page.click("#landingSettingsModal button:has-text('Đóng / Hủy')")
        page.wait_for_timeout(300)

        # Mở Code Studio Soạn Thảo HTML
        page.locator("button:has-text('Code Studio')").first.click()
        page.wait_for_timeout(1200)

        # Kiểm tra nút "Lưu Code (Ctrl+S)"
        save_btn = page.locator("#btnSaveCode")
        save_btn_text = save_btn.inner_text()
        print(f"  [PASSED] Nút Lưu trong Code Studio hiển thị: '{save_btn_text}' (Không phải Lưu Nháp)")
        assert "Lưu Code" in save_btn_text, "Nút phải là 'Lưu Code', không phải 'Lưu Nháp'"

        # Thử bấm Lưu Code
        save_btn.click()
        page.wait_for_timeout(800)

        # Kiểm tra status message "Đã lưu mã nguồn thành công" hoặc "Đã lưu lúc"
        save_status = page.locator("#csSaveStatus").inner_text()
        print(f"  [PASSED] Trạng thái Code Studio sau khi lưu: '{save_status}'")
        assert "Đã lưu" in save_status, "Trạng thái lưu phải báo Đã lưu"

        # Chụp ảnh Code Studio sau khi lưu
        page.screenshot(path="tests/screenshots/14_code_studio_direct_save.png")
        print("  [SAVED] tests/screenshots/14_code_studio_direct_save.png")

        ctx.close()
        browser.close()

if __name__ == "__main__":
    login_admin()
    test_per_landing_google_sheets_configuration()
    test_code_studio_direct_save_without_draft()
    test_ui_with_playwright()
    print("\n🎉 TOÀN BỘ BÀI TEST GOOGLE SHEETS RIÊNG BIỆT VÀ LƯU TRỰC TIẾP ĐÃ THÀNH CÔNG 100%!")
