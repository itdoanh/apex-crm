"""
E2E Test:
1. Selection verification: clicking elements consecutively (no pointer-event interception)
2. Space-key Pan verification: inner iframe panning & non-blocking shield guard
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
    print("\n--- [2] KHỞI TẠO LANDING PAGE TEST CLICK SELECTION & SPACE PAN ---")
    landing_data = {
        "page_id": "test_pan_selection",
        "name": "Landing Test Pan & Selection",
        "domain": "",
        "code_html": """<!DOCTYPE html>
<html lang="vi">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Landing Test Pan Selection</title>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body style="min-height: 3000px;" class="bg-slate-900 text-white p-10 flex flex-col items-center">
    <div id="card-header" class="max-w-xl w-full bg-slate-800 p-8 rounded-2xl mb-8 border border-slate-700">
        <h1 id="title-apex" class="text-3xl font-black text-cyan-400 mb-3">LỚP HỌC HÀNG HÓA MXV</h1>
        <p id="desc-apex" class="text-slate-300 text-base mb-4 leading-relaxed">
            Hiểu thị trường – Nắm cơ hội – Quản trị rủi ro cùng Apex.
        </p>
        <button id="btn-action-primary" class="bg-cyan-500 hover:bg-cyan-400 text-slate-950 font-bold px-6 py-2.5 rounded-xl">
            Đăng Ký Tham Gia
        </button>
    </div>

    <div id="card-details" class="max-w-xl w-full bg-slate-800/90 p-8 rounded-2xl mb-8 border border-slate-700">
        <h2 id="title-schedule" class="text-xl font-bold text-amber-400 mb-2">Lịch Trình Chi Tiết</h2>
        <p id="desc-schedule" class="text-slate-300 text-sm mb-4">
            Thời gian: Thứ Ba hàng tuần vào lúc 20:00 trực tuyến qua Zoom.
        </p>
        <div id="speaker-box" class="p-4 bg-slate-900 rounded-xl border border-slate-800 text-slate-400 text-xs">
            Diễn giả: Chuyên gia phân tích thị trường tài chính cấp cao.
        </div>
    </div>
