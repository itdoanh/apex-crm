import sys
import os
import json
import urllib.request
import urllib.error
import http.cookiejar
import time

from playwright.sync_api import sync_playwright

BASE_URL = "http://localhost:8080"
cookie_jar = http.cookiejar.CookieJar()
opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(cookie_jar))

def login():
    login_data = json.dumps({"username": "admin", "password": "ApexAdmin@2026!"}).encode('utf-8')
    req = urllib.request.Request(f"{BASE_URL}/api/admin/login", data=login_data, headers={"Content-Type": "application/json"})
    with opener.open(req) as resp:
        return json.loads(resp.read().decode('utf-8'))

def create_landing():
    landing_data = {
        "page_id": "test_full_drag_resize",
        "name": "Landing Full Drag Resize",
        "domain": "",
        "code_html": """<!DOCTYPE html>
<html lang="vi">
<head>
    <meta charset="UTF-8">
    <title>Full Drag Resize</title>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-slate-900 text-white p-10" style="min-height: 2500px;">
    <div id="test-card" class="w-80 bg-slate-800 p-6 rounded-xl border border-slate-700">
        <h1 id="test-title" class="text-2xl font-bold text-cyan-400 mb-4">TIÊU ĐỀ TEST</h1>
        <p id="test-desc" class="text-slate-300 mb-4">Đoạn văn bản thử nghiệm kéo thả và co giãn.</p>
        <button id="test-btn" class="bg-cyan-500 text-slate-950 font-bold px-4 py-2 rounded">Bấm Vào Đây</button>
    </div>
</body>
</html>""",
        "settings": {"nguon_default": "test"}
    }
    req = urllib.request.Request(f"{BASE_URL}/api/landings/save", data=json.dumps(landing_data).encode('utf-8'), headers={"Content-Type": "application/json"})
    opener.open(req)
    req_file = urllib.request.Request(f"{BASE_URL}/api/landings/file", data=json.dumps({
        "domain": "", "page_id": "test_full_drag_resize", "file_path": "index.html", "file_type": "text/html", "content": landing_data["code_html"]
    }).encode('utf-8'), headers={"Content-Type": "application/json"})
    opener.open(req_file)

def cleanup():
    req = urllib.request.Request(f"{BASE_URL}/api/landings/delete", data=json.dumps({"page_id": "test_full_drag_resize", "domain": ""}).encode('utf-8'), headers={"Content-Type": "application/json"})
    try: opener.open(req)
    except: pass

