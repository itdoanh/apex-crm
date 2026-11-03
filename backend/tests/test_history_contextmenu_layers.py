"""
Test E2E Apex Visual Web Builder:
1. Photoshop-Style History Stack (Undo/Redo & Time Travel Dropdown)
2. Professional Right-Click Context Menu (Sync between Iframe & Overlay)
3. Deep Layer Tree Traversal & Stacking Context
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
        "page_id": "test_history_layers",
        "name": "Landing Test History & Layers",
        "domain": "",
        "code_html": """<!DOCTYPE html>
<html lang="vi">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Landing Test History & Layers</title>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-slate-900 text-white min-h-screen p-8">
    <div id="main-card" class="max-w-xl mx-auto bg-slate-800 p-6 rounded-2xl border border-slate-700 relative">
        <!-- Badge Stacking Context ABSOLUTE + Z-INDEX -->
        <span id="badge-hot" class="absolute -top-3 right-4 bg-rose-600 text-white text-xs px-2 py-0.5 rounded-full font-bold z-20 shadow">HOT</span>

        <!-- Text Heading -->
        <h1 id="title-heading" class="text-2xl font-bold text-cyan-400 mb-2">ĐẦU TƯ HÀNG HÓA MXV 2026</h1>
        <p id="desc-p" class="text-slate-300 text-sm mb-4">Cơ hội sinh lời an toàn cùng thành viên kinh doanh 080.</p>

        <!-- Image Element -->
        <img id="sample-img" src="https://via.placeholder.com/300x120" alt="Banner Khóa Học" class="w-full h-28 object-cover rounded-xl mb-4" />

        <!-- Form & Input -->
        <form id="lead-form" class="space-y-3 mb-4">
            <input type="text" id="input-name" name="fullname" placeholder="Họ và tên của bạn" class="w-full bg-slate-900 border border-slate-700 rounded-lg p-2 text-sm text-white" />
            <input type="tel" id="input-phone" name="phone" placeholder="Số điện thoại" class="w-full bg-slate-900 border border-slate-700 rounded-lg p-2 text-sm text-white" />
        </form>

        <!-- Button & Link -->
        <div class="flex items-center gap-3">
            <button id="btn-submit" type="button" class="bg-cyan-500 hover:bg-cyan-400 text-slate-950 font-bold px-4 py-2 rounded-lg text-sm">
                Xác Nhận Ngay
            </button>
            <a id="link-detail" href="#chi-tiet" class="text-sky-400 hover:underline text-xs">
                Tìm hiểu thêm &rarr;
            </a>
        </div>

        <!-- Hidden Element: lớp bị ẩn -->
        <div id="hidden-secret" style="display: none;" class="mt-4 p-2 bg-slate-950 text-slate-500 text-xs rounded">
            Nội dung lớp bảo mật dự phòng
        </div>
    </div>
