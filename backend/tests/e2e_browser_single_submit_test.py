"""
E2E browser (Chromium thật) — kiểm tra 1 lần bấm nút đăng ký sinh ĐÚNG 1 request
POST /api/leads và KHÔNG sinh bản ghi nào trong spam_quarantine.

Bắt được 2 lớp bug:
  1. tracker.js 404 (do <base> đặt sai chỗ) ⇒ window.ApexTracker không tồn tại.
  2. tracker.js cũ (thiếu LEAD_DEDUP_WINDOW_MS) ⇒ 1 lần bấm sinh 3 request
     ⇒ lead thật bị ghi vào spam_quarantine ("Lỗi Proof-of-Work").

Cách chạy:
  python backend/tests/e2e_browser_single_submit_test.py
"""

import os
import re
import subprocess
import sys
import time

BASE = os.environ.get("APEX_TEST_BASE", "http://127.0.0.1:8080")
LANDING_PATH = os.environ.get("APEX_TEST_LANDING", "/p/landing_cu")
PHONE = os.environ.get("APEX_TEST_PHONE", "0905" + str(int(time.time()) % 1000000).zfill(6))
NAME = "Tran Thi E2E"

DOCKER = os.environ.get("APEX_PG_CONTAINER", "apex-postgres")
DB = os.environ.get("APEX_PG_DB", "apex_leads")
DBUSER = os.environ.get("APEX_PG_USER", "postgres")

results = []


def check(name, ok, detail=""):
    results.append((name, ok, detail))
    print(f"  [{'PASS' if ok else 'FAIL'}] {name}" + (f" :: {detail}" if detail else ""))
    return ok


def psql(sql):
    out = subprocess.check_output(
        ["docker", "exec", DOCKER, "psql", "-U", DBUSER, "-d", DB, "-A", "-t", "-c", sql],
        text=True,
        stderr=subprocess.DEVNULL,
    )
    return out.strip().splitlines()[0].strip() if out.strip() else ""


def count_leads(phone):
    return int(psql(f"SELECT COUNT(*) FROM leads WHERE so_dien_thoai = '{phone}'") or 0)


def count_quarantine(phone):
    return int(psql(f"SELECT COUNT(*) FROM spam_quarantine WHERE phone = '{phone}'") or 0)


def main():
    try:
        from playwright.sync_api import sync_playwright
    except ImportError:
        print("!! Thieu playwright. Cai: pip install playwright && playwright install chromium")
        return 2

    leads_before = count_leads(PHONE)
    quar_before = count_quarantine(PHONE)
    print(f">> Phone test: {PHONE} | leads truoc={leads_before} quarantine truoc={quar_before}")
    print(f">> Landing   : {BASE}{LANDING_PATH}\n")

    url = BASE + LANDING_PATH
    lead_requests = []
    tracker_hits = []

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        ctx = browser.new_context(
            user_agent=(
                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "
                "(KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
            ),
            viewport={"width": 1366, "height": 900},
            locale="vi-VN",
        )
        page = ctx.new_page()

        def on_request(req):
            if "/api/leads" in req.url:
                body = req.post_data or ""
                m = re.search(r'"challenge_id"\s*:\s*"([^"]+)"', body)
                lead_requests.append(
                    {"url": req.url, "method": req.method, "challenge": m.group(1) if m else None}
                )

        def on_response(resp):
            if "tracker.js" in resp.url:
                tracker_hits.append((resp.status, resp.url))

        page.on("request", on_request)
        page.on("response", on_response)

        print(">> [1] Mo landing")
        page.goto(url, wait_until="load", timeout=45000)
        page.wait_for_timeout(1500)

        print(">> [2] Kiem tra tracker.js da nap")
        has_tracker = page.evaluate("() => !!window.ApexTracker")
        check("tracker.js da nap (window.ApexTracker ton tai)", has_tracker)
        for st, u in tracker_hits:
            print(f"     tracker.js -> {st} {u}")
        check(
            "tracker.js tra ve HTTP 200",
            any(st == 200 for st, _ in tracker_hits),
            f"hits={tracker_hits}",
        )

        guard = page.evaluate("() => window.__REGISTRATION_FORM_HANDLED__ === true")
        check("Form guard (__REGISTRATION_FORM_HANDLED__) da bat", guard)

        dedup = page.evaluate(
            "() => !!(window.ApexTracker && window.ApexTracker.sendLeadToBackend)"
        )
        check("ApexTracker.sendLeadToBackend co san", dedup)

        # Xoá cờ "đã đăng ký" để không bị auto-redirect sang chucmung
        ctx.clear_cookies()
        page.evaluate(
            "() => { try { localStorage.removeItem('apex_lead_registered');"
            " localStorage.removeItem('apex_registered_at'); } catch (e) {} }"
        )

        print(">> [3] Gõ tay vao form (mo phong nguoi that)")
        name_sel = "#fullName"
        phone_sel = "#phoneNumber"
        if page.locator(name_sel).count() == 0:
            print("!! Khong tim thay #fullName tren landing nay")
            browser.close()
            return 1

        page.click(name_sel)
        for ch in NAME:
            page.type(name_sel, ch, delay=60)
        time.sleep(1.2)
        page.click(phone_sel)
        for ch in PHONE:
            page.type(phone_sel, ch, delay=60)
        time.sleep(1.2)

        print(">> [4] Bam nut DANG KY (1 lan duy nhat)")
        clicked = page.evaluate(
            """() => {
                const f = document.getElementById('registrationForm');
                if (!f) return 'NO_FORM';
                const btn = f.querySelector('button[type="submit"]') || f.querySelector('button');
                if (!btn) return 'NO_BTN';
                btn.click();
                return 'CLICKED';
            }"""
        )
        check("Da bam nut dang ky", clicked == "CLICKED", clicked)

        # Cho đủ thời gian để mọi request trùng lặp (nếu có) đều kịp gửi
        time.sleep(5.0)
        browser.close()

    print("\n>> [5] Ket qua request")
    for r in lead_requests:
        print(f"     - {r['method']} {r['url']} challenge={r['challenge']}")
    check(
        "Chi 1 request POST /api/leads duy nhat",
        len(lead_requests) == 1,
        f"thuc te={len(lead_requests)}",
    )

    leads_after = count_leads(PHONE)
    quar_after = count_quarantine(PHONE)

    print("\n>> [6] Kiem tra database")
    print(f"     leads[{PHONE}]:      {leads_before} -> {leads_after}")
    print(f"     quarantine[{PHONE}]: {quar_before} -> {quar_after}")
    check(
        "Lead that da duoc ghi vao DB",
        leads_after - leads_before == 1,
        f"them={leads_after - leads_before}",
    )
    check(
        "Khong co ban ghi nao vao vung cach ly",
        quar_after - quar_before == 0,
        f"them={quar_after - quar_before}",
    )

    print("\n" + "=" * 62)
    passed = sum(1 for _, ok, _ in results if ok)
    total = len(results)
    print(f">> KET QUA: {passed}/{total} assertions pass")
    for name, ok, detail in results:
        if not ok:
            print(f"   FAIL: {name} :: {detail}")
    print("=" * 62)
    return 0 if passed == total else 1


if __name__ == "__main__":
    sys.exit(main())
