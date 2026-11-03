/* ==============================================================
   APEX VISUAL WEB BUILDER - FORM AUTO-SCANNER & DYNAMIC MAPPING
   ============================================================== */

        // ============================================================
        // 17.8.2. BỘ QUÉT FORM TỰ ĐỘNG & ÁNH XẠ TRƯỜNG DỮ LIỆU (FORM AUTO-SCANNER)
        // ============================================================
        let scannedFormFieldsList = [];

        async function scanLandingFormFields() {
            const btn = document.getElementById('btnScanForm');
            const statusEl = document.getElementById('scanFormStatus');
            const pageId = document.getElementById('cfgPageId').value.trim();
            const domain = document.getElementById('cfgDomain').value.trim();

            if (!pageId) {
                alert('Vui lòng lưu hoặc nhập Mã Định Danh (Page ID) trước khi quét form!');
                return;
            }

            btn.disabled = true;
            btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> <span>Đang quét DOM...</span>';
            statusEl.innerHTML = `<span class="text-amber-400"><i class="fa-solid fa-spinner fa-spin mr-1"></i> Đang tải và phân tích HTML...</span>`;

            try {
                let htmlContent = '';
                // 1. Kiểm tra nếu đang mở trong Code Studio
                if (currentStudioLanding && currentStudioLanding.page_id === pageId && typeof studioFileContents !== 'undefined') {
                    htmlContent = studioFileContents['index.html'] || document.getElementById('csCodeEditor')?.value || '';
                }

                // 2. Nếu chưa có HTML, fetch từ server
                if (!htmlContent) {
                    const res = await fetch(`/api/landings/get?page_id=${encodeURIComponent(pageId)}&domain=${encodeURIComponent(domain)}`);
                    if (res.ok) {
                        const data = await res.json();
                        if (data.landing && data.landing.code_html) {
                            htmlContent = data.landing.code_html;
                        } else if (data.files && data.files.length > 0) {
                            const idxFile = data.files.find(f => f.file_path === 'index.html' || f.file_path.endsWith('.html'));
                            if (idxFile) htmlContent = idxFile.content;
                        }
                    }
                }

                // 3. Nếu chưa có HTML hoặc HTML trống, yêu cầu nhập mã nguồn
                if (!htmlContent || !htmlContent.trim()) {
                    statusEl.innerHTML = `<span class="text-amber-400 font-semibold"><i class="fa-solid fa-triangle-exclamation mr-1"></i> Chưa có mã HTML. Vui lòng nhập mã code vào Code Studio trước.</span>`;
                    alert('Chưa có nội dung HTML để phân tích. Vui lòng nhập hoặc dán mã code của bạn vào Code Studio trước khi thực hiện Quét Form & Input!');
                    return;
                }

                // Tiến hành phân tích cú pháp cây DOM
                const parser = new DOMParser();
                const doc = parser.parseFromString(htmlContent, 'text/html');

                const customSelector = document.getElementById('cfgFormSelector').value.trim();
                let forms = [];
                if (customSelector) {
                    try {
                        forms = Array.from(doc.querySelectorAll(customSelector));
                    } catch(e) {
                        console.warn('Selector tùy chỉnh không hợp lệ, fallback về selector mặc định', e);
                    }
                }
                if (forms.length === 0) {
                    forms = Array.from(doc.querySelectorAll('form, .ladi-form, [data-role="form"]'));
                }

                // Tập hợp các input elements
                let elementsToScan = [];
                if (forms.length > 0) {
                    forms.forEach(f => {
                        const inputs = f.querySelectorAll('input, select, textarea');
                        inputs.forEach(inp => {
                            if (!elementsToScan.includes(inp)) elementsToScan.push(inp);
                        });
                    });
                } else {
                    // Nếu không có thẻ form rõ ràng, quét toàn bộ input trong body (trừ script, style)
                    elementsToScan = Array.from(doc.body ? doc.body.querySelectorAll('input, select, textarea') : []);
                }

                // Lọc bỏ các input không cần thiết (hidden token, csrf, submit, button)
                const validInputs = elementsToScan.filter(el => {
                    const type = (el.getAttribute('type') || 'text').toLowerCase();
                    if (type === 'submit' || type === 'button' || type === 'reset' || type === 'image') return false;
                    // Bỏ qua honeypot nội bộ
                    const name = el.getAttribute('name') || '';
                    if (name === '_hp_check' || name === '_challenge_token' || name === '_client_ts') return false;
                    return true;
                });

                if (validInputs.length === 0) {
                    renderScannedInputsTable([]);
                    statusEl.innerHTML = `<span class="text-sky-400 font-semibold"><i class="fa-solid fa-circle-info mr-1"></i> Trang nội dung không có form input (Landing cảm ơn / hiển thị thuần)</span>`;
                    return;
                }

                // Trích xuất metadata và áp dụng thuật toán heuristic matching
                const extractedFields = validInputs.map((el, index) => {
                    const tag = el.tagName.toLowerCase();
                    const type = (el.getAttribute('type') || (tag === 'textarea' ? 'textarea' : 'text')).toLowerCase();
                    const name = el.getAttribute('name') || '';
                    const id = el.getAttribute('id') || '';
                    const placeholder = el.getAttribute('placeholder') || '';
                    const isRequired = el.hasAttribute('required') || el.classList.contains('required');

                    // Tìm label đi kèm
                    let labelText = '';
                    if (id) {
                        const labelEl = doc.querySelector(`label[for="${id}"]`);
                        if (labelEl) labelText = labelEl.innerText.trim();
                    }
                    if (!labelText) {
                        const parentLabel = el.closest('label');
                        if (parentLabel) labelText = parentLabel.innerText.trim();
                    }

                    // Xây dựng selector tối ưu nhất
                    let selector = '';
                    if (id) selector = `#${id}`;
                    else if (name) selector = `${tag}[name="${name}"]`;
                    else selector = `${tag}:nth-of-type(${index + 1})`;

                    // Thuật toán Smart Field Heuristic Matching
                    const searchText = `${name} ${id} ${placeholder} ${labelText}`.toLowerCase();
                    let leadField = 'custom';

                    if (type === 'tel' || /(phone|sdt|tel|dien_thoai|so_dien_thoai|thoại|di_dong|mobile)/i.test(searchText)) {
                        leadField = 'phone';
                    } else if (/(fullname|hoten|ho_ten|tên|name|họ|khach_hang)/i.test(searchText)) {
                        leadField = 'fullname';
                    } else if (type === 'email' || /(email|mail|thư)/i.test(searchText)) {
                        leadField = 'email';
                    } else if (tag === 'textarea' || /(note|ghi_chu|tin_nhan|message|nhu_cau|cau_hoi|y_kien)/i.test(searchText)) {
                        leadField = 'note';
                    } else if (/(package|khoa_hoc|gói|khoá_học|course|san_pham)/i.test(searchText)) {
                        leadField = 'package';
                    } else if (/(source|nguon|campaign)/i.test(searchText)) {
                        leadField = 'source';
                    }

                    return {
                        tag,
                        type,
                        name,
                        id,
                        placeholder: placeholder || labelText,
                        labelText,
                        selector,
                        lead_field: leadField,
                        required: isRequired || (leadField === 'phone' || leadField === 'fullname')
                    };
                });

                scannedFormFieldsList = extractedFields;
                renderScannedInputsTable(extractedFields);

                const formsCount = forms.length > 0 ? forms.length : 1;
                statusEl.innerHTML = `<span class="text-emerald-400 font-bold"><i class="fa-solid fa-circle-check mr-1"></i> Quét thành công: ${extractedFields.length} trường (${formsCount} form)</span>`;

            } catch (err) {
                console.error('Lỗi khi quét form:', err);
                statusEl.innerHTML = `<span class="text-rose-400"><i class="fa-solid fa-triangle-exclamation mr-1"></i> Lỗi: ${escapeHTML(err.message)}</span>`;
                alert('Lỗi quét form: ' + err.message);
            } finally {
                btn.disabled = false;
                btn.innerHTML = '<i class="fa-solid fa-magnifying-glass"></i> <span>Quét Form Từ HTML</span>';
            }
        }

        function renderScannedInputsTable(fields) {
            const tbody = document.getElementById('tbodyScannedInputs');
            if (!fields || fields.length === 0) {
                tbody.innerHTML = `
                    <tr>
                        <td colspan="6" class="py-6 text-center text-slate-500 font-sans">
                            Chưa có trường nhập liệu nào được cấu hình. Bấm <b class="text-indigo-400">"Quét Form Từ HTML"</b> bên trên.
                        </td>
                    </tr>
                `;
                return;
            }

            tbody.innerHTML = fields.map((f, idx) => {
                const tagBadge = f.tag === 'textarea' ? '<span class="bg-purple-950 text-purple-300 border border-purple-800 px-1.5 py-0.5 rounded text-[10px]">textarea</span>'
                    : f.type === 'tel' ? '<span class="bg-emerald-950 text-emerald-300 border border-emerald-800 px-1.5 py-0.5 rounded text-[10px]">tel</span>'
                    : f.type === 'email' ? '<span class="bg-blue-950 text-blue-300 border border-blue-800 px-1.5 py-0.5 rounded text-[10px]">email</span>'
                    : `<span class="bg-slate-800 text-slate-300 border border-slate-700 px-1.5 py-0.5 rounded text-[10px]">${escapeHTML(f.type || f.tag)}</span>`;

                const nameDisplay = f.name ? `name="<b>${escapeHTML(f.name)}</b>"` : (f.id ? `id="<b>${escapeHTML(f.id)}</b>"` : `<i>selector</i>`);

                return `
                    <tr data-field-row data-tag="${escapeHTML(f.tag)}" data-type="${escapeHTML(f.type)}" data-name="${escapeHTML(f.name || '')}" data-id="${escapeHTML(f.id || '')}" data-ph="${escapeHTML(f.placeholder || '')}" class="hover:bg-slate-900/80 transition">
                        <td class="py-2.5 px-3">
                            <div class="flex items-center gap-1.5">
                                ${tagBadge}
                            </div>
                        </td>
                        <td class="py-2.5 px-3">
                            <input type="text" value="${escapeHTML(f.selector || '')}" class="map-selector w-full bg-slate-950 border border-slate-800 rounded px-2 py-1 text-[11px] text-cyan-300 font-mono focus:border-cyan-500" title="CSS Selector để bắt trường này">
                            <div class="text-[10px] text-slate-500 font-sans mt-0.5">${nameDisplay}</div>
                        </td>
                        <td class="py-2.5 px-3 max-w-[140px] truncate">
                            <span class="text-slate-300 font-sans text-xs" title="${escapeHTML(f.placeholder || f.labelText || '')}">
                                ${escapeHTML(f.placeholder || f.labelText || '(Không có nhãn)')}
                            </span>
                        </td>
                        <td class="py-2.5 px-3">
                            <select class="map-lead-field w-full bg-slate-950 border border-slate-800 rounded px-2 py-1 text-xs text-white focus:border-cyan-500 font-sans">
                                <option value="phone" ${f.lead_field === 'phone' ? 'selected' : ''}>📞 phone (Số điện thoại - Bắt buộc)</option>
                                <option value="fullname" ${f.lead_field === 'fullname' ? 'selected' : ''}>👤 fullname (Họ và tên)</option>
                                <option value="email" ${f.lead_field === 'email' ? 'selected' : ''}>✉️ email (Hòm thư điện tử)</option>
                                <option value="note" ${f.lead_field === 'note' ? 'selected' : ''}>📝 note (Ghi chú / Nhu cầu)</option>
                                <option value="package" ${f.lead_field === 'package' ? 'selected' : ''}>📦 package (Gói / Khóa học)</option>
                                <option value="source" ${f.lead_field === 'source' ? 'selected' : ''}>🌐 source (Nguồn chiến dịch)</option>
                                <option value="custom" ${f.lead_field === 'custom' ? 'selected' : ''}>⚙️ custom (Trường tùy chỉnh)</option>
                                <option value="ignore" ${f.lead_field === 'ignore' ? 'selected' : ''}>🚫 ignore (Bỏ qua - không lưu)</option>
                            </select>
                        </td>
                        <td class="py-2.5 px-3 text-center">
                            <input type="checkbox" class="map-required rounded text-cyan-500 focus:ring-0" ${f.required ? 'checked' : ''} title="Đánh dấu trường bắt buộc nhập">
                        </td>
                        <td class="py-2.5 px-3 text-center">
                            <button type="button" onclick="removeMappingRow(this)" class="text-slate-500 hover:text-rose-400 p-1 transition" title="Xóa trường này khỏi cấu hình">
                                <i class="fa-solid fa-trash-can"></i>
                            </button>
                        </td>
                    </tr>
                `;
            }).join('');
        }

        function addManualMappingRow(data = {}) {
            const tbody = document.getElementById('tbodyScannedInputs');
            if (tbody.querySelector('td[colspan]')) {
                tbody.innerHTML = '';
            }

            const row = document.createElement('tr');
            row.setAttribute('data-field-row', 'true');
            row.setAttribute('data-tag', data.tag || 'input');
            row.setAttribute('data-type', data.type || 'text');
            row.setAttribute('data-name', data.name || '');
            row.setAttribute('data-id', data.id || '');
            row.setAttribute('data-ph', data.placeholder || '');
            row.className = 'hover:bg-slate-900/80 transition';
            row.innerHTML = `
                <td class="py-2.5 px-3">
                    <span class="bg-slate-800 text-slate-300 border border-slate-700 px-1.5 py-0.5 rounded text-[10px]">manual</span>
                </td>
                <td class="py-2.5 px-3">
                    <input type="text" value="${escapeHTML(data.selector || 'input[name="ten_truong"]')}" class="map-selector w-full bg-slate-950 border border-slate-800 rounded px-2 py-1 text-[11px] text-cyan-300 font-mono focus:border-cyan-500">
                </td>
                <td class="py-2.5 px-3">
                    <span class="text-slate-300 font-sans text-xs">${escapeHTML(data.placeholder || 'Thêm thủ công')}</span>
                </td>
                <td class="py-2.5 px-3">
                    <select class="map-lead-field w-full bg-slate-950 border border-slate-800 rounded px-2 py-1 text-xs text-white focus:border-cyan-500 font-sans">
                        <option value="phone" ${data.lead_field === 'phone' ? 'selected' : ''}>📞 phone (Số điện thoại)</option>
                        <option value="fullname" ${data.lead_field === 'fullname' ? 'selected' : ''}>👤 fullname (Họ và tên)</option>
                        <option value="email" ${data.lead_field === 'email' ? 'selected' : ''}>✉️ email (Hòm thư điện tử)</option>
                        <option value="note" ${data.lead_field === 'note' ? 'selected' : ''}>📝 note (Ghi chú / Nhu cầu)</option>
                        <option value="package" ${data.lead_field === 'package' ? 'selected' : ''}>📦 package (Gói / Khóa học)</option>
                        <option value="source" ${data.lead_field === 'source' ? 'selected' : ''}>🌐 source (Nguồn chiến dịch)</option>
                        <option value="custom" ${data.lead_field === 'custom' || !data.lead_field ? 'selected' : ''}>⚙️ custom (Trường tùy chỉnh)</option>
                        <option value="ignore" ${data.lead_field === 'ignore' ? 'selected' : ''}>🚫 ignore (Bỏ qua)</option>
                    </select>
                </td>
                <td class="py-2.5 px-3 text-center">
                    <input type="checkbox" class="map-required rounded text-cyan-500 focus:ring-0" ${data.required ? 'checked' : ''}>
                </td>
                <td class="py-2.5 px-3 text-center">
                    <button type="button" onclick="removeMappingRow(this)" class="text-slate-500 hover:text-rose-400 p-1 transition">
                        <i class="fa-solid fa-trash-can"></i>
                    </button>
                </td>
            `;
            tbody.appendChild(row);
        }

        function removeMappingRow(btn) {
            const row = btn.closest('tr');
            if (row) row.remove();
            const tbody = document.getElementById('tbodyScannedInputs');
            if (tbody.children.length === 0) {
                renderScannedInputsTable([]);
            }
        }

        async function autoInjectTrackingScript() {
            const pageId = document.getElementById('cfgPageId').value.trim();
            const domain = document.getElementById('cfgDomain').value.trim();
            if (!pageId) {
                alert('Vui lòng chọn hoặc nhập Landing Page trước!');
                return;
            }

            const injectTag = '\n<!-- [APEX AUTO-INJECT TRACKER & FORM INTERCEPTOR] -->\n<script src="/tracker.js" data-landing-id="' + pageId + '"><' + '/script>\n';

            // Nếu đang trong Code Studio
            if (currentStudioLanding && currentStudioLanding.page_id === pageId) {
                const editor = document.getElementById('csCodeEditor');
                let code = editor ? editor.value : (studioFileContents['index.html'] || '');

                if (code.includes('tracker.js')) {
                    alert('Landing Page này đã có thẻ script tracker.js!');
                    return;
                }

                if (code.toLowerCase().includes('</body>')) {
                    const idx = code.toLowerCase().lastIndexOf('</body>');
                    code = code.substring(0, idx) + injectTag + code.substring(idx);
                } else {
                    code = code + injectTag;
                }

                if (editor) editor.value = code;
                studioFileContents['index.html'] = code;
                studioEditorDirty = true;
                if (typeof updateEditorStats === 'function') updateEditorStats(true);
                alert('✅ Đã nhúng thẻ tracker.js thành công vào mã HTML! Hãy bấm "Lưu Code" hoặc "Deploy".');
                return;
            }

            alert('Đã kích hoạt chế độ tự động nhúng Script. Backend sẽ tự động chèn thẻ tracker.js khi khách truy cập Landing Page!');
        }

        function openStudioFormScanner() {
            if (!currentStudioLanding) return;
            openLandingSettings(currentStudioLanding.domain || '', currentStudioLanding.page_id);
            // Chuyển sang Tab 2 (Form & Quét Input) và tự động kích hoạt quét
            setTimeout(() => {
                switchSettingsTab('form');
                scanLandingFormFields();
            }, 300);
        }

