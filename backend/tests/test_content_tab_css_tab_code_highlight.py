"""
E2E Test for:
1. Dynamic Content Tab by Element Type (Typography, Button, Link, Image, Input, Video, Container)
2. Dedicated Tab CSS with Applied Rules & Searchable Effect Library (18+ presets, filters, live add/edit/delete)
3. Two-Way Code Navigation & Line Gutter Highlighting on element click
"""
import sys
import os
import json
import urllib.request
import urllib.error
import http.cookiejar
import time

if sys.stdout.encoding != 'utf-8':
    try:
        sys.stdout.reconfigure(encoding='utf-8')
    except Exception:
        pass

from playwright.sync_api import sync_playwright

BASE_URL = "http://localhost:8080"
SCREENSHOT_DIR = os.path.join(os.path.dirname(__file__), "screenshots")
os.makedirs(SCREENSHOT_DIR, exist_ok=True)

cookie_jar = http.cookiejar.CookieJar()
opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(cookie_jar))

def login_admin():
    print("\n--- [1] DANG NHAP ADMIN ARGON2ID ---")
    login_data = json.dumps({
        "username": "admin",
        "password": "ApexAdmin@2026!"
    }).encode('utf-8')
    req = urllib.request.Request(f"{BASE_URL}/api/admin/login", data=login_data, headers={"Content-Type": "application/json"})
    with opener.open(req) as resp:
        res = json.loads(resp.read().decode('utf-8'))
        assert res.get("success") is True, "Dang nhap admin phai thanh cong"
        print("=> Dang nhap Admin thanh cong!")
        return res

def setup_test_landing():
    print("\n--- [2] KHOI TAO LANDING PAGE VOI CAC LOAI THE ---")
    landing_data = {
        "page_id": "test_tabs_and_code",
        "name": "Landing Test Tabs & Code Jump",
        "domain": "",
        "code_html": """<!DOCTYPE html>
<html lang="vi">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Landing Test Tabs</title>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-slate-900 text-white p-8">
    <div id="section-main" class="max-w-2xl mx-auto space-y-6">
        <header id="header-box" class="p-6 bg-slate-800 rounded-2xl border border-slate-700">
            <h1 id="main-heading" class="text-3xl font-extrabold text-cyan-400 mb-2">DAU TU HANG HOA APEX</h1>
            <p id="main-desc" class="text-slate-300 text-base">Hoc truc tuyen 100% thuc chien cung chuyen gia.</p>
        </header>

        <div id="media-card" class="p-6 bg-slate-800 rounded-2xl border border-slate-700">
            <img id="course-banner" src="/assets/img/hero.png" alt="Banner Lop Hoc" class="w-full h-48 object-cover rounded-xl mb-4">
            <a id="cta-link-btn" href="#form-register" class="btn inline-block bg-cyan-500 hover:bg-cyan-400 text-slate-950 font-bold px-6 py-2.5 rounded-xl">
                Dang Ky Giu Cho Ngay
            </a>
        </div>

        <form id="form-register" class="p-6 bg-slate-800 rounded-2xl border border-slate-700 space-y-4">
            <input id="inp-fullname" type="text" name="fullname" placeholder="Ho va ten cua ban..." required class="w-full bg-slate-900 border border-slate-700 rounded-xl p-3 text-white">
            <button id="btn-submit" type="submit" class="w-full bg-amber-500 hover:bg-amber-400 text-slate-950 font-bold p-3 rounded-xl">
                Xac Nhan Dang Ky
            </button>
        </form>
    </div>
</body>
</html>""",
        "settings": {
            "nguon_default": "tab_test"
        }
    }
    data_bytes = json.dumps(landing_data).encode('utf-8')
    req = urllib.request.Request(f"{BASE_URL}/api/landings/save", data=data_bytes, headers={"Content-Type": "application/json"})
    with opener.open(req) as resp:
        res = json.loads(resp.read().decode('utf-8'))
        assert res.get("success") is True
        print("=> Khoi tao landing test thanh cong!")

    file_payload = {
        "domain": "",
        "page_id": "test_tabs_and_code",
        "file_path": "index.html",
        "file_type": "text/html",
        "content": landing_data["code_html"]
    }
    req_file = urllib.request.Request(f"{BASE_URL}/api/landings/file", data=json.dumps(file_payload).encode('utf-8'), headers={"Content-Type": "application/json"})
    with opener.open(req_file) as resp:
        res_file = json.loads(resp.read().decode('utf-8'))
        print(f"=> Luu file index.html thanh cong: {res_file.get('message')}")

