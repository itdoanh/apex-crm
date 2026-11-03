"""
Test Apex Visual Web Builder (Non-Destructive AST Instrumentation & Overlay Canvas Isolation)
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
    print("\n--- [1] ĐĂNG NHẬP ADMIN BẰNG ARGON2ID ---")
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
    print("\n--- [2] KHỞI TẠO LANDING PAGE THỬ NGHIỆM CHO VISUAL BUILDER ---")
    landing_data = {
        "page_id": "test_visual_builder",
        "name": "Landing Test Visual Web Builder",
        "domain": "",
        "code_html": """<!DOCTYPE html>
<html lang="vi">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Landing Test Visual Web Builder</title>
    <!-- Tailwind Play CDN -->
    <script src="https://cdn.tailwindcss.com"></script>
    <style>
        .custom-card { border-radius: 16px; transition: transform 0.2s; }
    </style>
</head>
<body class="bg-slate-900 text-white min-h-screen flex flex-col items-center justify-center p-6">
    <div id="main-container" class="max-w-2xl w-full bg-slate-800 p-8 rounded-2xl shadow-xl text-center border border-slate-700">
        <h1 id="hero-title" class="text-3xl font-extrabold text-cyan-400 mb-4">LỚP HỌC ONLINE ĐẦU TƯ MXV</h1>
        <p id="hero-desc" class="text-slate-300 text-base mb-6 leading-relaxed">
            Khóa học trực tuyến đặc biệt giúp bạn nắm bắt cơ hội tài chính an toàn và minh bạch.
        </p>
        <div class="flex items-center justify-center gap-4 mb-6">
            <a id="btn-cta" href="#dang-ky" class="bg-cyan-500 hover:bg-cyan-400 text-slate-950 font-bold px-6 py-3 rounded-xl shadow-lg transition">
                Đăng Ký Ngay
            </a>
            <img id="partner-logo" src="https://via.placeholder.com/120x40" alt="Logo MXV" class="h-10 rounded" />
        </div>
        <!-- Thẻ ephemeral mô phỏng thư viện thứ 3 -->
        <div id="footer-note" class="text-xs text-slate-500">
            © 2026 Apex Financial. Bảo lưu mọi quyền.
        </div>
    </div>
