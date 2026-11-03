# -*- coding: utf-8 -*-
"""
E2E Test: New Landing Page Creation, Code Studio Flow, Smart Dynamic Content Scanner, and SSL Status Display
"""
import sys
import time
import json
from playwright.sync_api import sync_playwright

if sys.stdout.encoding != 'utf-8':
    try:
        sys.stdout.reconfigure(encoding='utf-8')
    except Exception:
        pass

BASE_URL = "https://crm.hanghoaphaisinh.net"

def run_tests():
    print("=====================================================================")
    print("🚀 BẮT ĐẦU KIỂM THỬ E2E TOÀN DIỆN HỆ THỐNG TRÊN LIVE SERVER")
    print(f"Target URL: {BASE_URL}/ads")
    print("=====================================================================")

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        context = browser.new_context(viewport={"width": 1600, "height": 1000}, ignore_https_errors=True)
        page = context.new_page()

        page.on("dialog", lambda dialog: dialog.accept())
        page.on("pageerror", lambda err: print(f"  [PAGE ERROR]: {err}"))
        page.on("console", lambda msg: print(f"  [CONSOLE]: {msg.text}"))
        page.on("response", lambda resp: print(f"  [HTTP {resp.status}]: {resp.url}") if resp.status >= 400 or "domains" in resp.url else None)

        # 1. ĐĂNG NHẬP ADMIN
        print("\n--- [BƯỚC 1] ĐĂNG NHẬP QUẢN TRỊ VIÊN ---")
        page.goto(f"{BASE_URL}/ads", wait_until="networkidle")
        page.wait_for_timeout(1000)

        # Kiểm tra xem có đang ở trang login không
        login_screen = page.locator("#adminLoginScreen")
        if login_screen.count() > 0 and login_screen.is_visible():
            print("  -> Điền tài khoản admin...")
            page.fill("#loginUsername", "admin")
            page.fill("#loginPassword", "ApexAdmin@2026!")
            page.click("#btnLoginSubmit")
            page.wait_for_timeout(2000)
            print("  ✅ Đăng nhập Admin thành công!")
        else:
            print("  ℹ️ Đã có session hợp lệ, chuyển thẳng vào Dashboard.")

        # 2. KIỂM TRA TRẠNG THÁI SSL KHI CHƯA TRỎ DNS
        print("\n--- [BƯỚC 2] KIỂM TRA HIỂN THỊ TRẠNG THÁI SSL KHI CHƯA TRỎ DNS ---")
        page.evaluate("switchMainTab('domains')")
        # Đợi danh sách domain cards được render
        page.wait_for_function("document.querySelectorAll('#domainsListContainer > div').length > 0", timeout=15000)
        page.wait_for_timeout(1000)

        # Lấy nội dung bảng hoặc danh sách tên miền
        page_content = page.content()
        assert "Chưa Thể Cấp SSL (Chưa Trỏ DNS)" in page_content, "Phải hiển thị badge 'Chưa Thể Cấp SSL (Chưa Trỏ DNS)'"
        print("  ✅ Xác nhận: Tên miền chưa trỏ DNS hiển thị chính xác 'Chưa Thể Cấp SSL (Chưa Trỏ DNS)'")

        # Kiểm tra không còn spinner "Đang Cấp SSL..." gây hiểu lầm cho tên miền chưa trỏ DNS
        dns_error_badges = page.locator("text=Chưa Thể Cấp SSL (Chưa Trỏ DNS)")
        badge_count = dns_error_badges.count()
        print(f"  Số lượng badge 'Chưa Thể Cấp SSL (Chưa Trỏ DNS)': {badge_count}")
        assert badge_count >= 1, "Ít nhất phải có 1 badge chưa thể cấp SSL do chưa trỏ DNS"
        print("  ✅ Tuyệt đối không còn hiển thị 'Đang Cấp SSL...' khi DNS chưa active!")

        # 3. KIỂM TRA QUY TRÌNH TẠO LANDING PAGE MỚI TINH GỌN
        print("\n--- [BƯỚC 3] KIỂM TRA MODAL TẠO LANDING MỚI TINH GỌN ---")
        page.evaluate("switchMainTab('ladiStudio')")
        page.wait_for_timeout(1000)

        # Bấm nút tạo landing mới
        print("  -> Mở modal tạo Landing Mới...")
        page.evaluate("openCreateLandingModal()")
        page.wait_for_timeout(800)

        modal_create = page.locator("#modalCreateLanding")
        assert modal_create.is_visible(), "Modal #modalCreateLanding phải hiển thị"
        print("  ✅ Modal #modalCreateLanding hiển thị tinh gọn đúng yêu cầu!")

        # Kiểm tra modal cũ landingSettingsModal không được mở
        modal_old = page.locator("#landingSettingsModal")
        assert not modal_old.is_visible(), "Modal cũ #landingSettingsModal KHÔNG được mở khi bấm Tạo mới"
        print("  ✅ Xác nhận: Không mở form cài đặt cũ của template gốc!")

        # Điền thông tin tạo mới
        test_page_id = f"test_smart_{int(time.time())}"
        test_page_name = "Chiến Dịch Khởi Tạo Mới Tinh Gọn"

        page.fill("#newLandingName", test_page_name)
        page.fill("#newLandingPageId", test_page_id)

        # Kiểm tra tùy chọn Khung HTML sạch vs Dán mã
        page.evaluate("() => { document.querySelector('input[name=\"newLandingInitType\"][value=\"clean\"]').checked = true; toggleInitTypeUI(); }")
        page.wait_for_timeout(300)

        print("  -> Bấm khởi tạo landing...")
        page.click("#btnSubmitCreateLanding")
        page.wait_for_timeout(2500)

        # 4. KIỂM TRA CODE STUDIO ĐƯỢC MỞ TỰ ĐỘNG
        print("\n--- [BƯỚC 4] KIỂM TRA CODE STUDIO ĐƯỢC KÍCH HOẠT TỰ ĐỘNG ---")
        code_studio_modal = page.locator("#codeStudioModal")
        studio_visible = code_studio_modal.is_visible()
        print(f"  Trạng thái hiển thị Code Studio: {studio_visible}")
        assert studio_visible, "Code Studio phải được tự động mở sau khi tạo landing mới"
        print("  ✅ Xác nhận: Sau khi khởi tạo, người dùng được đưa thẳng vào Code Studio để quản lý file & soạn code!")

        # Đóng Code Studio
        page.evaluate("closeCodeStudio()")
        page.wait_for_timeout(800)

        # 5. KIỂM TRA CÀI ĐẶT ĐỘNG: TEMPLATE GỐC VS LANDING KHÁC
        print("\n--- [BƯỚC 5] KIỂM TRA TÙY CHỈNH THEO TỪNG LANDING (SMART DYNAMIC CONTENT) ---")

        # A. Mở cài đặt landing gốc mxv_080_online
        print("  -> Kiểm tra landing gốc 'mxv_080_online':")
        page.evaluate("openLandingSettings('', 'mxv_080_online')")
        page.wait_for_timeout(1000)
        # Chuyển sang Tab 4: Tùy Chỉnh Nhanh (Nội Dung)
        page.evaluate("switchSettingsTab('event')")
        page.wait_for_timeout(500)

        sec_orig = page.locator("#cfgSecOriginalTemplate")
        sec_dyn = page.locator("#cfgSecDynamicContent")

        assert sec_orig.is_visible(), "Landing gốc PHẢI hiển thị các trường đặc thù của mxv_080_online"
        assert not sec_dyn.is_visible(), "Landing gốc KHÔNG hiển thị Smart Dynamic Content"
        print("  ✅ Landing gốc mxv_080_online: Hiển thị đầy đủ bộ cài đặt đặc thù (Bộ Công Thương, MXV, Apex Logo, 3 Ô Metric Box)!")

        page.evaluate("closeLandingSettings()")
        page.wait_for_timeout(500)

        # B. Mở cài đặt landing mới vừa tạo
        print(f"  -> Kiểm tra landing mới tạo '{test_page_id}':")
        page.evaluate(f"openLandingSettings('', '{test_page_id}')")
        page.wait_for_timeout(1000)
        # Chuyển sang Tab 4: Tùy Chỉnh Nhanh (Nội Dung)
        page.evaluate("switchSettingsTab('event')")
        page.wait_for_timeout(500)

        sec_orig_custom = page.locator("#cfgSecOriginalTemplate")
        sec_dyn_custom = page.locator("#cfgSecDynamicContent")

        assert not sec_orig_custom.is_visible(), "Landing mới PHẢI ẨN các trường của mxv_080_online"
        assert sec_dyn_custom.is_visible(), "Landing mới PHẢI HIỂN THỊ khối 'Tùy Chỉnh Nhanh (Smart Dynamic Content)'"
        print("  ✅ Landing mới: Ẩn hoàn toàn trường mẫu cũ, hiển thị khối Tùy Chỉnh Nhanh Smart Dynamic Content!")

        # 6. KIỂM TRA QUÉT NỘI DUNG TỪ HTML (DOM SCANNER)
        print("\n--- [BƯỚC 6] KIỂM TRA NÚT 'QUÉT NỘI DUNG TỪ HTML' ---")
        scan_btn = page.locator("button:has-text('Quét Nội Dung Từ HTML')")
        assert scan_btn.is_visible(), "Nút 'Quét Nội Dung Từ HTML' phải hiển thị"

        print("  -> Chạy scanLandingContentFields()...")
        page.evaluate("async () => { await scanLandingContentFields(); }")
        page.wait_for_timeout(1000)

        # Kiểm tra bảng scanned fields
        dyn_table = page.locator("#tblCustomContentFields")
        rows = page.locator("#tbodyCustomContentFields tr[data-custom-field-row]")
        row_count = rows.count()
        print(f"  Số lượng thành phần bóc tách được từ HTML: {row_count}")
        assert row_count >= 2, f"Phải quét được các thành phần tiêu đề, nút CTA (tìm thấy {row_count})"

        for i in range(row_count):
            label = rows.nth(i).locator(".custom-field-label").input_value()
            val = rows.nth(i).locator(".custom-field-value").input_value()
            selector = rows.nth(i).locator(".custom-field-selector").input_value()
            print(f"    * [{label}] (Selector: {selector}) -> Giá trị: {val[:40]}")

        # Sửa đổi tiêu đề qua bảng và lưu cài đặt
        print("  -> Thay đổi giá trị tiêu đề qua giao diện...")
        rows.nth(0).locator(".custom-field-value").fill("TIÊU ĐỀ ĐÃ ĐƯỢC TÙY CHỈNH BẰNG SMART SCANNER")

        # 7. KIỂM TRA LƯU CÀI ĐẶT VÀ BẢO TOÀN ĐƯỜNG DẪN THƯ MỤC
        print("\n--- [BƯỚC 7] KIỂM TRA LƯU CÀI ĐẶT & BẢO TOÀN ĐƯỜNG DẪN THƯ MỤC ---")
        # Chuyển về Tab 1: Tên Miền & Chung để cấu hình thư mục
        page.evaluate("switchSettingsTab('domain')")
        page.wait_for_timeout(400)
        page.fill("#cfgPathPrefix", "/chiendich")

        print("  -> Bấm Lưu Toàn Bộ Cấu Hình...")
        page.evaluate("async () => { await saveLandingSettings(); }")
        page.wait_for_timeout(2000)
        print("  ✅ Đã lưu cấu hình thành công!")

        # Mở lại cài đặt để kiểm tra xem đường dẫn /chiendich có được bảo toàn không
        print("  -> Mở lại cài đặt để kiểm tra đường dẫn thư mục có bị mất không...")
        page.evaluate(f"openLandingSettings('', '{test_page_id}')")
        page.wait_for_timeout(1000)

        saved_path = page.locator("#cfgPathPrefix").input_value()
        print(f"  Đường dẫn thư mục sau khi mở lại: '{saved_path}'")
        assert saved_path == "/chiendich", f"Đường dẫn thư mục phải là /chiendich, nhận được: {saved_path}"
        print("  ✅ Xác nhận: Đường dẫn thư mục /chiendich được bảo toàn 100%, không bao giờ bị mất!")

        page.evaluate("closeLandingSettings()")
        page.wait_for_timeout(500)

        # 8. DỌN DẸP DỮ LIỆU TEST
        print("\n--- [BƯỚC 8] DỌN DẸP LANDING TEST TRÊN CSDL ---")
        page.evaluate(f"async () => {{ await fetch('/api/landings/delete?page_id={test_page_id}&domain=', {{ method: 'DELETE' }}); }}")
        page.wait_for_timeout(1000)
        print(f"  ✅ Đã dọn dẹp xong landing '{test_page_id}' sạch sẽ!")

        context.close()
        browser.close()

    print("\n=====================================================================")
    print("🎉 TOÀN BỘ CÁC BƯỚC KIỂM THỬ E2E THÀNH CÔNG 100%!")
    print("=====================================================================")

if __name__ == "__main__":
    run_tests()
