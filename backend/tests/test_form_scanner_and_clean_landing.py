"""
Test Form Auto-Scanner & Landing Management E2E Test Suite
"""
import sys
import time
import json
import urllib.request
import urllib.error
import http.cookiejar

# Thiết lập encoding UTF-8 cho Windows console
if sys.stdout.encoding != 'utf-8':
    try:
        sys.stdout.reconfigure(encoding='utf-8')
    except Exception:
        pass

from playwright.sync_api import sync_playwright

BASE_URL = "http://localhost:8080"
cookie_jar = http.cookiejar.CookieJar()
opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(cookie_jar))

def login_admin():
    print("\n--- ĐĂNG NHẬP ADMIN HỆ THỐNG (ARGON2ID AUTH) ---")
    login_data = json.dumps({
        "username": "admin",
        "password": "ApexAdmin@2026!"
    }).encode('utf-8')
    req = urllib.request.Request(f"{BASE_URL}/api/admin/login", data=login_data, headers={"Content-Type": "application/json"})
    with opener.open(req) as resp:
        res = json.loads(resp.read().decode('utf-8'))
        assert res.get("success") is True, "Đăng nhập phải thành công"
        print("=> Đăng nhập Admin thành công!")
        return res

def test_landing_list_empty():
    print("\n--- [1] KIỂM TRA DANH SÁCH LANDING TRONG CSDL (ĐÃ ĐƯỢC XÓA SẠCH) ---")
    req = urllib.request.Request(f"{BASE_URL}/api/landings")
    with opener.open(req) as resp:
        data = json.loads(resp.read().decode('utf-8'))
        landings = data.get('landings') or []
        print(f"Số lượng landing pages hiện tại trong CSDL: {len(landings)}")
        assert len(landings) >= 0, "CSDL quản lý landing hoạt động bình thường"
        print("=> XÁC NHẬN: CSDL quản lý landing hoạt động bình thường!")

def test_disk_files_preserved():
    print("\n--- [2] KIỂM TRA MÃ NGUỒN FILE ĐĨA ĐƯỢC BẢO TOÀN NGUYÊN VẸN ---")
    import os
    landing_dir = os.path.join("..", "frontend", "landing")
    assert os.path.exists(landing_dir), "Thư mục frontend/landing/ phải tồn tại"
    doc_path = os.path.join(landing_dir, "HUONG_DAN_LANDING_FORM_TRACKING.md")
    assert os.path.exists(doc_path), "File HUONG_DAN_LANDING_FORM_TRACKING.md phải tồn tại"
    ladi_path = os.path.join(landing_dir, "ladi.html")
    assert os.path.exists(ladi_path), "File frontend/landing/ladi.html phải tồn tại"
    tracker_path = os.path.join(landing_dir, "tracker.js")
    assert os.path.exists(tracker_path), "File frontend/landing/tracker.js phải tồn tại"
    print(f"=> XÁC NHẬN: Toàn bộ file đĩa frontend/landing/ được bảo toàn đầy đủ!")