def run_tests():
    login_res = login()
    token = login_res.get("token")
    create_landing()
    try:
        with sync_playwright() as p:
            browser = p.chromium.launch(headless=True)
            context = browser.new_context(viewport={"width": 1920, "height": 1080})
            if token:
                context.add_cookies([{"name": "apex_admin_session", "value": token, "domain": "localhost", "path": "/"}])
            page = context.new_page()
            page.goto(f"{BASE_URL}/studio.html?page_id=test_full_drag_resize")
            page.wait_for_timeout(2000)
            page.evaluate("setStudioViewMode('visual')")
            page.wait_for_timeout(500)

            frame = page.frame_locator("#csPreviewIframe")
            btn = frame.locator("#test-btn")
            btn.click()
            page.wait_for_timeout(400)

            print("======================================================================")
            print("TEST 1: KÉO DI CHUYỂN LIÊN TIẾP 3 LẦN (100% ZOOM) - CHỐNG ĐÓNG BĂNG")
            print("======================================================================")
            b0 = btn.bounding_box()

            # Lần 1: Kéo +60, +30
            header = page.locator("#csGizmoHeader")
            h_box = header.bounding_box()
            page.mouse.move(h_box["x"] + 20, h_box["y"] + 10)
            page.mouse.down()
            page.mouse.move(h_box["x"] + 20 + 60, h_box["y"] + 10 + 30, steps=5)
            page.wait_for_timeout(100)
            b_during1 = btn.bounding_box()
            print(f"Lần 1 - Trong khi kéo: dx={b_during1['x'] - b0['x']}, dy={b_during1['y'] - b0['y']}")
            assert abs((b_during1['x'] - b0['x']) - 60) <= 2, "Lần 1 di chuyển phải bám sát chuột"
            page.mouse.up()
            page.wait_for_timeout(300)
            b_after1 = btn.bounding_box()
            print(f"Lần 1 - Sau khi thả:   dx={b_after1['x'] - b0['x']}, dy={b_after1['y'] - b0['y']}")
            assert abs((b_after1['x'] - b0['x']) - 60) <= 2, "Lần 1 sau khi thả phải giữ nguyên vị trí"

            # Lần 2: Kéo tiếp +40, +20
            h_box = page.locator("#csGizmoHeader").bounding_box()
            page.mouse.move(h_box["x"] + 20, h_box["y"] + 10)
            page.mouse.down()
            page.mouse.move(h_box["x"] + 20 + 40, h_box["y"] + 10 + 20, steps=5)
            page.wait_for_timeout(100)
            b_during2 = btn.bounding_box()
            print(f"Lần 2 - Trong khi kéo: dx={b_during2['x'] - b_after1['x']}, dy={b_during2['y'] - b_after1['y']}")
            assert abs((b_during2['x'] - b_after1['x']) - 40) <= 2, "Lần 2 KHÔNG ĐƯỢC ĐÓNG BĂNG (phải di chuyển theo chuột)!"
            page.mouse.up()
            page.wait_for_timeout(300)
            b_after2 = btn.bounding_box()
            print(f"Lần 2 - Sau khi thả:   dx={b_after2['x'] - b_after1['x']}, dy={b_after2['y'] - b_after1['y']}")
            assert abs((b_after2['x'] - b_after1['x']) - 40) <= 2, "Lần 2 sau khi thả giữ nguyên"

            # Lần 3: Kéo tiếp -30, -10
            h_box = page.locator("#csGizmoHeader").bounding_box()
            page.mouse.move(h_box["x"] + 20, h_box["y"] + 10)
            page.mouse.down()
            page.mouse.move(h_box["x"] + 20 - 30, h_box["y"] + 10 - 10, steps=5)
            page.wait_for_timeout(100)
            b_during3 = btn.bounding_box()
            print(f"Lần 3 - Trong khi kéo: dx={b_during3['x'] - b_after2['x']}, dy={b_during3['y'] - b_after2['y']}")
            assert abs((b_during3['x'] - b_after2['x']) - (-30)) <= 2, "Lần 3 kéo mượt mà"
            page.mouse.up()
            page.wait_for_timeout(300)
            b_after3 = btn.bounding_box()
            print(f"Lần 3 - Sau khi thả:   dx={b_after3['x'] - b_after2['x']}, dy={b_after3['y'] - b_after2['y']}")
            print("=> TEST 1 PASSED: Kéo liên tiếp 3 lần hoàn toàn chính xác, không bị giật hay đóng băng!")

            print("\n======================================================================")
            print("TEST 2: CO GIÃN HƯỚNG TÂY (W HANDLE) - CẠNH TRÁI DI CHUYỂN, CẠNH PHẢI CỐ ĐỊNH")
            print("======================================================================")
            b_before_w = btn.bounding_box()
            right_edge_before = b_before_w["x"] + b_before_w["width"]

            # Kéo chốt W sang trái 50px (chuột di chuyển -50px)
            w_handle = page.locator(".cs-handle-w")
            w_box = w_handle.bounding_box()
            page.mouse.move(w_box["x"] + w_box["width"]/2, w_box["y"] + w_box["height"]/2)
            page.mouse.down()
            page.mouse.move(w_box["x"] + w_box["width"]/2 - 50, w_box["y"] + w_box["height"]/2, steps=5)
            page.wait_for_timeout(100)
            b_during_w = btn.bounding_box()
            right_edge_during = b_during_w["x"] + b_during_w["width"]
            print(f"W Resize - Cạnh trái dịch: dx={b_during_w['x'] - b_before_w['x']} (kỳ vọng: -50)")
            print(f"W Resize - Chiều rộng tăng: dw={b_during_w['width'] - b_before_w['width']} (kỳ vọng: +50)")
            print(f"W Resize - Cạnh phải lệch: dRight={right_edge_during - right_edge_before} (kỳ vọng: ~0)")
            assert abs((b_during_w['x'] - b_before_w['x']) - (-50)) <= 3, "Cạnh trái phải dịch sang trái theo chuột"
            assert abs((b_during_w['width'] - b_before_w['width']) - 50) <= 3, "Chiều rộng phải tăng tương ứng"
            assert abs(right_edge_during - right_edge_before) <= 3, "Cạnh phải phải giữ nguyên vị trí"

            page.mouse.up()
            page.wait_for_timeout(300)
            b_after_w = btn.bounding_box()
            print(f"W Resize sau thả - dw={b_after_w['width'] - b_before_w['width']}, dx={b_after_w['x'] - b_before_w['x']}")
            assert abs((b_after_w['width'] - b_before_w['width']) - 50) <= 3
            print("=> TEST 2 PASSED: Kéo chốt hướng Tây (W) co dãn chính xác, cạnh phải neo cố định!")

            print("\n======================================================================")
            print("TEST 3: KÉO VÀ CO GIÃN TẠI TỶ LỆ ZOOM 60% (canvasZoom = 0.60)")
            print("======================================================================")
            page.evaluate("setCanvasZoom(0.60)")
            page.wait_for_timeout(400)

            card = frame.locator("#test-card")
            card.click(position={"x": 5, "y": 5})
            page.wait_for_timeout(300)
            sel_tag = page.evaluate("selectedNodeMeta ? selectedNodeMeta.tagName : ''")
            print(f"Selected element in Test 3: {sel_tag}")
            c0 = card.bounding_box()

            # Kéo card +120px trên màn hình
            h_box = page.locator("#csGizmoHeader").bounding_box()
            page.mouse.move(h_box["x"] + 20, h_box["y"] + 10)
            page.mouse.down()
            page.mouse.move(h_box["x"] + 20 + 120, h_box["y"] + 10 + 60, steps=5)
            page.wait_for_timeout(100)
            c_during_drag = card.bounding_box()
            print(f"Drag at 60% - Card dx={c_during_drag['x'] - c0['x']}, dy={c_during_drag['y'] - c0['y']} (kỳ vọng: +120, +60)")
            assert abs((c_during_drag['x'] - c0['x']) - 120) <= 3, "Khi zoom 60%, kéo 120px màn hình thì phần tử phải dịch đúng 120px"
            assert abs((c_during_drag['y'] - c0['y']) - 60) <= 3

            page.mouse.up()
            page.wait_for_timeout(300)
            c_after_drag = card.bounding_box()
            assert abs((c_after_drag['x'] - c0['x']) - 120) <= 3, "Sau khi thả không được giật lại"
            print("Kéo tại 60% zoom thành công bám sát chuột!")

            # Resize SE handle tại zoom 60% (+90px width, +50px height trên màn hình)
            se_handle = page.locator(".cs-handle-se")
            se_box = se_handle.bounding_box()
            page.mouse.move(se_box["x"] + se_box["width"]/2, se_box["y"] + se_box["height"]/2)
            page.mouse.down()
            page.mouse.move(se_box["x"] + se_box["width"]/2 + 90, se_box["y"] + se_box["height"]/2 + 50, steps=5)
            page.wait_for_timeout(100)
            c_during_res = card.bounding_box()
            print(f"Resize at 60% - Card dw={c_during_res['width'] - c_after_drag['width']}, dh={c_during_res['height'] - c_after_drag['height']} (kỳ vọng: +90, +50)")
            assert abs((c_during_res['width'] - c_after_drag['width']) - 90) <= 3, "Co dãn tại zoom 60% phải ăn khớp 100% với độ kéo chuột"
            assert abs((c_during_res['height'] - c_after_drag['height']) - 50) <= 3

            page.mouse.up()
            page.wait_for_timeout(300)
            c_after_res = card.bounding_box()
            assert abs((c_after_res['width'] - c_after_drag['width']) - 90) <= 3
            print("=> TEST 3 PASSED: Thu phóng canvas 60% hoàn toàn ăn khớp pixel-for-pixel với chuột!")

            print("\n======================================================================")
            print("TEST 4: KÉO VÀ CO GIÃN TẠI TỶ LỆ ZOOM 140% (canvasZoom = 1.40)")
            print("======================================================================")
            page.evaluate("setCanvasZoom(1.40)")
            page.wait_for_timeout(400)
            btn.click()
            page.wait_for_timeout(300)
            btn_z140_0 = btn.bounding_box()

            # Kéo +70, +35 trên màn hình
            h_box = page.locator("#csGizmoHeader").bounding_box()
            page.mouse.move(h_box["x"] + 20, h_box["y"] + 10)
            page.mouse.down()
            page.mouse.move(h_box["x"] + 20 + 70, h_box["y"] + 10 + 35, steps=5)
            page.wait_for_timeout(100)
            btn_z140_during = btn.bounding_box()
            print(f"Drag at 140% - Btn dx={btn_z140_during['x'] - btn_z140_0['x']}, dy={btn_z140_during['y'] - btn_z140_0['y']} (kỳ vọng: +70, +35)")
            assert abs((btn_z140_during['x'] - btn_z140_0['x']) - 70) <= 3

            page.mouse.up()
            page.wait_for_timeout(300)
            btn_z140_after = btn.bounding_box()
            assert abs((btn_z140_after['x'] - btn_z140_0['x']) - 70) <= 3
            print("=> TEST 4 PASSED: Phóng to canvas 140% kéo di chuyển chính xác 100%!")

            browser.close()
            print("\n🎉🎉🎉 TẤT CẢ CÁC BÀI TEST KÉO VÀ CO GIÃN (RESIZE/ZOOM) ĐỀU THÀNH CÔNG 100%!")
    finally:
        cleanup()

if __name__ == "__main__":
    run_tests()
