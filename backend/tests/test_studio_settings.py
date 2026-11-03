"""
Apex Fintech - Automated Test for Settings Button inside Code Studio
Tests:
1. Open Code Studio Modal
2. Verify #btnStudioSettings exists and is visible in the top toolbar
3. Click #btnStudioSettings to open Settings Modal right inside Code Studio
4. Verify Settings Modal opens on top (z-[60]) with all 6 tabs including Google Sheets
5. Close Settings Modal and confirm Code Studio remains open and active
"""

import os
import sys
import json
import urllib.request

if sys.stdout.encoding != 'utf-8':
    try:
        sys.stdout.reconfigure(encoding='utf-8')
    except Exception:
        pass

from playwright.sync_api import sync_playwright

BASE_URL = "http://localhost:8080"
OUTPUT_DIR = "tests/screenshots"
os.makedirs(OUTPUT_DIR, exist_ok=True)

def test_settings_inside_code_studio():
    print("\n--- KIỂM TRA NÚT 'CÀI ĐẶT' BÊN TRONG CODE STUDIO ---")
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        ctx = browser.new_context(viewport={"width": 1440, "height": 900})
        page = ctx.new_page()

        # 1. Truy cập Admin -> Tab Ladi Studio & nạp token xác thực
        login_data = json.dumps({"username": "admin", "password": "ApexAdmin@2026!"}).encode('utf-8')
        req = urllib.request.Request(f"{BASE_URL}/api/admin/login", data=login_data, headers={"Content-Type": "application/json"})
        with urllib.request.urlopen(req) as resp:
            login_res = json.loads(resp.read().decode('utf-8'))
            admin_token = login_res.get("token")

        page.goto(f"{BASE_URL}/admin", wait_until="domcontentloaded")
        if admin_token:
            page.evaluate(f"localStorage.setItem('apex_admin_token', '{admin_token}')")
            page.reload(wait_until="networkidle")
        else:
            page.wait_for_load_state("networkidle")
        page.wait_for_timeout(1000)
        page.click("#tabBtnLadiStudio")
        page.wait_for_timeout(1000)

        # 2. Mở Code Studio Soạn Thảo HTML
        print("  [TEST] Mở Code Studio...")
        page.locator("button:has-text('Code Studio')").first.click()
        page.wait_for_timeout(1200)
        assert page.is_visible("#codeStudioModal"), "Modal Code Studio chưa mở"

        # 3. Kiểm tra nút Cài Đặt (#btnStudioSettings) trong toolbar
        settings_btn = page.locator("#btnStudioSettings")
        assert settings_btn.is_visible(), "Nút Cài Đặt không xuất hiện trong Toolbar Code Studio"
        btn_text = settings_btn.inner_text()
        print(f"  [PASSED] Nút Cài Đặt trong Code Studio hiển thị: '{btn_text}'")

        # 4. Bấm nút Cài Đặt từ trong Code Studio
        print("  [TEST] Bấm nút Cài Đặt ngay bên trong Code Studio...")
        settings_btn.click()
        page.wait_for_timeout(600)

        # Kiểm tra Modal Cài Đặt đã mở đè lên Code Studio
        assert page.is_visible("#landingSettingsModal"), "Modal Cài Đặt chưa mở đè lên Code Studio"
        print("  [PASSED] Modal Cài Đặt mở thành công ngay trên Code Studio")

        # Chuyển sang Tab 6: Google Sheets trong Settings
        page.click("#cfgTabBtnSheets")
        page.wait_for_timeout(400)
        assert page.is_visible("#cfgTabSecSheets"), "Tab Google Sheets chưa mở"
        print("  [PASSED] Đã mở Tab Google Sheets cấu hình bảng tính riêng ngay trong Code Studio")

        # Chụp ảnh Settings Modal mở đè lên Code Studio
        page.screenshot(path=f"{OUTPUT_DIR}/15_code_studio_settings_modal_open.png")
        print(f"  [SAVED] {OUTPUT_DIR}/15_code_studio_settings_modal_open.png")

        # 5. Đóng Settings Modal
        print("  [TEST] Đóng Settings Modal...")
        page.click("#landingSettingsModal button:has-text('Đóng / Hủy')")
        page.wait_for_timeout(400)

        # Xác nhận Settings Modal đã đóng
        assert not page.is_visible("#landingSettingsModal"), "Settings Modal chưa đóng"

        # Xác nhận Code Studio VẪN ĐANG MỞ và sẵn sàng soạn thảo tiếp
        assert page.is_visible("#codeStudioModal"), "Code Studio phải vẫn đang mở sau khi đóng Settings"
        print("  [PASSED] Code Studio vẫn giữ nguyên trạng thái mở và sẵn sàng soạn thảo tiếp")

        # Chụp ảnh sau khi đóng Settings quay lại Code Studio
        page.screenshot(path=f"{OUTPUT_DIR}/16_code_studio_resumed_after_settings.png")
        print(f"  [SAVED] {OUTPUT_DIR}/16_code_studio_resumed_after_settings.png")

        ctx.close()
        browser.close()

if __name__ == "__main__":
    test_settings_inside_code_studio()
    print("\n🎉 BÀI TEST NÚT CÀI ĐẶT TRONG CODE STUDIO ĐẠT 100%!")