</body>
</html>""",
        "settings": {
            "nguon_default": "visual_builder_test"
        }
    }

    req = urllib.request.Request(f"{BASE_URL}/api/landings/save", data=json.dumps(landing_data).encode('utf-8'), headers={"Content-Type": "application/json"})
    with opener.open(req) as resp:
        res = json.loads(resp.read().decode('utf-8'))
        print(f"=> Tạo landing thành công: {res.get('message')}")

    # Lưu nội dung file index.html
    file_payload = {
        "domain": "",
        "page_id": "test_visual_builder",
        "file_path": "index.html",
        "file_type": "text/html",
        "content": landing_data["code_html"]
    }
    req_file = urllib.request.Request(f"{BASE_URL}/api/landings/file", data=json.dumps(file_payload).encode('utf-8'), headers={"Content-Type": "application/json"})
    with opener.open(req_file) as resp:
        res_file = json.loads(resp.read().decode('utf-8'))
        print(f"=> Lưu file index.html ban đầu thành công: {res_file.get('message')}")

def test_visual_builder_e2e(admin_token=None):
    print("\n--- [3] KIỂM THỬ PLAYWRIGHT E2E: APEX VISUAL WEB BUILDER ---")
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        context = browser.new_context(viewport={"width": 1600, "height": 960})

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
        page.on("pageerror", lambda err: print(f"PAGE ERROR: {err}"))
        page.on("console", lambda msg: print(f"CONSOLE: {msg.text}"))

        # 1. Truy cập admin /ads
        print("Truy cập trang Quản lý Admin: http://localhost:8080/ads ...")
        page.goto(f"{BASE_URL}/ads", wait_until="domcontentloaded")
        if admin_token:
            page.evaluate(f"localStorage.setItem('apex_admin_token', '{admin_token}')")
            page.reload(wait_until="networkidle")
        else:
            page.wait_for_load_state("networkidle")
        page.wait_for_timeout(1000)

        # 2. Mở Tab Quản lý Landing (Ladi Studio)
        print("Mở Tab Ladi Studio...")
        page.evaluate("switchMainTab('ladiStudio')")
        page.wait_for_timeout(800)

        # 3. Mở Code Studio cho landing 'test_visual_builder'
        print("Mở Apex Visual Code Studio...")
        page.evaluate("openCodeStudio('', 'test_visual_builder')")
        page.wait_for_timeout(1500)

        # Kiểm tra Modal Code Studio hiển thị
        studio_modal = page.locator("#codeStudioModal")
        assert studio_modal.is_visible(), "Modal Code Studio / Visual Web Builder phải hiển thị"
        print("=> Modal Visual Web Builder đã hiển thị thành công!")

        # 4. Kiểm tra AST Source Mapper đã parse thành công
        ast_nodes_count = page.evaluate("Object.keys(astMapper.astMap).length")
        print(f"=> Số lượng AST Nodes được parse: {ast_nodes_count}")
        assert ast_nodes_count > 0, "AST Source Mapper phải phân tích được các node HTML"

        # 5. Chụp ảnh Chế độ Split View (Code ↔ Visual Preview)
        shot_split = os.path.join(SCREENSHOT_DIR, "20_visual_builder_split_mode.png")
        page.screenshot(path=shot_split)
        print(f"Saved screenshot: {shot_split}")

        # 6. Kiểm tra các chế độ xem (View Modes): Code Only, Split, Visual Builder
        print("Kiểm tra chuyển sang chế độ Toàn Màn Hình Visual Builder ('visual')...")
        page.evaluate("setStudioViewMode('visual')")
        page.wait_for_timeout(600)

        editor_container = page.locator("#csEditorContainer")
        preview_panel = page.locator("#csPreviewPanel")
        inspector_panel = page.locator("#csVisualInspector")

        assert not editor_container.is_visible(), "Editor Container phải ẩn khi ở chế độ visual"
        assert preview_panel.is_visible(), "Preview Panel phải hiển thị khi ở chế độ visual"
        assert inspector_panel.is_visible(), "Inspector Panel phải hiển thị khi ở chế độ visual"
        print("=> Chuyển sang Visual Builder toàn màn hình thành công!")

        shot_visual = os.path.join(SCREENSHOT_DIR, "21_visual_builder_fullscreen.png")
        page.screenshot(path=shot_visual)
        print(f"Saved screenshot: {shot_visual}")

        # 7. Thao tác chọn phần tử trong Sandboxed Iframe (Click vào node)
        print("Thử nghiệm chọn phần tử #hero-title trong preview...")
        # Tìm nodeId tương ứng với h1 trong astMap
        h1_node_id = page.evaluate("""() => {
            for (let id in astMapper.astMap) {
                if (astMapper.astMap[id].tag === 'h1') return id;
            }
            return null;
        }""")
        print(f"=> Tìm thấy H1 Node ID trong AST: {h1_node_id}")
        assert h1_node_id is not None, "Phải tìm thấy thẻ h1 trong AST"

        # Kích hoạt chọn node h1
        page.evaluate(f"selectAstNode('{h1_node_id}')")
        page.wait_for_timeout(600)

        # Kiểm tra Gizmo Box và 8 Chốt Handles
        gizmo_box = page.locator("#csGizmoBox")
        assert gizmo_box.is_visible(), "Gizmo Box phải hiển thị khi phần tử được chọn"
        gizmo_tag = page.locator("#csGizmoTag").inner_text()
        print(f"=> Gizmo Box Tag hiển thị: {gizmo_tag}")
        assert "H1" in gizmo_tag, "Gizmo Tag phải chứa chữ H1"

        # Kiểm tra 8 chốt Handles
        handles_count = page.locator(".cs-gizmo-handle").count()
        print(f"=> Số lượng Gizmo Handles: {handles_count}")
        assert handles_count == 8, f"Phải có đủ 8 chốt co giãn (hiện có {handles_count})"

        # 8. Kiểm tra Bảng Thuộc Tính Visual Property Inspector (4 Tab)
        print("Kiểm tra 4 Tab trong Visual Property Inspector...")
        tab_tag_badge = page.locator("#inspTagBadge").inner_text()
        print(f"=> Inspector Tag Badge: {tab_tag_badge}")
        assert tab_tag_badge.lower() == "h1", "Badge phải là h1"

        # Kiểm tra Tab Content & Attr
        text_content_val = page.locator("#inspTextContent").input_value()
        print(f"=> Giá trị Text ban đầu: '{text_content_val}'")
        assert "LỚP HỌC ONLINE" in text_content_val, "Nội dung text của H1 phải chính xác"

        # 9. Kiểm thử Non-Destructive String Splicing: SỬA VĂN BẢN TRỰC TIẾP
        print("Sửa văn bản H1 thành 'LỚP HỌC ONLINE APEX MXV 2026' thông qua AST Splicer...")
        page.fill("#inspTextContent", "LỚP HỌC ONLINE APEX MXV 2026")
        page.evaluate("onInspectorTextInput()")
        page.wait_for_timeout(500)

        # Kiểm tra code trong Editor
        updated_code = page.evaluate("document.getElementById('csCodeEditor').value")
        assert "LỚP HỌC ONLINE APEX MXV 2026" in updated_code, "Mã nguồn phải được cập nhật text mới qua AST Splicer"
        assert "<!DOCTYPE html>" in updated_code, "Doctype vẫn phải được bảo toàn nguyên vẹn"
        assert "<!-- Tailwind Play CDN -->" in updated_code, "Comments gốc vẫn phải được bảo toàn 100%"
        print("=> XÁC NHẬN: Sửa văn bản Non-Destructive hoàn tất, giữ nguyên 100% comment và doctype!")

        # 10. Kiểm thử Tab Classes & Tailwind Resolver: Thêm Class Tailwind Pill
        print("Chuyển sang Tab 2: 'Classes' trong Inspector...")
        page.evaluate("switchInspectorTab('classes')")
        page.wait_for_timeout(400)

        # Thêm class mới: 'tracking-wider' và 'uppercase'
        print("Thêm class Tailwind 'uppercase' qua Quick Class...")
        page.evaluate("addQuickClass('uppercase')")
        page.wait_for_timeout(400)

        updated_classes = page.evaluate(f"astMapper.astMap['{h1_node_id}'].classes")
        print(f"=> Danh sách class sau khi thêm: {updated_classes}")
        assert "uppercase" in updated_classes, "Class 'uppercase' phải có trong AST Node classes"

        # 11. Kiểm thử Tab Styles & Layout
        print("Chuyển sang Tab 3: 'Styles' trong Inspector...")
        page.evaluate("switchInspectorTab('styles')")
        page.wait_for_timeout(400)

        print("Đổi màu chữ sang '#f59e0b'...")
        page.evaluate("onInspectorStyleChange('color', '#f59e0b')")
        page.wait_for_timeout(400)

        style_in_code = page.evaluate("document.getElementById('csCodeEditor').value")
        assert "color: #f59e0b" in style_in_code, "Thuộc tính style inline phải được ghi chính xác"
        print("=> XÁC NHẬN: Style inline cập nhật mượt mà!")

        # 12. Kiểm thử Tab Layers (DOM Tree View)
        print("Chuyển sang Tab 4: 'Layers' (Cây DOM)...")
        page.evaluate("switchInspectorTab('layers')")
        page.wait_for_timeout(400)

        tree_items = page.locator("#inspLayerTreeContainer > div")
        print(f"=> Số lượng node hiển thị trên Layer Tree: {tree_items.count()}")
        assert tree_items.count() > 0, "Layer Tree phải hiển thị các node DOM"

        # 13. Kiểm thử Freeze Mode vs Live Interactive Mode Toggle
        print("Kiểm tra Toggle Freeze Mode...")
        freeze_text_before = page.locator("#textToggleFreeze").inner_text()
        print(f"Trạng thái Freeze ban đầu: {freeze_text_before}")
        assert "Freeze" in freeze_text_before

        page.evaluate("toggleStudioFreezeMode()")
        page.wait_for_timeout(400)
        freeze_text_after = page.locator("#textToggleFreeze").inner_text()
        print(f"Trạng thái sau toggle: {freeze_text_after}")
        assert "Live" in freeze_text_after

        # Bật lại Freeze Mode
        page.evaluate("toggleStudioFreezeMode()")
        page.wait_for_timeout(400)

        # 14. Chụp ảnh có Gizmo Box & Inspector đang active
        shot_inspector = os.path.join(SCREENSHOT_DIR, "22_visual_builder_inspector_active.png")
        page.screenshot(path=shot_inspector)
        print(f"Saved screenshot: {shot_inspector}")

        # 15. Kiểm tra Lưu Mã Nguồn (Save Code) & Deploy
        print("Thực hiện Lưu Mã Nguồn vào CSDL qua API /api/landings/file...")
        page.evaluate("saveStudioCode()")
        page.wait_for_timeout(1200)

        save_status = page.locator("#csSaveStatus").inner_text()
        print(f"=> Trạng thái lưu: {save_status}")
        assert "đã lưu" in save_status.lower(), "Trạng thái phải báo đã lưu"

        # 16. Xác thực CSDL lưu đúng bản mã có chứa các sửa đổi
        print("Kiểm tra CSDL PostgreSQL qua API /api/landings/get...")
        req = urllib.request.Request(f"{BASE_URL}/api/landings/get?page_id=test_visual_builder&domain=")
        with opener.open(req) as resp:
            data = json.loads(resp.read().decode('utf-8'))
            files = data.get("files", [])
            index_file = next((f for f in files if f["file_path"] == "index.html"), None)
            saved_content = index_file["content"] if index_file else data.get("landing", {}).get("code_html", "")

            assert "LỚP HỌC ONLINE APEX MXV 2026" in saved_content, "CSDL phải lưu văn bản mới"
            assert "uppercase" in saved_content, "CSDL phải lưu class mới"
            assert "<!DOCTYPE html>" in saved_content, "Doctype trong CSDL phải nguyên vẹn"
            assert "<!-- Tailwind Play CDN -->" in saved_content, "Comment trong CSDL phải nguyên vẹn"
            print("=> CSDL POSTGRESQL LƯU CHUẨN XÁC 100% MÃ NGUỒN KHÔNG PHÁ HỦY!")

        # 17. Dọn dẹp landing thử nghiệm
        print("\nDọn dẹp landing page test...")
        page.on("dialog", lambda dialog: dialog.accept())
        page.evaluate("closeCodeStudio()")
        page.wait_for_load_state("networkidle")
        page.wait_for_timeout(1000)
        page.evaluate("deleteLanding('', 'test_visual_builder')")
        page.wait_for_timeout(1000)
        print("=> Đã dọn dẹp sạch sẽ landing thử nghiệm!")

        browser.close()

if __name__ == "__main__":
    try:
        login_res = login_admin()
        setup_test_landing()
        test_visual_builder_e2e(login_res.get("token"))
        print("\n=======================================================")
        print("🎉 TẤT CẢ CÁC BÀI TEST VISUAL WEB BUILDER ĐỀU THÀNH CÔNG (PASSED)!")
        print("=======================================================")
    except Exception as e:
        print(f"\n❌ LỖI KIỂM THỬ: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
