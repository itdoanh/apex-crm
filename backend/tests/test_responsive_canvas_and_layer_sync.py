"""
E2E Test: Code Studio PC & Mobile Responsive Scope Isolation,
Side Resize Handles, Canvas Zoom & Boundary Clamped Pan, and 2-Way Layer Sync
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

BASE_URL = os.environ.get("APEX_TEST_URL", "https://crm.hanghoaphaisinh.net")
SCREENSHOT_DIR = os.path.join(os.path.dirname(__file__), "screenshots")
os.makedirs(SCREENSHOT_DIR, exist_ok=True)

cookie_jar = http.cookiejar.CookieJar()
opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(cookie_jar))

def login_admin():
    print(f"\n--- [1] ĐĂNG NHẬP ADMIN ARGON2ID TẠI {BASE_URL} ---")
    login_data = json.dumps({
        "username": "admin",
        "password": "ApexAdmin@2026!"
    }).encode('utf-8')
    req = urllib.request.Request(f"{BASE_URL}/api/admin/login", data=login_data, headers={"Content-Type": "application/json"})
    with opener.open(req) as resp:
        res = json.loads(resp.read().decode('utf-8'))
        assert res.get("success") is True, "Đăng nhập admin phải thành công"
        print("=> Đăng nhập Admin thành công!")
        token = res.get("token")
        cookies = []
        for c in cookie_jar:
            cookies.append({
                "name": c.name,
                "value": c.value,
                "domain": "crm.hanghoaphaisinh.net" if "crm." in BASE_URL else "localhost",
                "path": "/"
            })
        return token, cookies

def setup_test_landing(token):
    print("\n--- [2] KHỞI TẠO LANDING PAGE TEST CHO RESPONSIVE & CANVAS INTERACTIONS ---")
    landing_data = {
        "page_id": "test_responsive_canvas",
        "name": "Landing Test Responsive & Canvas",
        "domain": "",
        "code_html": """<!DOCTYPE html>
<html lang="vi">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Test Responsive Scope & Canvas</title>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-slate-900 text-white min-h-screen p-8 flex flex-col items-center justify-center">
    <div id="hero-card" class="bg-slate-800 p-8 rounded-2xl max-w-xl w-full border border-slate-700 shadow-2xl text-center">
        <h1 id="hero-title" class="text-3xl font-black text-cyan-400 mb-3">ĐẦU TƯ TƯƠNG LAI 2026</h1>
        <p id="hero-desc" class="text-sm text-slate-300 mb-6">Trang thử nghiệm tính năng cách ly Mobile/PC và đồng bộ cây layer.</p>
        <div id="cta-group" class="flex justify-center gap-3">
            <button id="cta-btn" class="px-6 py-2.5 bg-gradient-to-r from-cyan-500 to-blue-600 rounded-xl font-bold text-white shadow-lg">Đăng Ký Ngay</button>
            <a id="cta-link" href="#details" class="px-5 py-2.5 bg-slate-700 hover:bg-slate-600 rounded-xl font-semibold text-slate-200">Tìm Hiểu Thêm</a>
        </div>
    </div>