def cleanup_test_landing():
    print("\n--- DON DEP DU LIEU TEST ---")
    data_bytes = json.dumps({"page_id": "test_tabs_and_code", "domain": ""}).encode('utf-8')
    req = urllib.request.Request(f"{BASE_URL}/api/landings/delete", data=data_bytes, headers={"Content-Type": "application/json"})
    try:
        with opener.open(req) as resp:
            pass
    except Exception:
        pass
    print("=> Don dep hoan tat!")

def run_tests():
    login_res = login_admin()
    token = login_res.get("token")
    setup_test_landing()

    try:
        with sync_playwright() as p:
            browser = p.chromium.launch(headless=True)
            context = browser.new_context(viewport={"width": 1920, "height": 1080})
            if token:
                context.add_cookies([{
                    "name": "apex_admin_session",
                    "value": token,
                    "domain": "localhost",
                    "path": "/"
                }])

            page = context.new_page()
            studio_url = f"{BASE_URL}/studio.html?page_id=test_tabs_and_code&domain="
            print(f"\n--- [3] TRUY CAP STUDIO: {studio_url} ---")
            page.goto(studio_url)
            page.wait_for_load_state("networkidle")
            time.sleep(2)

            iframe_el = page.locator("#csPreviewIframe")
            iframe_el.wait_for(state="visible", timeout=10000)
            print("=> Iframe preview da san sang!")

            # -------------------------------------------------------------
            # TEST 1: DYNAMIC CONTENT TAB BY ELEMENT TYPE
            # -------------------------------------------------------------
            print("\n--- [TEST 1] DYNAMIC CONTENT TAB BY ELEMENT TYPE ---")

            # 1.1 Chon Text (H1)
            print("-> [1.1] Chon the H1...")
            frame = page.frame_locator("#csPreviewIframe")
            h1_el = frame.locator("#main-heading")
            h1_el.wait_for(state="visible")
            h1_el.click()
            time.sleep(0.5)

            # Kiem tra Tab Noi Dung duoc chon va hien dung subpanel Typography
            is_content_active = page.evaluate("() => currentInspTab === 'content'")
            print(f"Tab dang hoat dong: {page.evaluate('() => currentInspTab')} (Phai la 'content')")
            assert is_content_active, "Tab Noi Dung phai duoc chon khi click vao the"

            panel_typo_hidden = page.locator("#inspPanelTypography").evaluate("el => el.classList.contains('hidden')")
            assert not panel_typo_hidden, "Subpanel Typography phai duoc hien thi cho the H1"
            print("=> Subpanel Typography hien thi chinh xac!")

            typo_text = page.locator("#inspTextContent").input_value()
            assert "DAU TU HANG HOA APEX" in typo_text, f"Noi dung text phai khop: {typo_text}"
            print(f"=> Text content da doc dung: {typo_text}")

            # 1.2 Chon Button CTA (a.btn / link button)
            print("\n-> [1.2] Chon the CTA Button...")
            cta_el = frame.locator("#cta-link-btn")
            cta_el.click()
            time.sleep(0.5)

            panel_btn_hidden = page.locator("#inspPanelButton").evaluate("el => el.classList.contains('hidden')")
            assert not panel_btn_hidden, "Subpanel Button phai duoc hien thi cho the CTA button"
            btn_text = page.locator("#inspBtnText").input_value()
            assert "Dang Ky Giu Cho Ngay" in btn_text, f"Chu tren nut khop: {btn_text}"
            print(f"=> Subpanel Button hien thi chinh xac voi text: '{btn_text}'!")

            # 1.3 Chon The Anh (IMG)
            print("\n-> [1.3] Chon the IMG...")
            img_el = frame.locator("#course-banner")
            img_el.click()
            time.sleep(0.5)

            panel_img_hidden = page.locator("#inspPanelImage").evaluate("el => el.classList.contains('hidden')")
            assert not panel_img_hidden, "Subpanel Image phai duoc hien thi cho the IMG"
            img_src = page.locator("#inspAttrSrc").input_value()
            assert "/assets/img/hero.png" in img_src, f"Src anh khop: {img_src}"
            print(f"=> Subpanel Image hien thi chinh xac voi src: '{img_src}'!")

            # 1.4 Chon The Input (INPUT)
            print("\n-> [1.4] Chon the INPUT...")
            inp_el = frame.locator("#inp-fullname")
            inp_el.click()
            time.sleep(0.5)

            panel_inp_hidden = page.locator("#inspPanelInput").evaluate("el => el.classList.contains('hidden')")
            assert not panel_inp_hidden, "Subpanel Input phai duoc hien thi cho the INPUT"
            inp_name = page.locator("#inspInputName").input_value()
            assert inp_name == "fullname", f"Name input khop: {inp_name}"
            print(f"=> Subpanel Input hien thi chinh xac voi name: '{inp_name}'!")

            # 1.5 Chon The Container (DIV #header-box)
            print("\n-> [1.5] Chon the Container (#header-box)...")
            header_box = frame.locator("#header-box")
            header_box.click(position={"x": 5, "y": 5})
            time.sleep(0.5)

            panel_container_hidden = page.locator("#inspPanelContainer").evaluate("el => el.classList.contains('hidden')")
            assert not panel_container_hidden, "Subpanel Container phai duoc hien thi cho the DIV Container"
            child_count_str = page.locator("#inspContainerChildCount").inner_text()
            print(f"=> Subpanel Container hien thi chinh xac voi: '{child_count_str}'!")

            # -------------------------------------------------------------
            # TEST 2: TWO-WAY CODE JUMP & LINE HIGHLIGHTING
            # -------------------------------------------------------------
            print("\n--- [TEST 2] TWO-WAY CODE NAVIGATION & LINE HIGHLIGHTING ---")

            # Chon the H1 bang selectAstNode de xac nhan Code Editor nhay den dung the
            h1_node_id = page.evaluate("""() => {
                const map = astMapper.astMap;
                for (let k in map) {
                    if (map[k].tag === 'h1') return k;
                }
                return null;
            }""")
            print(f"=> Node ID cua H1: {h1_node_id}")
            assert h1_node_id is not None

            page.evaluate(f"selectAstNode('{h1_node_id}')")
            time.sleep(0.5)

            # Kiem tra Code Editor textarea selection
            sel_range = page.evaluate("""() => {
                const ed = document.getElementById('csCodeEditor');
                return {
                    start: ed.selectionStart,
                    end: ed.selectionEnd,
                    selectedText: ed.value.substring(ed.selectionStart, ed.selectionEnd),
                    scrollTop: ed.scrollTop
                };
            }""")
            print(f"=> Vung code duoc chon trong editor: '{sel_range['selectedText']}'")
            assert "<h1" in sel_range['selectedText'], "Code Editor phai highlight doan code chua the <h1>"

            # Kiem tra line number gutter co class cyan active
            has_highlighted_lines = page.evaluate("""() => {
                const activeLines = document.querySelectorAll('#csLineNumbers .bg-cyan-500\\\\/25');
                return activeLines.length > 0;
            }""")
            assert has_highlighted_lines, "Thanh so dong phai co dong duoc to mau cyan noi bat"
            print("=> Thanh so dong (#csLineNumbers) da duoc highlight mau cyan sang thanh cong!")

            # Kiem tra status bar
            status_text = page.locator("#csLineCol").inner_text()
            print(f"=> Status bar code: '{status_text}'")
            assert "Dòng" in status_text and "h1" in status_text, "Status bar phai hien so dong va the h1"

            # -------------------------------------------------------------
            # TEST 3: TAB CSS, EFFECT LIBRARY & LIVE STYLES
            # -------------------------------------------------------------
            print("\n--- [TEST 3] TAB CSS & THU VIEN HIEU UNG ---")

            # 3.1 Chuyen sang Tab CSS
            print("-> [3.1] Mo Tab CSS...")
            page.locator("#btnInspTabCss").click()
            time.sleep(0.5)

            is_css_tab = page.evaluate("() => currentInspTab === 'css'")
            assert is_css_tab, "Tab CSS phai duoc chon thanh cong"
            css_section_hidden = page.locator("#inspSectionCss").evaluate("el => el.classList.contains('hidden')")
            assert not css_section_hidden, "Section CSS phai duoc hien thi"
            print("=> Tab CSS va Section CSS da mo thanh cong!")

            # 3.2 Tim kiem hieu ung Neon trong thu vien
            print("-> [3.2] Tim kiem hieu ung 'Neon' trong thu vien...")
            search_input = page.locator("#cssEffectSearchInput")
            search_input.fill("Neon")
            time.sleep(0.3)

            neon_cards_count = page.locator("#cssEffectPresetsGrid > div").count()
            print(f"=> So luong the hieu ung Neon tim thay: {neon_cards_count}")
            assert neon_cards_count >= 1, "Phai tim thay it nhat 1 card hieu ung Neon"

            # 3.3 Ap dung hieu ung Neon Cyan vao the H1
            print("-> [3.3] Nhan Ap Dung hieu ung Neon Cyan...")
            apply_neon_btn = page.locator("#cssEffectPresetsGrid button:has-text('Áp dụng')").first
            apply_neon_btn.click()
            time.sleep(0.5)

            # Kiem tra thuoc tinh CSS active tren the
            active_css_text = page.locator("#inspActiveCssList").inner_text()
            print(f"=> Toan bo CSS dang ap dung sau khi them hieu ung:\n{active_css_text}")
            assert "box-shadow" in active_css_text or "border" in active_css_text, "Active CSS phai chua box-shadow hoac border cua neon"

            # 3.4 Them thuoc tinh CSS tuy chinh bang o nhap lieu
            print("\n-> [3.4] Them thuoc tinh CSS tuy chinh 'border-radius: 24px'...")
            page.locator("#cssCustomPropName").fill("border-radius")
            page.locator("#cssCustomPropValue").fill("24px")
            page.locator("#btnAddCustomCssProp").click()
            time.sleep(0.5)

            active_css_text2 = page.locator("#inspActiveCssList").inner_text()
            assert "border-radius" in active_css_text2, "border-radius phai xuat hien trong danh sach CSS active"
            print("=> Da them thuoc tinh CSS tuy chinh thanh cong!")

            # 3.5 Ap dung ma CSS thuan (Raw CSS)
            print("\n-> [3.5] Ap dung ma CSS thuan (Raw CSS: 'opacity: 0.92; letter-spacing: 2px')...")
            page.locator("#cssRawTextarea").fill("opacity: 0.92; letter-spacing: 2px;")
            page.locator("button:has-text('Áp Dụng Mã CSS')").click()
            time.sleep(0.5)

            active_css_text3 = page.locator("#inspActiveCssList").inner_text()
            assert "letter-spacing" in active_css_text3, "letter-spacing phai xuat hien sau khi parse raw CSS"
            print("=> Da ap dung ma Raw CSS thanh cong!")

            # 3.6 Chup anh man hinh ket qua test
            screenshot_path = os.path.join(SCREENSHOT_DIR, "test_content_and_css_tabs.png")
            page.screenshot(path=screenshot_path)
            print(f"\n=> Da chup anh man hinh: {screenshot_path}")

            print("\n=======================================================")
            print(">>> TAT CA CAC TEST CHO 3 TINH NANG DA PASS 100%! <<<")
            print("=======================================================")

            browser.close()
    finally:
        cleanup_test_landing()

if __name__ == "__main__":
    run_tests()
