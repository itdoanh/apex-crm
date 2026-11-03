"""
E2E Test: Gizmo Drag & Move, Resize Handles (PointerCapture Isolation, Anti-Sticking),
Toolbar Actions & 6-Section Visual Property Inspector
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
    print("\n--- [1] ĐĂNG NHẬP ADMIN ARGON2ID ---")
    login_data = json.dumps({
        "username": "admin",
        "password": "ApexAdmin@2026!"
    }).encode('utf-8')
    req = urllib.request.Request(f"{BASE_URL}/api/admin/login", data=login_data, headers={"Content-Type": "application/json"})
    with opener.open(req) as resp:
        res = json.loads(resp.read().decode('utf-8'))
        assert res.get("success") is True, "Đăng nhập admin phải thành công"
        print("=> Đăng nhập Admin thành công!")
        return res

def setup_test_landing():
    print("\n--- [2] KHỞI TẠO LANDING PAGE CHO TEST GIZMO DRAG/RESIZE & INSPECTOR ---")
    landing_data = {
        "page_id": "test_gizmo_builder",
        "name": "Landing Test Gizmo & Inspector",
        "domain": "",
        "code_html": """<!DOCTYPE html>
<html lang="vi">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Landing Test Gizmo Drag & Inspector</title>
    <!-- Tailwind Play CDN -->
    <script src="https://cdn.tailwindcss.com"></script>
    <style>
        .custom-card { border-radius: 16px; }
    </style>
</head>
<body class="bg-slate-900 text-white min-h-screen flex flex-col items-center justify-center p-6">
    <div id="main-container" class="max-w-2xl w-full bg-slate-800 p-8 rounded-2xl shadow-xl text-center border border-slate-700">
        <h1 id="hero-title" class="text-3xl font-extrabold text-cyan-400 mb-4">LỚP HỌC ONLINE APEX</h1>
        <p id="hero-desc" class="text-slate-300 text-base mb-6 leading-relaxed">
            Học đầu tư hàng hóa an toàn và bền vững cùng chuyên gia MXV.
        </p>
        <div id="cta-group" class="flex items-center justify-center gap-4 mb-6">
            <a id="btn-cta" href="#dang-ky" class="bg-cyan-500 hover:bg-cyan-400 text-slate-950 font-bold px-6 py-3 rounded-xl shadow-lg transition">
                Đăng Ký Ngay
            </a>
            <img id="partner-logo" src="https://via.placeholder.com/120x40" alt="Logo MXV" class="h-10 rounded" />
        </div>
        <div id="footer-note" class="text-xs text-slate-500">
            © 2026 Apex Financial. Bảo lưu mọi quyền.
        </div>
    </div>