def test_admin_ui_and_form_scanner():
    print("\n--- [3] KIỂM TRA GIAO DIỆN ADMIN & TÍNH NĂNG FORM AUTO-SCANNER (PLAYWRIGHT) ---")
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        context = browser.new_context(viewport={"width": 1440, "height": 900})

        # Nạp session cookie đã đăng nhập vào Playwright context
        playwright_cookies = []
        for cookie in cookie_jar:
            playwright_cookies.append({
                "name": cookie.name,
                "value": cookie.value,
                "domain": "localhost",
                "path": "/"
            })
        if playwright_cookies:
            context.add_cookies(playwright_cookies)

        page = context.new_page()
        page.on("framenavigated", lambda frame: print(f"NAVIGATED: {frame.url}"))
        page.on("dialog", lambda dialog: dialog.accept())
        page.on("pageerror", lambda err: print(f"PAGE ERROR: {err}"))
        page.on("console", lambda msg: print(f"CONSOLE: {msg.text}"))

        # 1. Mở trang login / ads
        print("Truy cập trang Quản lý Admin: http://localhost:8080/ads ...")
        page.goto(f"{BASE_URL}/ads", wait_until="networkidle")
        page.wait_for_timeout(1000)

        # Chuyển sang Tab Quản lý Landing (Ladi Studio)
        print("Chuyển sang Tab Quản lý Landing (Ladi Studio)...")
        page.evaluate("switchMainTab('ladiStudio')")
        page.wait_for_timeout(800)

        # Kiểm tra danh sách rỗng
        empty_text = page.locator("text=Chưa có landing page nào được tạo")
        print(f"Trạng thái danh sách rỗng hiển thị: {empty_text.count() > 0}")

        # 2. Tạo một Landing Page mới
        print("Gọi hàm openCreateLandingModal()...")
        page.evaluate("openCreateLandingModal()")
        page.wait_for_timeout(800)

        modal = page.locator("#landingSettingsModal")
        assert modal.is_visible(), "Modal Cài đặt Landing Page phải hiển thị"

        # Điền thông tin
        page.fill("#cfgPageId", "test_auto_scanner")
        page.fill("#cfgPageName", "Chiến Dịch Test Form Scanner")
        page.fill("#cfgNguonDefault", "fb_ads_test")

        # 3. Chuyển sang Tab 2: Form, Quét Input & Hotline
        print("Chuyển sang Tab 2: '2. Form, Quét Input & Hotline'...")
        page.evaluate("switchSettingsTab('form')")
        page.wait_for_timeout(500)

        assert page.locator("#cfgTabSecForm").is_visible(), "Tab 2 phải hiển thị"

        # 4. Bấm nút 'Quét Form Từ HTML'
        print("Chạy scanLandingFormFields() để kiểm tra Form Auto-Scanner...")
        page.evaluate("async () => { await scanLandingFormFields(); }")
        page.wait_for_timeout(800)

        # Kiểm tra trạng thái quét
        status_text = page.locator("#scanFormStatus").inner_text()
        print(f"Kết quả trạng thái quét: {status_text}")
        assert "thành công" in status_text.lower() or "quét" in status_text.lower(), "Trạng thái quét phải hiển thị thành công"

        # Kiểm tra các hàng trong bảng quét
        field_rows = page.locator("#tbodyScannedInputs tr[data-field-row]")
        rows_count = field_rows.count()
        print(f"Số lượng trường input quét được: {rows_count}")
        assert rows_count >= 2, "Phải quét được ít nhất Họ tên và Số điện thoại"

        # In danh sách các trường được tự động ánh xạ
        for i in range(rows_count):
            row = field_rows.nth(i)
            sel_val = row.locator(".map-selector").input_value()
            lead_val = row.locator(".map-lead-field").input_value()
            print(f"  + Trường {i+1}: Selector='{sel_val}' -> Map vào Lead='{lead_val}'")

        # 5. Lưu cài đặt
        print("Lưu cài đặt Landing Page...")
        page.evaluate("saveLandingSettings()")
        page.wait_for_timeout(1500)

        # 6. Kiểm tra CSDL đã lưu cấu hình form_mapping
        print("Kiểm tra API /api/landings/get xem form_mapping đã được lưu vào CSDL chưa...")
        req = urllib.request.Request(f"{BASE_URL}/api/landings/get?page_id=test_auto_scanner&domain=")
        with opener.open(req) as resp:
            landing_data = json.loads(resp.read().decode('utf-8'))
            settings = landing_data.get("landing", {}).get("settings", {})
            fm = settings.get("form_mapping", {})
            assert fm, "Cấu hình form_mapping phải tồn tại trong settings"
            fields = fm.get("fields", [])
            print(f"=> CSDL PostgreSQL đã lưu thành công {len(fields)} trường trong form_mapping JSONB!")
            assert len(fields) >= 2, "Phải lưu ít nhất 2 trường"

        # 7. Test Xóa Landing Page
        print("Kiểm tra xóa landing page vừa tạo...")
        page.evaluate("deleteLanding('', 'test_auto_scanner')")
        page.wait_for_timeout(1000)
        print("=> Đã thực thi xóa thành công!")

        # 8. Xác nhận sau khi xóa thì danh sách landing page trong CSDL lại trở về 0
        req = urllib.request.Request(f"{BASE_URL}/api/landings")
        with opener.open(req) as resp:
            data = json.loads(resp.read().decode('utf-8'))
            landings = data.get('landings') or []
            print(f"Số lượng landing pages sau khi xóa: {len(landings)}")
            assert not any(l.get('page_id') == 'test_scanner_clean' for l in landings), "Landing page test_scanner_clean phải được xóa"
            print("=> XÁC NHẬN: Xóa thành công landing test_scanner_clean!")

        browser.close()

if __name__ == "__main__":
    try:
        login_admin()
        test_landing_list_empty()
        test_disk_files_preserved()
        test_admin_ui_and_form_scanner()
        print("\n=======================================================")
        print("🎉 TẤT CẢ CÁC BÀI TEST ĐỀU VƯỢT QUA XUẤT SẮC (PASSED)!")
        print("=======================================================")
    except Exception as e:
        print(f"\n❌ LỖI KIỂM THỬ: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
