"""
Capture visual screenshots of the responsive landing page and admin CRM dashboard
"""
import os
from playwright.sync_api import sync_playwright

BASE_URL = "http://localhost:8080"
OUTPUT_DIR = "tests/screenshots"

os.makedirs(OUTPUT_DIR, exist_ok=True)

with sync_playwright() as p:
    browser = p.chromium.launch(headless=True)

    # 1. Landing Mobile View (iPhone 14 / 390x844) to verify form above the fold
    ctx_mobile = browser.new_context(viewport={"width": 390, "height": 844}, is_mobile=True)
    page_mobile = ctx_mobile.new_page()
    page_mobile.goto(f"{BASE_URL}/ladi.html?bypass=1&name=Nguyen%20An&phone=0984386538", wait_until="networkidle")
    page_mobile.screenshot(path=f"{OUTPUT_DIR}/01_landing_mobile_elevated.png", full_page=False)
    print("Saved: 01_landing_mobile_elevated.png")
    ctx_mobile.close()

    # 2. Admin CRM Table View (Desktop 1440x900)
    ctx_desktop = browser.new_context(viewport={"width": 1440, "height": 900})
    page = ctx_desktop.new_page()
    page.goto(f"{BASE_URL}/admin", wait_until="networkidle")
    page.wait_for_timeout(1000)
    page.screenshot(path=f"{OUTPUT_DIR}/02_admin_crm_table_view.png")
    print("Saved: 02_admin_crm_table_view.png")

    # 3. Admin CRM Drawer 360 Open
    page.locator("#crmTableBody tr").first.click()
    page.wait_for_timeout(600)
    page.screenshot(path=f"{OUTPUT_DIR}/03_admin_customer_360_drawer.png")
    print("Saved: 03_admin_customer_360_drawer.png")

    # Close drawer
    page.click("#leadDrawer button:has(.fa-xmark)")
    page.wait_for_timeout(300)

    # 4. Admin CRM Kanban View (7 Stages)
    page.click("#viewBtnKanban")
    page.wait_for_timeout(600)
    page.screenshot(path=f"{OUTPUT_DIR}/04_admin_crm_kanban_view.png")
    print("Saved: 04_admin_crm_kanban_view.png")

    # 5. Admin Analytics Tab (Chart.js & Golden Hour)
    page.click("#tabBtnAnalytics")
    page.wait_for_timeout(1200)
    page.screenshot(path=f"{OUTPUT_DIR}/05_admin_analytics_charts.png")
    print("Saved: 05_admin_analytics_charts.png")

    # 6. Admin Google Sheets Sync Tab
    page.click("#tabBtnSheets")
    page.wait_for_timeout(1000)
    page.screenshot(path=f"{OUTPUT_DIR}/06_admin_google_sheets_sync_list.png")
    print("Saved: 06_admin_google_sheets_sync_list.png")

    # 7. Admin Google Sheets Modal (Matrix Mapping)
    page.click("button:has-text('Thêm Kết Nối Google Sheet Mới')")
    page.wait_for_timeout(600)
    page.screenshot(path=f"{OUTPUT_DIR}/07_admin_google_sheets_config_modal.png")
    print("Saved: 07_admin_google_sheets_config_modal.png")
    page.click("#googleSheetModal button:has-text('Đóng / Hủy')")
    page.wait_for_timeout(300)

    # 8. Admin Google Sheets Audit Logs Modal
    page.click("button:has-text('Xem Lịch Sử Đồng Bộ (Audit Logs)')")
    page.wait_for_timeout(800)
    page.screenshot(path=f"{OUTPUT_DIR}/08_admin_google_sheets_audit_logs.png")
    print("Saved: 08_admin_google_sheets_audit_logs.png")
    page.click("#syncLogsModal button:has-text('Đóng')")

    ctx_desktop.close()
    browser.close()
print("All screenshots captured successfully!")
