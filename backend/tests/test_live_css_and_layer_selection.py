"""
E2E Test verifying:
1. Live Applied CSS Extraction in Tab CSS (extracts real stylesheet & computed styles without needing inline style)
2. Tab Layer Selection & State Retention:
   - Clicking element in preview highlights layer in Tab Layer with active cyan border & 'CHỌN' badge
   - Preserves Tab Layer when active (does not kick user to content tab)
   - Clicking in Code Editor automatically selects that layer in Tab Layer
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
        assert res.get("success") is True
        print("=> Dang nhap Admin thanh cong!")
        return res

def setup_test_landing():
    print("\n--- [2] KHOI TAO LANDING PAGE TEST ---")
    landing_data = {
        "page_id": "test_live_css_layer",
        "name": "Landing Test Live CSS & Layer Selection",
        "domain": "",
        "code_html": """<!DOCTYPE html>
<html lang="vi">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Landing Test Live CSS & Layer</title>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-slate-900 text-white p-8">
    <div id="section-main" class="max-w-2xl mx-auto space-y-6">
        <header id="header-box" class="p-6 bg-slate-800 rounded-2xl border border-slate-700">
            <h1 id="main-heading" class="text-3xl font-extrabold text-cyan-400 mb-2">DAU TU HANG HOA APEX</h1>
            <p id="main-desc" class="text-slate-300 text-base">Lop hoc thuc chien 100%.</p>
        </header>

        <div id="media-card" class="p-6 bg-slate-800 rounded-2xl border border-slate-700">
            <img id="course-banner" src="/assets/img/hero.png" alt="Banner" class="w-full h-48 object-cover rounded-xl mb-4">
            <a id="cta-link-btn" href="#form-register" class="btn inline-block bg-cyan-500 text-slate-950 font-bold px-6 py-2.5 rounded-xl">
                Dang Ky Ngay
            </a>
        </div>
    </div>