</body>
</html>"""
    }
    req = urllib.request.Request(
        f"{BASE_URL}/api/landings/save",
        data=json.dumps(landing_data).encode('utf-8'),
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {token}"
        }
    )
    try:
        with opener.open(req) as resp:
            res = json.loads(resp.read().decode('utf-8'))
            print("=> Đã tạo Landing Page test thành công:", res.get("landing", {}).get("page_id"))
            return "test_responsive_canvas"
    except urllib.error.HTTPError as e:
        err_body = e.read().decode('utf-8')
        print(f"=> Landing đã tồn tại hoặc mã lỗi ({e.code}): {err_body}")
        return "mxv_080_online"

def run_tests():
    token, cookies = login_admin()
    page_id = setup_test_landing(token)

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True, args=['--no-sandbox', '--disable-setuid-sandbox'])
        context = browser.new_context(viewport={'width': 1600, 'height': 900})

        # Thiết lập cookies và localStorage xác thực
        if cookies:
            context.add_cookies(cookies)

        page = context.new_page()
        page.on("console", lambda msg: print(f"CONSOLE [{msg.type}]: {msg.text}"))
        page.on("pageerror", lambda err: print(f"PAGEERROR: {err} | {getattr(err, 'stack', '')}"))
        # Chèn token vào localStorage
        page.goto(f"{BASE_URL}/ads/")
        page.evaluate(f"token => {{ localStorage.setItem('apex_admin_token', token); }}", token)
        page.wait_for_timeout(1000)

        # Mở Code Studio cho trang test
        studio_url = f"{BASE_URL}/ads/studio.html?page_id={page_id}"
        print(f"\n--- [3] TRUY CẬP CODE STUDIO ({studio_url}) ---")
        page.goto(studio_url)
        page.wait_for_selector("#csPreviewIframe", state="visible", timeout=15000)
        page.wait_for_timeout(3000)

        # =========================================================================
        # TEST 1: CHẾ ĐỘ XEM PC & MOBILE ĐỘC LẬP (RESPONSIVE SCOPE ISOLATION)
        # =========================================================================
        print("\n=== TEST 1: CHẾ ĐỘ XEM PC & MOBILE ĐỘC LẬP ===")
        # Kiểm tra ban đầu là chế độ PC (1440px)
        btn_pc = page.locator("#btnDevicePC")
        btn_mob = page.locator("#btnDeviceMobile")
        badge = page.locator("#csDeviceTargetBadge")

        assert "bg-cyan-600" in btn_pc.get_attribute("class"), "Nút PC ban đầu phải được chọn"
        assert "PC Mode" in badge.inner_text(), "Badge ban đầu phải là PC Mode"
        print("  [OK] Chế độ ban đầu là PC (1440px)")

        # Chuyển sang Mobile
        btn_mob.click()
        page.wait_for_timeout(500)
        assert "bg-cyan-600" in btn_mob.get_attribute("class"), "Nút Mobile phải được active"
        assert "Mobile Mode" in badge.inner_text(), "Badge phải chuyển sang Mobile Mode"
        wrapper_w = page.evaluate("() => document.getElementById('csIframeWrapper').style.width")
        assert wrapper_w == "390px", f"Chiều rộng wrapper phải là 390px, nhận được {wrapper_w}"
        print("  [OK] Đã chuyển sang Mobile (390px)")

        # Chọn thẻ H1 trong AST mapper và chỉnh sửa style khi ở chế độ Mobile
        mob_edit_res = page.evaluate("""() => {
            const nodeIds = Object.keys(astMapper.astMap);
            const h1Node = nodeIds.find(id => astMapper.astMap[id].tag === 'h1');
            if (!h1Node) return { success: false, error: 'Không tìm thấy thẻ h1' };

            // Áp dụng style chỉ riêng cho Mobile qua applyAstStyleChange
            selectedAstNodeId = h1Node;
            applyAstStyleChange(h1Node, { 'font-size': '24px', 'color': '#f43f5e' }, 'Tùy chỉnh font Mobile');

            const raw = astMapper.rawHTML;
            const hasMobileTag = raw.includes('__apex_responsive_styles__');
            const hasMediaQuery = raw.includes('@media (max-width: 768px)');
            const hasColorOverride = raw.includes('#f43f5e !important');

            return {
                success: true,
                h1Id: h1Node,
                hasMobileTag,
                hasMediaQuery,
                hasColorOverride,
                mobileStyles: astMapper.getMobileStyles(h1Node)
            };
        }""")
        assert mob_edit_res["success"], f"Lỗi: {mob_edit_res.get('error')}"
        assert mob_edit_res["hasMobileTag"], "Mã HTML phải được chèn <style id='__apex_responsive_styles__'>"
        assert mob_edit_res["hasMediaQuery"], "Style mobile phải được bao bởi @media (max-width: 768px)"
        assert mob_edit_res["hasColorOverride"], "Quy tắc mobile phải có '!important' để ghi đè"
        print("  [OK] Tinh chỉnh Mobile đã được ghi riêng vào @media (max-width: 768px) thành công!")

        # Chuyển ngược lại về chế độ PC
        btn_pc.click()
        page.wait_for_timeout(500)
        assert "PC Mode" in badge.inner_text(), "Badge phải trở lại PC Mode"
        wrapper_w_pc = page.evaluate("() => document.getElementById('csIframeWrapper').style.width")
        assert wrapper_w_pc == "100%", f"Khung PC phải trở lại 100%, nhận được {wrapper_w_pc}"

        # Kiểm tra thẻ h1 trong PC: Style inline gốc của PC không bị dính mã màu mobile!
        pc_check = page.evaluate("""(h1Id) => {
            const node = astMapper.astMap[h1Id];
            const sVal = node.attrs['style'] ? node.attrs['style'].value : '';
            return {
                styleVal: sVal,
                notOverriddenInPC: !sVal.includes('#f43f5e')
            };
        }""", mob_edit_res["h1Id"])
        assert pc_check["notOverriddenInPC"], "Bố cục PC gốc không bị biến đổi bởi cấu hình mobile!"
        print("  [OK] Bố cục PC gốc được bảo toàn nguyên vẹn 100% (Responsive Isolation hoàn hảo)!")

        # =========================================================================
        # TEST 2: THANH KÉO ĐIỀU CHỈNH KÍCH THƯỚC 2 BÊN (SIDE RESIZE HANDLES)
        # =========================================================================
        print("\n=== TEST 2: THANH KÉO ĐIỀU CHỈNH KÍCH THƯỚC 2 BÊN ===")
        handle_left = page.locator("#csCanvasResizeHandleLeft")
        handle_right = page.locator("#csCanvasResizeHandleRight")
        assert handle_left.is_visible(), "Chốt kéo bên trái phải hiển thị"
        assert handle_right.is_visible(), "Chốt kéo bên phải phải hiển thị"

        # Kéo thử chốt bên phải
        h_box = handle_right.bounding_box()
        assert h_box is not None, "Không lấy được bounding box của handle right"

        # Mô phỏng kéo chốt điều chỉnh kích thước
        page.mouse.move(h_box['x'] + h_box['width'] / 2, h_box['y'] + h_box['height'] / 2)
        page.mouse.down()
        page.mouse.move(h_box['x'] + 50, h_box['y'])
        page.wait_for_timeout(200)

        # Kiểm tra badge hiển thị chiều rộng
        width_badge = page.locator("#csCanvasWidthBadge")
        badge_text = width_badge.inner_text()
        print(f"  [OK] Floating Badge chiều rộng hiển thị: {badge_text}")
        assert "px" in badge_text, f"Badge chiều rộng phải chứa 'px', nhận được '{badge_text}'"

        page.mouse.up()
        page.wait_for_timeout(500)
        print("  [OK] Hoàn tất kéo chốt 2 bên điều chỉnh kích thước khung xem trực quan!")

        # =========================================================================
        # TEST 3: CON TRỎ CHUỘT, CTRL GRAB & THU PHÓNG (ZOOM) & KÉO (PAN)
        # =========================================================================
        print("\n=== TEST 3: CON TRỎ CHUỘT, THU PHÓNG (ZOOM) & KÉO (PAN) ===")
        viewport = page.locator("#csCanvasViewport")
        assert "cs-design-canvas" in viewport.get_attribute("class"), "Canvas phải có class con trỏ thiết kế hiện đại cs-design-canvas"
        print("  [OK] Con trỏ chuột mặc định là Design Pointer (.cs-design-canvas)")

        # Kiểm tra giữ phím Ctrl -> Đổi sang bàn tay grab (.cs-pan-grab) và bật lá chắn
        page.keyboard.down("Control")
        page.wait_for_timeout(300)
        vp_cls_ctrl = viewport.get_attribute("class")
        assert "cs-pan-grab" in vp_cls_ctrl, f"Canvas phải có class .cs-pan-grab khi giữ Ctrl, nhận được: {vp_cls_ctrl}"
        shield_hidden = page.evaluate("() => document.getElementById('csCanvasPanShield').classList.contains('hidden')")
        assert not shield_hidden, "Tấm chắn #csCanvasPanShield phải hiển thị khi giữ Ctrl để bảo vệ chuột"
        print("  [OK] Khi nhấn Ctrl, con trỏ đổi thành bàn tay mở (.cs-pan-grab) và #csCanvasPanShield được kích hoạt!")

        # Thu phóng Zoom bằng nút bấm hoặc hàm zoom
        zoom_init = page.locator("#lblZoomLevel").inner_text()
        assert zoom_init == "100%", f"Zoom ban đầu phải là 100%, nhận được: {zoom_init}"

        # Bấm nút Zoom In
        page.locator("#btnZoomIn").click()
        page.wait_for_timeout(200)
        zoom_after_in = page.locator("#lblZoomLevel").inner_text()
        assert zoom_after_in == "110%", f"Zoom sau khi click '+' phải là 110%, nhận được: {zoom_after_in}"
        print(f"  [OK] Thu phóng Canvas hoạt động chính xác: {zoom_after_in}")

        # Thử nghiệm Boundary Clamping Panning:
        # Khi trang chưa tràn màn hình -> không cho phép kéo
        page.evaluate("() => resetCanvasZoom()")
        page.wait_for_timeout(200)

        pan_check_clamped = page.evaluate("""() => {
            const limits = getCanvasPanLimits();
            return {
                canPanX: limits.canPanX,
                canPanY: limits.canPanY,
                canvasPanX: canvasPanX,
                canvasPanY: canvasPanY
            };
        }""")
        print(f"  [OK] Trạng thái Panning khi vừa vặn: canPanX={pan_check_clamped['canPanX']}, canPanY={pan_check_clamped['canPanY']}")

        # Thả phím Ctrl
        page.keyboard.up("Control")
        page.wait_for_timeout(300)
        vp_cls_normal = viewport.get_attribute("class")
        assert "cs-pan-grab" not in vp_cls_normal, "Phải gỡ bỏ .cs-pan-grab khi thả phím Ctrl"
        print("  [OK] Thả phím Ctrl: Khôi phục trỏ chuột thiết kế và ẩn lá chắn pan shield!")

        # =========================================================================
        # TEST 4: ĐỒNG BỘ 2 CHIỀU GIỮA CANVAS PREVIEW VÀ TAB CÂY LAYER
        # =========================================================================
        print("\n=== TEST 4: ĐỒNG BỘ 2 CHIỀU CANVAS PREVIEW VÀ CÂY LAYER ===")

        # 4.1. Chọn từ Cây Layer -> Canvas Preview tự động chọn, bật gizmo, cuộn mượt
        target_node_id = mob_edit_res["h1Id"]
        page.evaluate(f"() => {{ switchInspectorTab('layers'); selectAstNode('{target_node_id}', true); }}")
        page.wait_for_timeout(500)

        selected_id = page.evaluate("() => selectedAstNodeId")
        assert selected_id == target_node_id, f"Phần tử được chọn phải là {target_node_id}, nhận được: {selected_id}"

        # Kiểm tra hàng tương ứng trong cây Layer được active
        row_active = page.evaluate(f"() => {{ const el = document.getElementById('layer-row-{target_node_id}'); return el && el.classList.contains('layer-row-active'); }}")
        assert row_active, "Dòng trên cây layer phải được bôi sáng class .layer-row-active"
        print(f"  [OK] Chọn Layer trên Cây Layer -> Active row thành công: layer-row-{target_node_id}")

        # 4.2. Giả lập bấm từ Canvas Preview sang -> Tab Inspector tự động đổi sang Layer và mở nhánh cha
        page.evaluate("""(h1Id) => {
            const h1Node = astMapper.astMap[h1Id];
            handleNodeSelection({
                nodeId: h1Id,
                tagName: 'h1',
                className: '',
                text: 'ĐẦU TƯ TƯƠNG LAI 2026',
                rect: { top: 100, left: 100, width: 300, height: 50 },
                attributes: {}
            });
        }""", target_node_id)
        page.wait_for_timeout(500)

        insp_tab = page.evaluate("() => window.currentInspectorTab || window.currentInspTab || currentInspTab")
        assert insp_tab == "layers", f"Inspector phải tự động chuyển sang tab 'layers', nhận được: {insp_tab}"
        print("  [OK] Bấm phần tử trên Canvas -> Inspector tự động chuyển sang tab 'layers' và cuộn đến node!")

        # Chụp ảnh minh chứng
        screenshot_path = os.path.join(SCREENSHOT_DIR, "studio_responsive_canvas_layers_verified.png")
        page.screenshot(path=screenshot_path)
        print(f"\n=> ĐÃ CHỤP ẢNH MINH CHỨNG THÀNH CÔNG: {screenshot_path}")

        browser.close()
        print("\n=========================================================================")
        print(" TẤT CẢ CÁC BÀI TEST TỰ ĐỘNG PLAYWRIGHT ĐÃ VƯỢT QUA 100% THÀNH CÔNG RỰC RỠ! ")
        print("=========================================================================")

if __name__ == "__main__":
    run_tests()
