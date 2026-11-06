"""
E2E READ-ONLY trên production — KHÔNG submit lead, KHÔNG ghi DB, KHÔNG đụng dữ liệu nghiệp vụ.

Kiểm tra 3 fix đã lên production:
  1. tracker.js 200 + có LEAD_DEDUP_WINDOW_MS ở MỌI đường dẫn phục vụ.
  2. HTML landing có đường dẫn tracker.js TUYỆT ĐỐI (không còn src="tracker.js").
  3. HTML landing có cờ __REGISTRATION_FORM_HANDLED__ (form guard).
  4. KHÔNG chồng chéo landing: data-landing-id + APEX_LANDING_CONFIG đúng landing.
"""
import re
import ssl
import urllib.request

HOST = "ads.giaodichhanghoatuonglai.com"
CTX = ssl.create_default_context()
CTX.check_hostname = False
CTX.verify_mode = ssl.CERT_NONE

# (path, expected tracker.js path, expected landing_id, co form dang ky)
CASES = [
    ("/landing", "/landing/tracker.js", "landing_cu", True),
    ("/p/landing_cu", "/p/landing_cu/tracker.js", "landing_cu", True),
    ("/chucmung", "/chucmung/tracker.js", "landing_lznoew", False),
]

results = []


def check(name, ok, detail=""):
    results.append((name, ok, detail))
    print(f"  [{'PASS' if ok else 'FAIL'}] {name}" + (f" :: {detail}" if detail else ""))


def get(url):
    req = urllib.request.Request(url, headers={"User-Agent": "ApexVerify/1.0"})
    with urllib.request.urlopen(req, timeout=25, context=CTX) as r:
        return r.status, r.read().decode("utf-8", "replace")


for path, expect, expect_landing, has_form in CASES:
    url = f"https://{HOST}{path}"
    try:
        status, html = get(url)
    except Exception as e:
        check(f"[{path}] load landing HTML", False, str(e))
        continue

    print(f"\n{url}  status={status}  len={len(html)}")
    srcs = re.findall(r'<script[^>]*src="([^"]*tracker\.js)"', html)
    lid = re.findall(r'data-landing-id="([^"]+)"', html)
    print(f"   tracker.js src = {srcs}")
    print(f"   data-landing-id = {lid}")

    check(f"[{path}] HTML 200", status == 200, f"status={status}")
    check(f"[{path}] tracker.js src TUYET DOI", any(expect in s for s in srcs), f"can '{expect}', thuc te={srcs}")
    check(f"[{path}] khong con src tuong doi", 'src="tracker.js"' not in html)
    check(f"[{path}] data-landing-id dung landing", expect_landing in lid, f"can '{expect_landing}', thuc te={lid}")
    if has_form:
        # Landing có form đăng ký ⇒ BẮT BUỘC có form guard.
        check(f"[{path}] co form guard __REGISTRATION_FORM_HANDLED__", "__REGISTRATION_FORM_HANDLED__" in html)
    else:
        # Trang cảm ơn không có form đăng ký ⇒ KHÔNG cần guard (và không được ép).
        has_reg = "registrationForm" in html
        check(f"[{path}] khong co form dang ky nen khong ep guard", not has_reg or "__REGISTRATION_FORM_HANDLED__" not in html, f"has_registrationForm={has_reg}")

    turl = f"https://{HOST}{expect}"
    try:
        tstatus, tjs = get(turl)
        check(f"[{expect}] tracker.js 200", tstatus == 200, f"status={tstatus}")
        check(f"[{expect}] co LEAD_DEDUP_WINDOW_MS", "LEAD_DEDUP_WINDOW_MS" in tjs, f"len={len(tjs)}")
        check(f"[{expect}] co guard check", "__REGISTRATION_FORM_HANDLED__" in tjs)
        check(f"[{expect}] co autofill-safe initAutofill", "initAutofill" in tjs)
    except Exception as e:
        check(f"[{expect}] tracker.js load", False, str(e))

# config.json phai dung landing_id (khong chong cheo landing)
for path, expect_landing in [("/landing/config.json", "landing_cu"), ("/p/landing_cu/config.json", "landing_cu")]:
    try:
        status, body = get(f"https://{HOST}{path}")
        m = re.search(r'"landing_id"\s*:\s*"([^"]*)"', body)
        check(f"[{path}] config.json landing_id dung", m and m.group(1) == expect_landing, f"thuc te={m.group(1) if m else None}")
    except Exception as e:
        check(f"[{path}] config.json load", False, str(e))

print("\n" + "=" * 62)
passed = sum(1 for _, ok, _ in results if ok)
total = len(results)
print(f">> KET QUA: {passed}/{total} assertions pass")
for name, ok, detail in results:
    if not ok:
        print(f"   FAIL: {name} :: {detail}")
print("=" * 62)