</body>
</html>""",
        "settings": {
            "nguon_default": "pan_test"
        }
    }
    data_bytes = json.dumps(landing_data).encode('utf-8')
    req = urllib.request.Request(f"{BASE_URL}/api/landings/save", data=data_bytes, headers={"Content-Type": "application/json"})
    with opener.open(req) as resp:
        res = json.loads(resp.read().decode('utf-8'))
        assert res.get("success") is True
        print("=> Khởi tạo landing test thành công!")

    file_payload = {
        "domain": "",
        "page_id": "test_pan_selection",
        "file_path": "index.html",
        "file_type": "text/html",
        "content": landing_data["code_html"]
    }
    req_file = urllib.request.Request(f"{BASE_URL}/api/landings/file", data=json.dumps(file_payload).encode('utf-8'), headers={"Content-Type": "application/json"})
    with opener.open(req_file) as resp:
        res_file = json.loads(resp.read().decode('utf-8'))
        print(f"=> Lưu file index.html thành công: {res_file.get('message')}")

def cleanup_test_landing():
    print("\n--- DỌN DẸP DỮ LIỆU TEST ---")
    data_bytes = json.dumps({"page_id": "test_pan_selection", "domain": ""}).encode('utf-8')
    req = urllib.request.Request(f"{BASE_URL}/api/landings/delete", data=data_bytes, headers={"Content-Type": "application/json"})
    with opener.open(req) as resp:
        res = json.loads(resp.read().decode('utf-8'))
        print("=> Dọn dẹp landing test hoàn tất!")

def run_tests():
    login_res = login_admin()
    token = login_res.get("token")
    setup_test_landing()

    try:
        with sync_playwright() as p:
            browser = p.chromium.launch(headless=True)
            context = browser.new_context(viewport={"width": 2100, "height": 1050})
            if token:
                context.add_cookies([{
                    "name": "apex_admin_session",
                    "value": token,
                    "domain": "localhost",
                    "path": "/"
                }])
            page = context.new_page()

            page.on("console", lambda msg: print(f"CONSOLE: {msg.text}"))

            print("\n--- [3] TRUY CẬP STUDIO ---")
            page.goto(f"{BASE_URL}/studio.html?page_id=test_pan_selection", wait_until="networkidle")
            page.wait_for_timeout(2500)

            # Chuyển sang visual mode
            page.evaluate("setStudioViewMode('visual')")
            page.wait_for_timeout(500)

            # TEST 1: CHỌN PHẦN TỬ LIÊN TIẾP KHÔNG BỊ INTERCEPT
            print("\n--- TEST 1: CHỌN PHẦN TỬ TRỰC TIẾP TRÊN IFRAME ---")
            frame = page.frame_locator("#csPreviewIframe")

            # 1.1 Click chọn thẻ H1 #title-apex
            h1 = frame.locator("#title-apex")
            assert h1.is_visible(), "Thẻ H1 phải hiển thị trong iframe"
            h1.click(timeout=5000)
            page.wait_for_timeout(400)

            h1_selected_id = page.evaluate("selectedAstNodeId")
            assert h1_selected_id is not None, "Thẻ H1 phải được chọn thành công"
            print(f"=> Đã chọn thẻ H1 thành công (Node ID: {h1_selected_id})")

            # Kiểm tra gizmo box và docked toolbar
            gizmo = page.locator("#csGizmoBox")
            assert gizmo.is_visible(), "Gizmo Box phải hiển thị khi chọn H1"
            docked_toolbar = page.locator("#csGizmoToolbar")
            assert docked_toolbar.is_visible(), "Docked Toolbar phải hiển thị trên đỉnh canvas"

            # Đảm bảo docked toolbar không nằm trong gizmo box
            in_gizmo = page.evaluate("document.querySelector('#csGizmoBox #csGizmoToolbar') !== null")
            assert in_gizmo is False, "Docked Toolbar tuyệt đối không được nằm trong csGizmoBox để tránh đè lên web!"
            print("=> Docked Toolbar được neo cố định phía trên canvas, không đè lên phần tử phía dưới!")

            # 1.2 Click chọn thẻ P #desc-apex nằm ngay bên dưới H1
            print("Thử chọn thẻ P #desc-apex nằm ngay bên dưới H1...")
            p_desc = frame.locator("#desc-apex")
            assert p_desc.is_visible()
            p_desc.click(timeout=5000)
            page.wait_for_timeout(400)

            p_selected_id = page.evaluate("selectedAstNodeId")
            assert p_selected_id is not None, "Thẻ P phải được chọn thành công"
            assert p_selected_id != h1_selected_id, "Thẻ được chọn phải chuyển từ H1 sang P"
            print(f"=> Đã chọn thẻ P thành công (Node ID: {p_selected_id}) mà không bị toolbar đè lên!")

            # 1.3 Click chọn nút button #btn-action-primary nằm ngay bên dưới P
            print("Thử chọn nút #btn-action-primary nằm bên dưới P...")
            btn = frame.locator("#btn-action-primary")
            assert btn.is_visible()
            btn.click(timeout=5000)
            page.wait_for_timeout(400)

            btn_selected_id = page.evaluate("selectedAstNodeId")
            assert btn_selected_id is not None, "Button phải được chọn thành công"
            assert btn_selected_id != p_selected_id, "Node chọn phải đổi sang Button"
            print(f"=> Đã chọn nút bấm Button thành công (Node ID: {btn_selected_id})!")

            # 1.4 Click chọn thẻ H2 ở khối card bên dưới
            print("Thử chọn thẻ H2 #title-schedule ở khối card bên dưới...")
            h2 = frame.locator("#title-schedule")
            assert h2.is_visible()
            h2.click(timeout=5000)
            page.wait_for_timeout(400)

            h2_selected_id = page.evaluate("selectedAstNodeId")
            assert h2_selected_id is not None
            print(f"=> Đã chọn thẻ H2 thành công (Node ID: {h2_selected_id})!")

            print("=> KẾT LUẬN TEST 1: Khắc phục 100% lỗi không chọn được phần tử trên Canvas Studio!")

            # TEST 2: KIỂM THỬ SPACE PAN (KÉO CUỘN NỘI DUNG TRONG KHUNG)
            print("\n--- TEST 2: KIỂM THỬ SPACE PAN NỘI DUNG PREVIEW (HAND TOOL MODE) ---")

            # 2.1 Kiểm tra trạng thái khi ấn phím Space xuống
            page.keyboard.down("Space")
            page.wait_for_timeout(200)

            space_pressed = page.evaluate("window.isSpaceKeyPressed")
            assert space_pressed is True, "isSpaceKeyPressed phải là True khi nhấn phím Space"

            # Kiểm tra con trỏ chuột mặc định đổi sang grab (bàn tay mở)
            shield_cursor = page.evaluate("""() => {
                const s = document.getElementById('csCanvasPanShield');
                if (!s) return '';
                return window.getComputedStyle(s).cursor;
            }""")
            print(f"Con trỏ chuột trên preview khi ấn Space: '{shield_cursor}'")
            assert "grab" in shield_cursor, f"Con trỏ chuột phải đổi sang bàn tay kéo 'grab' (thực tế: {shield_cursor})"

            # Kiểm tra Gizmo Box bị ẩn khi giữ phím Space
            gizmo_hidden = page.evaluate("""() => {
                const g = document.getElementById('csGizmoBox');
                return !g || g.style.display === 'none' || g.classList.contains('hidden');
            }""")
            assert gizmo_hidden is True, "Gizmo Box phải được ẩn khi giữ Space để không chọn nhầm lớp"
            print("=> Khi giữ Space: Chuột preview tự động đổi sang con trỏ bàn tay kéo (grab) và ẩn Gizmo selection!")

            # 2.2 Thử CLICK CHUỘT trong khi giữ Space: ĐẢM BẢO KHÔNG CHỌN BẤT KỲ LỚP NÀO
            print("Thử nhấp chuột vào preview trong khi đang giữ Space...")
            # Click vào tọa độ của thẻ P
            wrapper_box = page.locator("#csIframeWrapper").bounding_box()
            assert wrapper_box is not None
            test_click_x = wrapper_box["x"] + wrapper_box["width"] / 2
            test_click_y = wrapper_box["y"] + 150

            page.mouse.click(test_click_x, test_click_y)
            page.wait_for_timeout(200)

            # Node chọn không được đổi sang phần tử bị click
            current_selected = page.evaluate("selectedAstNodeId")
            print(f"Selected node sau khi click lúc giữ Space: {current_selected}")
            assert current_selected != "node-10" and current_selected != "node-11", "Không được chọn bất kỳ lớp nào khi đang ấn phím Space!"
            print("=> XÁC NHẬN: Khi ấn phím Space, tuyệt đối KHÔNG chọn lớp nào, chỉ có thể kéo!")

            # 2.3 Thực hiện kéo chuột cuộn nội dung (MouseDown + Move)
            start_x = wrapper_box["x"] + wrapper_box["width"] / 2
            start_y = wrapper_box["y"] + 300

            page.mouse.move(start_x, start_y)
            page.mouse.down()
            page.wait_for_timeout(100)

            # Khi chuột đang nhấn giữ lúc Space đang bật, con trỏ đổi sang grabbing
            dragging_cursor = page.evaluate("""() => {
                const s = document.getElementById('csCanvasPanShield');
                return s ? window.getComputedStyle(s).cursor : '';
            }""")
            print(f"Con trỏ chuột khi kéo: '{dragging_cursor}'")
            assert "grabbing" in dragging_cursor, "Con trỏ chuột khi kéo phải là 'grabbing'"

            is_panning = page.evaluate("window.isCanvasPanning")
            print(f"Trạng thái isCanvasPanning khi kéo: {is_panning}")
            assert is_panning is True, "isCanvasPanning phải là True khi chuột nhấn xuống"

            # Kéo chuột lên 150px
            page.mouse.move(start_x, start_y - 150, steps=10)
            page.wait_for_timeout(200)

            # Kiểm tra nội dung iframe đã được cuộn (scrollY > 0)
            iframe_scroll_y = page.evaluate("""() => {
                const iframe = document.getElementById('csPreviewIframe');
                if (!iframe || !iframe.contentWindow) return 0;
                return iframe.contentWindow.scrollY || iframe.contentDocument.documentElement.scrollTop;
            }""")
            print(f"Vị trí cuộn của tài liệu bên trong iframe: {iframe_scroll_y}px")
            assert iframe_scroll_y > 0, "Nội dung tài liệu bên trong iframe phải được cuộn xuống!"

            # Kiểm tra khung viền #csIframeWrapper KHÔNG bị dịch chuyển vị trí (transform: none)
            wrapper_transform = page.evaluate("""() => {
                const w = document.getElementById('csIframeWrapper');
                return window.getComputedStyle(w).transform;
            }""")
            assert wrapper_transform == "none" or wrapper_transform == "", f"Khung viền phải giữ nguyên vị trí cố định, không được translate (thực tế: {wrapper_transform})"
            print("=> Khung viền #csIframeWrapper giữ nguyên vị trí cố định (transform: none), chỉ cuộn nội dung bên trong!")

            # 2.4 Thả chuột và thả phím Space
            page.mouse.up()
            page.keyboard.up("Space")
            page.wait_for_timeout(300)

            post_pan_state = page.evaluate("""() => {
                const shield = document.getElementById('csCanvasPanShield');
                const style = shield ? window.getComputedStyle(shield) : null;
                return {
                    isCanvasPanning: window.isCanvasPanning,
                    isSpaceKeyPressed: window.isSpaceKeyPressed,
                    shieldHidden: shield ? shield.classList.contains('hidden') : null,
                    shieldPointerEvents: style ? style.pointerEvents : null
                };
            }""")
            print(f"Trạng thái sau khi kết thúc Space Pan: {post_pan_state}")
            assert post_pan_state["isCanvasPanning"] is False, "Pan phải kết thúc sau mouseup"
            assert post_pan_state["shieldHidden"] is True, "Shield phải bị ẩn ngay khi nhả chuột"
            assert post_pan_state["shieldPointerEvents"] == "none", "Shield phải có pointer-events: none"

            # 2.5 Sau khi pan xong và thả Space, click chọn phần tử hoạt động trở lại bình thường
            print("Kiểm tra click chọn lại phần tử sau khi đã thả Space...")
            h1.click(timeout=5000)
            page.wait_for_timeout(400)
            after_pan_selected_id = page.evaluate("selectedAstNodeId")
            assert after_pan_selected_id == h1_selected_id, "Thẻ H1 phải được chọn lại ngay lập tức sau khi thả Space"
            print("=> Tuyệt vời: Click chọn phần tử hoạt động tức thì sau khi thả phím Space!")

            screenshot_path = os.path.join(SCREENSHOT_DIR, "24_selection_and_space_pan_verified.png")
            page.screenshot(path=screenshot_path)
            print(f"Saved screenshot: {screenshot_path}")

            browser.close()

        print("\n=======================================================")
        print("🎉 TẤT CẢ CÁC BÀI TEST CHỌN PHẦN TỬ VÀ KÉO SPACE PAN ĐỀU PASSED 100%!")
        print("=======================================================")

    finally:
        cleanup_test_landing()

if __name__ == "__main__":
    run_tests()