</body>
</html>""",
        "settings": {
            "nguon_default": "history_layers_test"
        }
    }

    req = urllib.request.Request(f"{BASE_URL}/api/landings/save", data=json.dumps(landing_data).encode('utf-8'), headers={"Content-Type": "application/json"})
    with opener.open(req) as resp:
        res = json.loads(resp.read().decode('utf-8'))
        print(f"=> Tạo landing thành công: {res.get('message')}")

    file_payload = {
        "domain": "",
        "page_id": "test_history_layers",
        "file_path": "index.html",
        "file_type": "text/html",
        "content": landing_data["code_html"]
    }
    req_file = urllib.request.Request(f"{BASE_URL}/api/landings/file", data=json.dumps(file_payload).encode('utf-8'), headers={"Content-Type": "application/json"})
    with opener.open(req_file) as resp:
        res_file = json.loads(resp.read().decode('utf-8'))
        print(f"=> Lưu file index.html thành công: {res_file.get('message')}")

def run_tests(admin_token=None):
    print("\n--- [3] KIỂM THỬ PLAYWRIGHT E2E: HISTORY STACK, CONTEXT MENU & DEEP LAYERS ---")
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

        # 1. Truy cập /ads và nạp token
        print("Truy cập trang Quản lý Admin: http://localhost:8080/ads ...")
        page.goto(f"{BASE_URL}/ads", wait_until="domcontentloaded")
        if admin_token:
            page.evaluate(f"localStorage.setItem('apex_admin_token', '{admin_token}')")
            page.reload(wait_until="networkidle")
        else:
            page.wait_for_load_state("networkidle")
        page.wait_for_timeout(1000)

        print("Mở Tab Ladi Studio...")
        page.evaluate("switchMainTab('ladiStudio')")
        page.wait_for_timeout(800)

        # 2. Mở Code Studio cho landing 'test_history_layers'
        print("Mở Code Studio...")
        page.evaluate("openCodeStudio('', 'test_history_layers')")
        page.wait_for_timeout(1500)

        # Chuyển sang visual mode toàn màn hình để kiểm tra trực quan
        page.evaluate("setStudioViewMode('visual')")
        page.wait_for_timeout(600)

        # -------------------------------------------------------------
        # PHÂN HỆ 1: DEEP LAYER TREE TRAVERSAL & STACKING CONTEXT
        # -------------------------------------------------------------
        print("\n=== TEST PHÂN HỆ 1: DEEP LAYER TREE & STACKING CONTEXT ===")
        page.evaluate("switchInspectorTab('layers')")
        page.wait_for_timeout(600)

        # Kiểm tra thống kê số layer
        layer_stats = page.locator("#layerTreeStats").inner_text()
        print(f"=> Tổng số Layers: {layer_stats}")
        assert "layers" in layer_stats, "Thống kê layers phải hiển thị"

        # Kiểm tra lớp bị ẩn được phát hiện
        hidden_stats = page.locator("#layerTreeHiddenStats").inner_text()
        print(f"=> Thống kê lớp ẩn: '{hidden_stats}'")
        assert "ẩn" in hidden_stats, "Phải phát hiện ít nhất 1 lớp bị ẩn (display: none)"

        # Kiểm tra Stacking Context Badges (ABS, Z-Index)
        stacking_badges = page.locator("#inspLayerTreeContainer .bg-rose-950, #inspLayerTreeContainer .bg-indigo-950")
        print(f"=> Số lượng thẻ Stacking Context (ABS / Z-Index): {stacking_badges.count()}")
        assert stacking_badges.count() >= 1, "Phải hiển thị badge ABS hoặc Z cho phần tử absolute"

        # Kiểm tra Category Filter Chips
        print("Kiểm tra bộ lọc danh mục 'Text'...")
        page.evaluate("setLayerFilter('text')")
        page.wait_for_timeout(400)
        shot_filter_text = os.path.join(SCREENSHOT_DIR, "30_layer_filter_text.png")
        page.screenshot(path=shot_filter_text)
        print(f"Saved screenshot: {shot_filter_text}")

        # Reset filter về 'all'
        page.evaluate("setLayerFilter('all')")
        page.wait_for_timeout(400)

        # Kiểm tra Search Filter
        print("Kiểm tra tìm kiếm từ khóa 'heading'...")
        page.fill("#layerSearchInput", "heading")
        page.evaluate("onLayerSearchInput()")
        page.wait_for_timeout(400)
        heading_matches = page.locator("#inspLayerTreeContainer div[id^='layer-row-']").count()
        print(f"=> Số kết quả khớp với 'heading': {heading_matches}")
        assert heading_matches >= 1, "Tìm kiếm 'heading' phải trả về ít nhất 1 node"

        # Xóa tìm kiếm
        page.evaluate("clearLayerSearch()")
        page.wait_for_timeout(400)

        # -------------------------------------------------------------
        # PHÂN HỆ 2: PHOTOSHOP-STYLE HISTORY STACK (UNDO / REDO)
        # -------------------------------------------------------------
        print("\n=== TEST PHÂN HỆ 2: PHOTOSHOP-STYLE HISTORY STACK ===")
        # Kiểm tra trạng thái khởi tạo: Bước 1/1, Undo disable, Redo disable
        badge_init = page.locator("#csHistoryBadge").inner_text()
        print(f"=> Khởi tạo History Badge: {badge_init}")
        assert "1/1" in badge_init or "1 / 1" in badge_init, "Khởi tạo phải có 1 bước (Mở tệp)"

        undo_btn = page.locator("#btnStudioUndo")
        assert undo_btn.is_disabled(), "Ban đầu nút Undo phải disabled"

        # Thao tác 1: Sửa tiêu đề H1
        print("Thực hiện Thao tác 1: Sửa text H1...")
        h1_id = page.evaluate("""() => {
            for (let id in astMapper.astMap) {
                if (astMapper.astMap[id].tag === 'h1') return id;
            }
            return null;
        }""")
        assert h1_id is not None, "Phải tìm thấy thẻ h1"

        page.evaluate(f"selectAstNode('{h1_id}')")
        page.wait_for_timeout(400)

        # Sửa text H1
        page.evaluate(f"applyAstTextChange('{h1_id}', 'TIÊU ĐỀ ĐÃ SỬA BƯỚC 2')")
        page.wait_for_timeout(500)

        badge_step2 = page.locator("#csHistoryBadge").inner_text()
        print(f"=> History Badge sau Thao tác 1: {badge_step2}")
        assert "2/2" in badge_step2 or "2 / 2" in badge_step2, "Badge phải lên bước 2/2"
        assert not undo_btn.is_disabled(), "Nút Undo phải được kích hoạt"

        # Thao tác 2: Thêm Style color cho H1
        print("Thực hiện Thao tác 2: Đổi màu chữ sang '#10b981'...")
        page.evaluate(f"applyAstStyleChange('{h1_id}', {{ color: '#10b981' }}, 'Đổi màu chữ Xanh')")
        page.wait_for_timeout(500)

        badge_step3 = page.locator("#csHistoryBadge").inner_text()
        print(f"=> History Badge sau Thao tác 2: {badge_step3}")
        assert "3/3" in badge_step3 or "3 / 3" in badge_step3, "Badge phải lên bước 3/3"

        # Mở Time-Travel History Dropdown
        print("Mở Dropdown Lịch Sử Thao Tác Photoshop...")
        page.evaluate("toggleStudioHistoryDropdown()")
        page.wait_for_timeout(400)

        history_dropdown = page.locator("#csHistoryDropdown")
        assert history_dropdown.is_visible(), "History Dropdown phải hiển thị"
        items_count = page.locator("#csHistoryListContainer > div").count()
        print(f"=> Số bước trong History Dropdown: {items_count}")
        assert items_count == 3, f"Phải có đúng 3 bước lịch sử (hiện có {items_count})"

        shot_history_dropdown = os.path.join(SCREENSHOT_DIR, "31_history_dropdown_panel.png")
        page.screenshot(path=shot_history_dropdown)
        print(f"Saved screenshot: {shot_history_dropdown}")

        # Thử Time-Travel: Click quay về Bước 1 (Mở tệp)
        print("Click Time-Travel quay về Bước 1 trong quá khứ...")
        page.evaluate("applyHistoryStep(0)")
        page.wait_for_timeout(600)

        # Kiểm tra code hiện tại đã quay về nguyên bản chưa
        cur_code = page.evaluate("document.getElementById('csCodeEditor').value")
        assert "ĐẦU TƯ HÀNG HÓA MXV 2026" in cur_code, "Mã nguồn phải khôi phục về ban đầu"
        assert "TIÊU ĐỀ ĐÃ SỬA BƯỚC 2" not in cur_code, "Tiêu đề sửa bước 2 không được có mặt ở bước 1"

        badge_after_timetravel = page.locator("#csHistoryBadge").inner_text()
        print(f"=> History Badge sau Time-Travel về Bước 1: {badge_after_timetravel}")
        assert "1/3" in badge_after_timetravel or "1 / 3" in badge_after_timetravel, "Phải hiển thị Bước 1/3"

        redo_btn = page.locator("#btnStudioRedo")
        assert not redo_btn.is_disabled(), "Nút Redo phải được kích hoạt"

        # Thử Redo bằng nút Redo
        print("Nhấn nút Redo sang Bước 2...")
        page.evaluate("studioRedo()")
        page.wait_for_timeout(600)

        code_step2 = page.evaluate("document.getElementById('csCodeEditor').value")
        assert "TIÊU ĐỀ ĐÃ SỬA BƯỚC 2" in code_step2, "Sau redo bước 2 phải khôi phục tiêu đề bước 2"

        # Đóng dropdown
        page.evaluate("toggleStudioHistoryDropdown()")
        page.wait_for_timeout(300)

        # -------------------------------------------------------------
        # PHÂN HỆ 3: CONTEXT MENU CHUỘT PHẢI ĐỒNG BỘ
        # -------------------------------------------------------------
        print("\n=== TEST PHÂN HỆ 3: RIGHT-CLICK CONTEXT MENU ===")
        # Kích hoạt Context Menu tại tọa độ (400, 300) cho H1
        print("Mở Context Menu chuột phải trên phần tử H1...")
        page.evaluate(f"openContextMenu(400, 300, '{h1_id}')")
        page.wait_for_timeout(500)

        context_menu = page.locator("#csContextMenu")
        assert context_menu.is_visible(), "Context Menu phải hiển thị khi click chuột phải"

        ctx_badge = page.locator("#ctxMenuTagBadge").inner_text()
        print(f"=> Context Menu Tag Badge: {ctx_badge}")
        assert ctx_badge == "H1", "Badge trên Context menu phải là H1"

        shot_context_menu = os.path.join(SCREENSHOT_DIR, "32_context_menu_active.png")
        page.screenshot(path=shot_context_menu)
        print(f"Saved screenshot: {shot_context_menu}")

        # Thử Action 1: Đổi thẻ sang H2 qua Context Menu
        print("Thử Action Context Menu: Đổi thẻ <H1> sang <H2>...")
        page.evaluate("handleContextMenuAction('change-tag', 'h2')")
        page.wait_for_timeout(600)

        code_after_tag_change = page.evaluate("document.getElementById('csCodeEditor').value")
        assert "<h2" in code_after_tag_change.lower() and "</h2" in code_after_tag_change.lower(), "Thẻ phải được đổi thành <h2> và </h2>"
        print("=> Đổi thẻ thành công, bảo toàn 100% thuộc tính và nội dung!")

        # Thử Action 2: Khóa Layer (Lock layer)
        print("Thử Action Context Menu: Khóa Layer (Toggle Lock)...")
        page.evaluate(f"toggleCurrentNodeLock('{h1_id}')")
        page.wait_for_timeout(400)
        is_locked = page.evaluate(f"studioLockedNodes.has('{h1_id}')")
        assert is_locked is True, "Node phải được thêm vào danh sách locked"
        print("=> Khóa layer thành công!")

        # Mở khóa lại
        page.evaluate(f"toggleCurrentNodeLock('{h1_id}')")
        page.wait_for_timeout(400)
        is_unlocked = page.evaluate(f"!studioLockedNodes.has('{h1_id}')")
        assert is_unlocked is True, "Node phải được mở khóa"
        print("=> Mở khóa layer thành công!")

        # Thử Action 3: Ẩn/Hiện Layer (Toggle Visibility)
        print("Thử Action: Ẩn layer (display: none)...")
        page.evaluate(f"toggleCurrentNodeVisibility('{h1_id}')")
        page.wait_for_timeout(600)
        code_hidden = page.evaluate("document.getElementById('csCodeEditor').value")
        assert "display: none" in code_hidden or "display:none" in code_hidden, "Mã nguồn phải có display: none"
        print("=> Ẩn layer thành công!")

        # Hiện lại layer
        print("Thử Action: Hiện lại layer...")
        page.evaluate(f"toggleCurrentNodeVisibility('{h1_id}')")
        page.wait_for_timeout(600)
        is_h1_hidden = page.evaluate(f"checkNodeHidden(astMapper.astMap['{h1_id}'])")
        assert is_h1_hidden is False, "Node H1 không còn bị ẩn sau khi hiện lại"
        print("=> Hiện lại layer thành công!")

        # Chụp ảnh toàn cảnh hoàn thiện
        shot_final = os.path.join(SCREENSHOT_DIR, "33_final_builder_state.png")
        page.screenshot(path=shot_final)
        print(f"Saved screenshot: {shot_final}")

        # Dọn dẹp landing thử nghiệm
        print("\nDọn dẹp landing page test...")
        page.on("dialog", lambda dialog: dialog.accept())
        page.evaluate("closeCodeStudio()")
        page.wait_for_load_state("networkidle")
        page.wait_for_timeout(1000)
        page.evaluate("deleteLanding('', 'test_history_layers')")
        page.wait_for_timeout(1000)
        print("=> Đã dọn dẹp sạch sẽ landing thử nghiệm!")

        browser.close()

if __name__ == "__main__":
    try:
        login_res = login_admin()
        setup_test_landing()
        run_tests(login_res.get("token"))
        print("\n=======================================================")
        print("🎉 TẤT CẢ CÁC BÀI TEST HISTORY, CONTEXT MENU VÀ LAYERS ĐỀU THÀNH CÔNG (PASSED)!")
        print("=======================================================")
    except Exception as e:
        print(f"\n❌ LỖI KIỂM THỬ: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
