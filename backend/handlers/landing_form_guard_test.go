package handlers

import (
	"strings"
	"testing"
)

// TestInjectRegistrationFormGuard_SetsFlag: landing TỰ gọi sendLeadToBackend
// cho form registrationForm mà chưa khai báo cờ ⇒ server phải tự chèn cờ.
func TestInjectRegistrationFormGuard_SetsFlag(t *testing.T) {
	html := `<!DOCTYPE html><html><head><title>T</title></head><body>
<form id="registrationForm" onsubmit="handleFormSubmit(event)">
  <input id="fullName"><input id="phoneNumber">
</form>
<script>
document.addEventListener('submit', function (e) {
  window.ApexTracker.sendLeadToBackend({ fullName: 'a', phoneNumber: 'b' });
});
</script>
</body></html>`

	out := injectRegistrationFormGuard(html)

	if !strings.Contains(out, "window.__REGISTRATION_FORM_HANDLED__ = true") {
		t.Fatal("phải chèn cờ __REGISTRATION_FORM_HANDLED__ cho landing tự xử lý form")
	}
	// Cờ phải nằm TRONG <head> để chắc chắn chạy trước tracker.js.
	headEnd := strings.Index(strings.ToLower(out), "</head>")
	flagIdx := strings.Index(out, "window.__REGISTRATION_FORM_HANDLED__ = true")
	if flagIdx == -1 || headEnd == -1 || flagIdx > headEnd {
		t.Fatalf("cờ phải được chèn trong <head> (flagIdx=%d headEnd=%d)", flagIdx, headEnd)
	}
	// HTML gốc không được bị mất nội dung.
	if !strings.Contains(out, `onsubmit="handleFormSubmit(event)"`) {
		t.Fatal("HTML gốc bị mất nội dung")
	}
}

// TestInjectRegistrationFormGuard_KeepsUserFlag: landing đã tự khai báo cờ ⇒
// server KHÔNG được ghi đè (ý định của người dùng được tôn trọng).
func TestInjectRegistrationFormGuard_KeepsUserFlag(t *testing.T) {
	html := `<html><head></head><body>
<form id="registrationForm"></form>
<script>
window.__REGISTRATION_FORM_HANDLED__ = true;
window.ApexTracker.sendLeadToBackend({});
</script></body></html>`

	out := injectRegistrationFormGuard(html)

	n := strings.Count(out, "window.__REGISTRATION_FORM_HANDLED__ = true")
	if n != 1 {
		t.Fatalf("cờ đã có sẵn phải giữ nguyên 1 lần, thực tế = %d", n)
	}
	if strings.Contains(out, "[APEX FORM GUARD]") {
		t.Fatal("không được chèn guard khi landing đã tự khai báo cờ")
	}
}

// TestInjectRegistrationFormGuard_NoSelfSubmit: landing KHÔNG tự gọi
// sendLeadToBackend ⇒ không chèn cờ (tracker.js phải tự gửi lead).
func TestInjectRegistrationFormGuard_NoSelfSubmit(t *testing.T) {
	html := `<html><head></head><body>
<form id="registrationForm"><input id="phoneNumber"></form>
</body></html>`

	out := injectRegistrationFormGuard(html)

	if strings.Contains(out, "__REGISTRATION_FORM_HANDLED__") {
		t.Fatal("landing không tự gửi lead thì KHÔNG được đặt cờ")
	}
}

// TestInjectRegistrationFormGuard_NoRegistrationForm: landing có gọi
// sendLeadToBackend nhưng KHÔNG phải form registrationForm ⇒ không chèn cờ.
func TestInjectRegistrationFormGuard_NoRegistrationForm(t *testing.T) {
	html := `<html><head></head><body>
<form id="otherForm"></form>
<script>window.ApexTracker.sendLeadToBackend({});</script>
</body></html>`

	out := injectRegistrationFormGuard(html)

	if strings.Contains(out, "__REGISTRATION_FORM_HANDLED__") {
		t.Fatal("không phải form registrationForm thì KHÔNG được đặt cờ")
	}
}

// TestInjectRegistrationFormGuard_NoHeadTag: HTML không có <head> ⇒ vẫn phải
// chèn cờ (đặt trước nội dung) để không mất tác dụng bảo vệ.
func TestInjectRegistrationFormGuard_NoHeadTag(t *testing.T) {
	html := `<div><form id="registrationForm"></form>
<script>window.ApexTracker.sendLeadToBackend({});</script></div>`

	out := injectRegistrationFormGuard(html)

	if !strings.Contains(out, "window.__REGISTRATION_FORM_HANDLED__ = true") {
		t.Fatal("HTML không có <head> vẫn phải chèn cờ")
	}
}
