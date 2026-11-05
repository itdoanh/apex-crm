import urllib.request

HOST = "ads.giaodichhanghoatuonglai.com"
BASE = "http://127.0.0.1:8080"

CASES = [
    ("/landing/", "/landing/tracker.js"),
    ("/landing", "/landing/tracker.js"),
    ("/", "/tracker.js"),
    ("/p/landing_cu", "/p/landing_cu/tracker.js"),
]

results = []


def check(name, ok, detail=""):
    results.append((name, ok, detail))
    print(f"  [{'PASS' if ok else 'FAIL'}] {name}" + (f" :: {detail}" if detail else ""))


for path, expect in CASES:
    req = urllib.request.Request(BASE + path, headers={"Host": HOST})
    try:
        with urllib.request.urlopen(req, timeout=20) as r:
            html = r.read().decode("utf-8", "replace")
            status = r.status
    except Exception as e:
        print(f"\n{BASE}{path} -> ERR {e}")
        check(f"[{path}] load duoc", False, str(e))
        continue

    import re

    srcs = re.findall(r'<script[^>]*src="([^"]*tracker\.js)"', html)
    print(f"\n{BASE}{path}  status={status}  len={len(html)}")
    print(f"   tracker.js src = {srcs}")
    check(f"[{path}] co tham chieu tracker.js", len(srcs) > 0, f"srcs={srcs}")
    check(
        f"[{path}] tracker.js tro tuyet doi dung duong dan",
        any(expect in s for s in srcs),
        f"can '{expect}', thuc te={srcs}",
    )
    check(
        f"[{path}] co data-landing-id",
        'data-landing-id="' in html,
    )

print("\n" + "=" * 62)
passed = sum(1 for _, ok, _ in results if ok)
total = len(results)
print(f">> KET QUA: {passed}/{total} assertions pass")
for name, ok, detail in results:
    if not ok:
        print(f"   FAIL: {name} :: {detail}")
print("=" * 62)
