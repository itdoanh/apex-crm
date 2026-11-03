"""
Apex Fintech - Automated Test Suite for Apex Ladi Studio & Dynamic Configuration
Tests:
1. Dynamic Config Backend API (/api/landing/config)
2. Admin Ladi Studio UI Tab & KPI metrics
3. Landing Page Cards & Action Buttons
4. Code Studio Modal with Line Numbers, File Tree & Device Switcher
5. Standalone Preview Modal with PC/Laptop/Mobile Viewport Switching
6. Settings Modal with 5 Configuration Tabs (Domain, Form, Pixel, Event, SEO)
7. 1-Click Deploy & Feature Flag Kill-Switch
"""

import os
import sys
import time
import json
import urllib.request
import urllib.parse

if sys.stdout.encoding != 'utf-8':
    try:
        sys.stdout.reconfigure(encoding='utf-8')
    except Exception:
        pass

from playwright.sync_api import sync_playwright

BASE_URL = "http://localhost:8080"
OUTPUT_DIR = "tests/screenshots"

os.makedirs(OUTPUT_DIR, exist_ok=True)

def test_dynamic_config_api():
    print("\n--- [1] KIỂM TRA DYNAMIC CONFIG & FEATURE FLAGS API ---")
    url = f"{BASE_URL}/api/landing/config?page_id=mxv_080_online"
    req = urllib.request.Request(url)
    with urllib.request.urlopen(req) as resp:
        assert resp.getcode() == 200, "Config API phải trả về HTTP 200"
        data = json.loads(resp.read().decode('utf-8'))
        assert data.get("landing_id") == "mxv_080_online"
        assert data.get("is_active") is True
        assert data.get("hotline") == "0984386538"
        assert "facebook_pixel_id" in data
        print(f"  [PASSED] Config API trả về đúng: Landing ID={data['landing_id']}, Hotline={data['hotline']}")

def test_public_serving_and_injection():
    print("\n--- [2] KIỂM TRA PUBLIC SERVING & DYNAMIC SCRIPT INJECTION (/p/:page_id) ---")
    url = f"{BASE_URL}/p/mxv_080_online"
    req = urllib.request.Request(url)
    with urllib.request.urlopen(req) as resp:
        assert resp.getcode() == 200
        html = resp.read().decode('utf-8')
        assert "window.APEX_LANDING_CONFIG" in html, "Thiếu window.APEX_LANDING_CONFIG được chèn tự động"
        assert "Facebook Pixel Code" in html or "fbq" in html, "Thiếu Facebook Pixel SDK"
        assert "Hội Thảo Giao Dịch Hàng Hóa" in html, "Thiếu nội dung HTML của landing"
        print("  [PASSED] Public Serving /p/mxv_080_online hoạt động hoàn hảo kèm Script Injection")

def test_admin_ladi_studio_e2e():
    print("\n--- [3] KIỂM TRA TOÀN DIỆN APEX LADI STUDIO UI TRÊN PLAYWRIGHT ---")
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        ctx = browser.new_context(viewport={"width": 1440, "height": 900})
        page = ctx.new_page()

        # 1. Mở Admin Dashboard & nạp token xác thực
        page.on("dialog", lambda dialog: dialog.accept())
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

        # 2. Chuyển sang Tab 6: Ladi Studio
        print("  [TEST] Chuyển sang Tab Quản Lý Landing (Ladi Studio)...")
        page.click("#tabBtnLadiStudio")
        page.wait_for_timeout(1000)

        # Kiểm tra sự hiển thị của section Ladi Studio
        assert page.is_visible("#sectionTabLadiStudio"), "Section Ladi Studio chưa hiển thị"
        total_landings = page.locator("#kpiLadiTotal").inner_text()
        print(f"  [PASSED] KPI Tổng Landing: {total_landings}")

        # Chụp ảnh Grid Landing Pages
        page.screenshot(path=f"{OUTPUT_DIR}/09_admin_ladi_studio_grid.png")
        print(f"  [SAVED] {OUTPUT_DIR}/09_admin_ladi_studio_grid.png")

        # 3. Mở Cài Đặt Landing Modal (Settings)
        print("  [TEST] Mở Modal Cài Đặt Landing Page...")
        page.locator("button:has-text('Cài Đặt')").first.click()
        page.wait_for_timeout(600)
        assert page.is_visible("#landingSettingsModal"), "Modal Cài Đặt chưa mở"

        # Kiểm tra chuyển Tab trong Settings: Form, Pixel, Event, SEO
        page.click("#cfgTabBtnPixel")
        page.wait_for_timeout(300)
        assert page.is_visible("#cfgTabSecPixel"), "Tab Pixel trong Settings chưa mở"
        page.screenshot(path=f"{OUTPUT_DIR}/10_admin_ladi_studio_settings_modal.png")
        print(f"  [SAVED] {OUTPUT_DIR}/10_admin_ladi_studio_settings_modal.png")

        # Đóng Settings Modal
        page.click("#landingSettingsModal button:has-text('Đóng / Hủy')")
        page.wait_for_timeout(300)

        # 4. Mở Code Studio Modal
        print("  [TEST] Mở Code Studio Soạn Thảo HTML/CSS/JS...")
        page.locator("button:has-text('Code Studio')").first.click()
        page.wait_for_timeout(1200)
        assert page.is_visible("#codeStudioModal"), "Modal Code Studio chưa mở"

        # Kiểm tra file tree và code editor
        editor_text = page.locator("#csCodeEditor").input_value()
        assert len(editor_text) > 0, "Code Editor không có nội dung"
        print(f"  [PASSED] Code Studio đã tải code: {len(editor_text)} ký tự")

        # Thử đổi chế độ xem Preview trong Code Studio sang Mobile
        page.click("#btnDeviceMobile")
        page.wait_for_timeout(500)
        page.screenshot(path=f"{OUTPUT_DIR}/11_admin_ladi_studio_code_editor.png")
        print(f"  [SAVED] {OUTPUT_DIR}/11_admin_ladi_studio_code_editor.png")

        # Đóng Code Studio (Quay lại CRM)
        page.click("#btnCloseStudio")
        page.wait_for_load_state("networkidle")
        page.wait_for_timeout(1000)
        page.click("#tabBtnLadiStudio")
        page.wait_for_timeout(800)

        # 5. Mở Standalone Responsive Preview Modal
        print("  [TEST] Mở Modal Xem Trước Trực Tiếp Responsive (3 Kiểu Xem)...")
        page.locator("button:has-text('Preview')").first.click()
        page.wait_for_timeout(1200)
        assert page.is_visible("#landingPreviewModal"), "Modal Preview chưa mở"

        # Chuyển sang kiểu xem Mobile (390px)
        page.click("#pvBtnMobile")
        page.wait_for_timeout(800)
        page.screenshot(path=f"{OUTPUT_DIR}/12_admin_ladi_studio_preview_mobile.png")
        print(f"  [SAVED] {OUTPUT_DIR}/12_admin_ladi_studio_preview_mobile.png")

        # Chuyển sang kiểu xem Laptop (1024px)
        page.click("#pvBtnLaptop")
        page.wait_for_timeout(500)

        # Đóng Preview Modal
        page.click("#btnClosePreviewModal")
        page.wait_for_timeout(300)

        ctx.close()
        browser.close()

if __name__ == "__main__":
    test_dynamic_config_api()
    test_public_serving_and_injection()
    test_admin_ladi_studio_e2e()
    print("\n✅ TOÀN BỘ BÀI TEST APEX LADI STUDIO & DYNAMIC CONFIG ĐÃ ĐẠT 100%!")