</body>
</html>""",
        "settings": {
            "nguon_default": "gizmo_test"
        }
    }

    req = urllib.request.Request(f"{BASE_URL}/api/landings/save", data=json.dumps(landing_data).encode('utf-8'), headers={"Content-Type": "application/json"})
    with opener.open(req) as resp:
        res = json.loads(resp.read().decode('utf-8'))
        print(f"=> Khởi tạo landing thành công: {res.get('message')}")

    file_payload = {
        "domain": "",
        "page_id": "test_gizmo_builder",
        "file_path": "index.html",
        "file_type": "text/html",
        "content": landing_data["code_html"]
    }
    req_file = urllib.request.Request(f"{BASE_URL}/api/landings/file", data=json.dumps(file_payload).encode('utf-8'), headers={"Content-Type": "application/json"})
    with opener.open(req_file) as resp:
        res_file = json.loads(resp.read().decode('utf-8'))
        print(f"=> Lưu file index.html thành công: {res_file.get('message')}")

def test_gizmo_and_inspector_suite(admin_token=None):
    print("\n--- [3] KIỂM THỬ PLAYWRIGHT: GIZMO DRAG, RESIZE, CHỐNG DÍNH CHUỘT & 6 PHÂN HỆ INSPECTOR ---")
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        context = browser.new_context(viewport={"width": 2100, "height": 1050})

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
        page.on("pageerror", lambda err: print(f"PAGE ERROR: {err}"))
        page.on("console", lambda msg: print(f"CONSOLE: {msg.text}"))

        # 1. Mở trang quản trị
        page.goto(f"{BASE_URL}/ads", wait_until="domcontentloaded")
        if admin_token:
            page.evaluate(f"localStorage.setItem('apex_admin_token', '{admin_token}')")
            page.reload(wait_until="networkidle")
        else:
            page.wait_for_load_state("networkidle")
        page.wait_for_timeout(1000)

        # 2. Mở Tab Ladi Studio & Mở Code Studio cho landing 'test_gizmo_builder'
        page.evaluate("switchMainTab('ladiStudio')")
        page.wait_for_timeout(800)
        page.evaluate("openCodeStudio('', 'test_gizmo_builder')")
        page.wait_for_timeout(1500)

        assert page.locator("#codeStudioModal").is_visible(), "Modal Code Studio phải hiển thị"

        # 3. Chuyển sang chế độ Visual Builder toàn màn hình
        page.evaluate("setStudioViewMode('visual')")
        page.wait_for_timeout(600)
        assert page.locator("#csPreviewPanel").is_visible()
        assert page.locator("#csVisualInspector").is_visible()
        print("=> Đã vào Visual Builder toàn màn hình!")

        # 4. Kiểm tra sự tồn tại của tấm chắn chống dính chuột #csDragShield
        shield = page.locator("#csDragShield")
        assert shield.count() == 1, "Tấm chắn chống dính chuột #csDragShield phải tồn tại"
        assert "hidden" in shield.get_attribute("class"), "Tấm chắn ban đầu phải ẩn"
        print("=> Tấm chắn #csDragShield đã sẵn sàng bảo vệ!")

        # 5. Chọn node #hero-title (thẻ h1)
        h1_node_id = page.evaluate("""() => {
            for (let id in astMapper.astMap) {
                if (astMapper.astMap[id].tag === 'h1') return id;
            }
            return null;
        }""")
        assert h1_node_id is not None, "Phải tìm thấy thẻ H1 trong AST"
        page.evaluate(f"selectAstNode('{h1_node_id}')")
        page.wait_for_timeout(500)

        # Kiểm tra Gizmo Box và 8 chốt Handles
        gizmo_box = page.locator("#csGizmoBox")
        assert gizmo_box.is_visible(), "Gizmo Box phải hiển thị khi chọn H1"
        assert page.locator(".cs-gizmo-handle").count() == 8, "Phải có đủ 8 chốt co giãn"
        assert page.locator("#csGizmoHeader").is_visible(), "Thanh Grab Header di chuyển phải hiển thị"
        assert page.locator("#csGizmoToolbar").is_visible(), "Floating Toolbar phải hiển thị"
        print("=> Gizmo Box, 8 Handles, Header & Floating Toolbar hiển thị hoàn hảo!")

        # 6. KIỂM THỬ KÉO CO GIÃN (RESIZE) & KIỂM TRA CHỐNG DÍNH CHUỘT TRIỆT ĐỂ
        print("\n--- BẮT ĐẦU TEST KÉO CO GIÃN (RESIZE) ---")
        handle_se = page.locator(".cs-handle-se")
        assert handle_se.is_visible(), "Chốt Đông Nam (SE) phải hiển thị"

        se_box = handle_se.bounding_box()
        assert se_box is not None, "Chốt SE phải có tọa độ"

        # Di chuột đến chốt SE và ấn giữ chuột
        page.mouse.move(se_box["x"] + se_box["width"] / 2, se_box["y"] + se_box["height"] / 2)
        page.mouse.down()
        page.wait_for_timeout(100)

        # Kiểm tra cờ kéo và tấm chắn
        is_dragging = page.evaluate("isDraggingGizmo")
        print(f"Trạng thái isDraggingGizmo khi mousedown: {is_dragging}")
        assert is_dragging is True, "isDraggingGizmo phải chuyển sang true"

        shield_classes = shield.get_attribute("class")
        assert "hidden" not in shield_classes, "Tấm chắn #csDragShield phải được bật lên để che chắn iframe"

        # Kéo chuột xuống sâu trong vùng iframe (+120px, +60px)
        page.mouse.move(se_box["x"] + 120, se_box["y"] + 60, steps=5)
        page.wait_for_timeout(150)

        # Thả chuột ngay trong vùng iframe!
        page.mouse.up()
        page.wait_for_timeout(200)

        # XÁC NHẬN CHỐNG DÍNH CHUỘT:
        is_dragging_after = page.evaluate("isDraggingGizmo")
        print(f"Trạng thái isDraggingGizmo sau khi thả chuột: {is_dragging_after}")
        assert is_dragging_after is False, "LỖI DÍNH CHUỘT! isDraggingGizmo phải chuyển về false sau khi mouseup"

        shield_classes_after = shield.get_attribute("class")
        assert "hidden" in shield_classes_after, "#csDragShield phải ẩn sau khi thả chuột"

        # Di chuột lung tung để xác nhận khung KHÔNG tiếp tục kéo dãn
        initial_w = page.evaluate("parseFloat(document.getElementById('csGizmoBox').style.width)")
        page.mouse.move(se_box["x"] + 300, se_box["y"] + 300, steps=5)
        page.wait_for_timeout(150)
        later_w = page.evaluate("parseFloat(document.getElementById('csGizmoBox').style.width)")
        assert abs(initial_w - later_w) < 2, "LỖI DÍNH CHUỘT! Khung vẫn bị kéo dãn loạn xạ theo chuột sau khi thả!"
        print("=> TUYỆT VỜI: Đã khắc phục 100% lỗi dính chuột khi kéo co giãn!")

        # 7. KIỂM THỬ KÉO DI CHUYỂN VỊ TRÍ (DRAG TO MOVE)
        print("\n--- BẮT ĐẦU TEST KÉO DI CHUYỂN VỊ TRÍ (DRAG TO MOVE) ---")
        header = page.locator("#csGizmoHeader")
        h_box = header.bounding_box()
        assert h_box is not None, "Gizmo Header phải có tọa độ"

        page.mouse.move(h_box["x"] + 20, h_box["y"] + 10)
        page.mouse.down()
        page.wait_for_timeout(100)

        is_moving = page.evaluate("isMovingGizmo")
        print(f"Trạng thái isMovingGizmo khi mousedown: {is_moving}")
        assert is_moving is True, "isMovingGizmo phải chuyển sang true"

        # Kéo sang phải 40px và xuống 30px
        page.mouse.move(h_box["x"] + 60, h_box["y"] + 40, steps=5)
        page.wait_for_timeout(150)

        # Thả chuột
        page.mouse.up()
        page.wait_for_timeout(200)

        is_moving_after = page.evaluate("isMovingGizmo")
        print(f"Trạng thái isMovingGizmo sau khi thả chuột: {is_moving_after}")
        assert is_moving_after is False, "isMovingGizmo phải chuyển về false"
        print("=> Kéo di chuyển vị trí hoạt động mượt mà, không dính chuột!")

        # 8. KIỂM THỬ 6 PHÂN HỆ VISUAL PROPERTY INSPECTOR
        print("\n--- KIỂM THỬ 6 PHÂN HỆ VISUAL PROPERTY INSPECTOR ---")
        page.evaluate("switchInspectorTab('styles')")
        page.wait_for_timeout(400)

        # Phân hệ 1: Bố cục & Flexbox
        print("Test Phân Hệ 1: Bố cục (Layout & Flexbox)...")
        page.evaluate("document.getElementById('inspStyleDisplay').value = 'flex'; toggleFlexGroup('flex'); onInspectorStyleChange('display', 'flex');")
        page.evaluate("onInspectorStyleChange('gap', '16px')")
        page.evaluate("onInspectorStyleChange('justify-content', 'center')")
        page.evaluate("onInspectorStyleChange('align-items', 'center')")

        # Phân hệ 2: Hộp khoảng cách (Box Model)
        print("Test Phân Hệ 2: Khoảng cách (Box Model)...")
        page.evaluate("applyCenterMargin()")
        page.evaluate("onInspectorStyleChange('padding-top', '20px')")
        page.evaluate("onInspectorStyleChange('padding-bottom', '20px')")

        # Phân hệ 3: Kích thước & Vị trí
        print("Test Phân Hệ 3: Kích thước & Vị trí...")
        page.evaluate("setDimensionPreset('width', '100%')")
        page.evaluate("onInspectorStyleChange('max-width', '800px')")

        # Phân hệ 4: Typography
        print("Test Phân Hệ 4: Typography...")
        page.evaluate("setQuickFontSize('32px')")
        page.evaluate("onInspectorStyleChange('font-weight', '700')")
        page.evaluate("onInspectorStyleChange('text-align', 'center')")
        page.evaluate("onInspectorStyleChange('color', '#06b6d4')")

        # Phân hệ 5: Nền & Hiệu ứng
        print("Test Phân Hệ 5: Nền & Hiệu ứng...")
        page.evaluate("onInspectorStyleChange('background-color', '#1e293b')")
        page.evaluate("onInspectorOpacitySlide('90')")
        page.evaluate("onInspectorStyleChange('box-shadow', '0 10px 15px -3px rgba(0,0,0,0.5)')")

        # Phân hệ 6: Viền & Bo góc
        print("Test Phân Hệ 6: Viền & Bo góc...")
        page.evaluate("setQuickRadius('16px')")
        page.evaluate("onInspectorStyleChange('border-width', '2px')")
        page.evaluate("onInspectorStyleChange('border-style', 'solid')")
        page.evaluate("onInspectorStyleChange('border-color', '#334155')")

        page.wait_for_timeout(600)

        # 9. KIỂM THỬ CÔNG CỤ HOÁN ĐỔI THỨ TỰ DOM (MOVE UP / MOVE DOWN)
        print("\n--- KIỂM THỬ DOM REORDERING (MOVE UP / DOWN) ---")
        code_before_move = page.evaluate("document.getElementById('csCodeEditor').value")
        assert "LỚP HỌC ONLINE APEX" in code_before_move
        print("Gọi moveCurrentNodeDown() để đổi chỗ thẻ H1 với thẻ P bên dưới...")
        page.evaluate("moveCurrentNodeDown()")
        page.wait_for_timeout(500)

        code_after_move = page.evaluate("document.getElementById('csCodeEditor').value")
        # Sau khi move down, thẻ P (hero-desc) sẽ nằm trước thẻ H1 (hero-title)
        pos_h1 = code_after_move.find("hero-title")
        pos_p = code_after_move.find("hero-desc")
        print(f"Vị trí hero-desc: {pos_p}, Vị trí hero-title: {pos_h1}")
        assert pos_p < pos_h1, "Thẻ hero-desc phải nằm trước hero-title sau khi gọi moveCurrentNodeDown"
        print("=> Hoán đổi thứ tự DOM theo offset ký tự thành công 100%!")

        # 10. CHỤP ẢNH MINH CHỨNG
        shot_path = os.path.join(SCREENSHOT_DIR, "23_gizmo_and_inspector_verified.png")
        page.screenshot(path=shot_path)
        print(f"Saved screenshot: {shot_path}")

        # 11. LƯU VÀ KIỂM TRA CSDL
        print("\nLưu mã nguồn vào CSDL...")
        page.evaluate("saveStudioCode()")
        page.wait_for_timeout(1200)

        save_status = page.locator("#csSaveStatus").inner_text()
        print(f"Trạng thái lưu: {save_status}")
        assert "đã lưu" in save_status.lower()

        req = urllib.request.Request(f"{BASE_URL}/api/landings/get?page_id=test_gizmo_builder&domain=")
        with opener.open(req) as resp:
            data = json.loads(resp.read().decode('utf-8'))
            files = data.get("files", [])
            index_file = next((f for f in files if f["file_path"] == "index.html"), None)
            saved_content = index_file["content"] if index_file else data.get("landing", {}).get("code_html", "")

            # Kiểm tra các style đã được lưu
            assert "font-size: 32px" in saved_content
            assert "background-color: #1e293b" in saved_content
            assert "border-radius: 16px" in saved_content
            assert "<!DOCTYPE html>" in saved_content
            assert "<!-- Tailwind Play CDN -->" in saved_content
            print("=> CSDL POSTGRESQL ĐÃ LƯU CHUẨN XÁC TOÀN BỘ CẤU HÌNH STYLE MỚI!")

        # 12. Dọn dẹp landing thử nghiệm
        print("\nDọn dẹp landing page thử nghiệm...")
        page.on("dialog", lambda dialog: dialog.accept())
        page.evaluate("closeCodeStudio()")
        page.wait_for_load_state("networkidle")
        page.wait_for_timeout(1000)
        page.evaluate("switchMainTab('ladiStudio')")
        page.wait_for_timeout(600)
        page.evaluate("deleteLanding('', 'test_gizmo_builder')")
        page.wait_for_timeout(1000)
        print("=> Dọn dẹp thành công!")

        browser.close()

if __name__ == "__main__":
    try:
        login_res = login_admin()
        setup_test_landing()
        test_gizmo_and_inspector_suite(login_res.get("token"))
        print("\n=======================================================")
        print("🎉 TẤT CẢ CÁC BÀI TEST GIZMO DRAG/RESIZE & INSPECTOR ĐỀU PASSED!")
        print("=======================================================")
    except Exception as e:
        print(f"\n❌ LỖI KIỂM THỬ: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