</body>
</html>""",
        "settings": {
            "nguon_default": "test"
        }
    }
    data_bytes = json.dumps(landing_data).encode('utf-8')
    req = urllib.request.Request(f"{BASE_URL}/api/landings/save", data=data_bytes, headers={"Content-Type": "application/json"})
    with opener.open(req) as resp:
        res = json.loads(resp.read().decode('utf-8'))
        assert res.get("success") is True

    file_payload = {
        "domain": "",
        "page_id": "test_live_css_layer",
        "file_path": "index.html",
        "file_type": "text/html",
        "content": landing_data["code_html"]
    }
    req_file = urllib.request.Request(f"{BASE_URL}/api/landings/file", data=json.dumps(file_payload).encode('utf-8'), headers={"Content-Type": "application/json"})
    with opener.open(req_file) as resp:
        pass
    print("=> Khoi tao landing test thanh cong!")

def cleanup_test_landing():
    data_bytes = json.dumps({"page_id": "test_live_css_layer", "domain": ""}).encode('utf-8')
    req = urllib.request.Request(f"{BASE_URL}/api/landings/delete", data=data_bytes, headers={"Content-Type": "application/json"})
    try:
        with opener.open(req) as resp:
            pass
    except Exception:
        pass

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
            studio_url = f"{BASE_URL}/studio.html?page_id=test_live_css_layer&domain="
            print(f"\n--- [3] TRUY CAP STUDIO: {studio_url} ---")
            page.goto(studio_url)
            page.wait_for_load_state("networkidle")
            time.sleep(2)

            iframe_el = page.locator("#csPreviewIframe")
            iframe_el.wait_for(state="visible", timeout=10000)
            frame = page.frame_locator("#csPreviewIframe")
            h1_el = frame.locator("#main-heading")
            h1_el.wait_for(state="visible")

            # -------------------------------------------------------------
            # TEST A: LIVE APPLIED CSS EXTRACTION TRONG TAB CSS
            # -------------------------------------------------------------
            print("\n--- [TEST A] KIEM TRA TRICH XUAT CSS DANG AP DUNG TRONG TAB CSS ---")
            # 1. Click chon the H1
            print("-> [A.1] Click chon the H1 trong Canvas Preview...")
            h1_el.click()
            time.sleep(0.5)

            # 2. Chuyen sang Tab CSS
            print("-> [A.2] Chuyen sang Tab CSS...")
            page.locator("#btnInspTabCss").click()
            time.sleep(0.5)

            # 3. Kiem tra so luong thuoc tinh CSS active phai > 0 (Khong duoc la 0 thuoc tinh)
            count_text = page.locator("#cssActivePropsCount").inner_text()
            print(f"=> So luong thuoc tinh CSS duoc trích xuất: {count_text}")
            count_num = int(count_text.replace("thuộc tính", "").strip())
            assert count_num > 5, f"Thuoc tinh CSS phai > 5 (thuc te la {count_num})"

            # 4. Kiem tra cac thuoc tinh Tailwind / Computed cot loi xuat hien
            active_list_text = page.locator("#inspActiveCssList").inner_text()
            print("=> Danh sach CSS active tren the H1:")
            print(active_list_text[:300] + "...")
            assert "font-size" in active_list_text or "color" in active_list_text, "Phai chua font-size hoac color"
            assert "font-weight" in active_list_text or "display" in active_list_text, "Phai chua font-weight hoac display"
            print("=> TEST A PASS: Tab CSS trich xuat day du CSS dang ap dung cua the thanh cong!")

            # -------------------------------------------------------------
            # TEST B: TAB LAYER AUTO-SELECTION & STATE RETENTION
            # -------------------------------------------------------------
            print("\n--- [TEST B] KIEM TRA TAB LAYER CHON DUNG LAYER & GIU NGUYEN TAB ---")

            # 1. Chuyen sang Tab Layer
            print("-> [B.1] Chuyen sang Tab Layer...")
            page.locator("#btnInspTabLayers").click()
            time.sleep(0.5)

            is_layers_tab = page.evaluate("() => currentInspTab === 'layers'")
            assert is_layers_tab, "Tab Layer phai duoc kich hoat"

            # 2. Kiem tra layer H1 dang duoc chon noi bat
            selected_node_id = page.evaluate("() => selectedAstNodeId")
            print(f"=> Node ID dang chon: {selected_node_id}")
            assert selected_node_id is not None

            h1_row = page.locator(f"#layer-row-{selected_node_id}")
            assert h1_row.is_visible(), f"Row #layer-row-{selected_node_id} phai hien thi trong DOM tree"

            is_active_class = h1_row.evaluate("el => el.classList.contains('layer-row-active')")
            assert is_active_class, "Row phai co class layer-row-active"
            h1_row_text = h1_row.inner_text()
            print(f"=> Noi dung row layer dang chon: {h1_row_text}")
            assert "CHỌN" in h1_row_text, "Row phai co badge CHỌN"
            print("=> Layer H1 duoc highlight noi bat chinh xac tren Tab Layer!")

            # 3. Khi DANG O TAB LAYER, click chon the Anh #course-banner trong Canvas Preview
            print("\n-> [B.2] Khi dang o Tab Layer, click the Anh #course-banner trong Canvas...")
            img_el = frame.locator("#course-banner")
            img_el.click()
            time.sleep(0.5)

            # 4. Xac nhan Tab KHONG bi nhay sang 'content' ma van GIU NGUYEN 'layers'
            current_tab_after_click = page.evaluate("() => currentInspTab")
            print(f"=> Tab hien tai sau khi click: '{current_tab_after_click}' (Phai la 'layers')")
            assert current_tab_after_click == "layers", "Tab Layer phai duoc GIU NGUYEN, khong duoc bi ep ve tab content"

            # 5. Xac nhan layer Anh duoc tu dong chon tren Tab Layer
            new_node_id = page.evaluate("() => selectedAstNodeId")
            print(f"=> Node ID moi cua the Anh: {new_node_id}")
            assert new_node_id != selected_node_id

            img_row = page.locator(f"#layer-row-{new_node_id}")
            assert img_row.is_visible(), "Row cua the Anh phai hien thi"
            is_img_active = img_row.evaluate("el => el.classList.contains('layer-row-active')")
            assert is_img_active, "Row the Anh phai co class layer-row-active"
            assert "img" in img_row.inner_text().lower() and "CHỌN" in img_row.inner_text(), "Row phai hien the img va badge CHỌN"
            print("=> Tab Layer tu dong chon dung the Anh va highlight day du!")

            # -------------------------------------------------------------
            # TEST C: CLICK CODE EDITOR TU DONG CHON DUNG LAYER O TAB LAYER
            # -------------------------------------------------------------
            print("\n--- [TEST C] CLICK TRONG CODE EDITOR TU DONG CHON LAYER O TAB LAYER ---")

            # Tim offset cua chuoi 'cta-link-btn' trong code editor
            cta_offset = page.evaluate("""() => {
                const ed = document.getElementById('csCodeEditor');
                const idx = ed.value.indexOf('id="cta-link-btn"');
                ed.focus();
                ed.setSelectionRange(idx + 5, idx + 5);
                onCodeEditorClick();
                return idx;
            }""")
            time.sleep(0.5)

            cta_node_id = page.evaluate("() => selectedAstNodeId")
            print(f"=> Node ID sau khi click code cua CTA button: {cta_node_id}")
            assert cta_node_id is not None

            cta_row = page.locator(f"#layer-row-{cta_node_id}")
            assert cta_row.is_visible(), "Row cua the CTA button phai hien thi trong Layer tab"
            assert cta_row.evaluate("el => el.classList.contains('layer-row-active')"), "Row CTA button phai active"
            print("=> Click code editor tu dong chon dung the trong Tab Layer thanh cong!")

            # Chup anh man hinh ket qua test
            screenshot_path = os.path.join(SCREENSHOT_DIR, "test_live_css_layer_selection.png")
            page.screenshot(path=screenshot_path)
            print(f"\n=> Da chup anh man hinh: {screenshot_path}")

            print("\n=================================================================")
            print(">>> TAT CA CAC TEST CHO LIVE CSS & LAYER SELECTION DA PASS 100%! <<<")
            print("=================================================================")

            browser.close()
    finally:
        cleanup_test_landing()

if __name__ == "__main__":
    run_tests()
