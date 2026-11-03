import sys
import time
import os
import json
import urllib.request
import http.cookiejar
from playwright.sync_api import sync_playwright

if sys.stdout.encoding != 'utf-8':
    try:
        sys.stdout.reconfigure(encoding='utf-8')
    except Exception:
        pass

BASE_URL = "http://localhost:8080"
SCREENSHOT_DIR = os.path.join(os.path.dirname(__file__), "screenshots")
os.makedirs(SCREENSHOT_DIR, exist_ok=True)

cookie_jar = http.cookiejar.CookieJar()
opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(cookie_jar))
login_data = json.dumps({"username": "admin", "password": "ApexAdmin@2026!"}).encode('utf-8')
req = urllib.request.Request(f"{BASE_URL}/api/admin/login", data=login_data, headers={"Content-Type": "application/json"})
with opener.open(req) as resp:
    pass

with sync_playwright() as p:
    browser = p.chromium.launch(headless=True)
    context = browser.new_context(viewport={"width": 1440, "height": 900})

    playwright_cookies = []
    for cookie in cookie_jar:
        playwright_cookies.append({
            "name": cookie.name,
            "value": cookie.value,
            "domain": "localhost",
            "path": "/"
        })
    context.add_cookies(playwright_cookies)

    page = context.new_page()
    page.goto(f"{BASE_URL}/ads", wait_until="networkidle")
    page.wait_for_timeout(1000)

    # Switch to Ladi Studio
    page.evaluate("switchMainTab('ladiStudio')")
    page.wait_for_timeout(600)

    # Open Create Landing Modal
    page.evaluate("openCreateLandingModal()")
    page.wait_for_timeout(600)

    # Switch to Tab 2
    page.evaluate("switchSettingsTab('form')")
    page.wait_for_timeout(400)

    # Trigger scan
    page.evaluate("scanLandingFormFields()")
    page.wait_for_timeout(1500)

    # Take screenshot of Modal with Scanned Form Fields
    shot_path_1 = os.path.join(SCREENSHOT_DIR, "17_form_auto_scanner_table.png")
    page.screenshot(path=shot_path_1)
    print(f"Saved screenshot: {shot_path_1}")

    browser.close()
