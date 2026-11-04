/* ==============================================================
   APEX VISUAL WEB BUILDER - STUDIO CORE, FILE MANAGER & SHORTCUTS
   Workspace, Code Editor, 2-Way Sync, File Tree, Shortcuts & Deploy
   ============================================================== */

        // ============================================================
        // 4. LẮNG NGHE PHÍM TẮT WINDOW & SỰ KIỆN CLICK CHUỘT TOÀN DIỆN
        // ============================================================
        window.addEventListener('keydown', function(e) {
            const studioModal = document.getElementById('codeStudioModal');
            if (!studioModal || studioModal.classList.contains('hidden')) return;

            const activeEl = document.activeElement;
            const isInputActive = activeEl && (activeEl.tagName === 'INPUT' || activeEl.tagName === 'TEXTAREA' || activeEl.isContentEditable);
            const isCodeEditor = activeEl && activeEl.id === 'csCodeEditor';

            // Escape: Đóng Tìm kiếm / Context Menu / History hoặc chọn thẻ cha
            if (e.key === 'Escape') {
                const searchWidget = document.getElementById('csSearchReplaceWidget');
                if (searchWidget && !searchWidget.classList.contains('hidden')) {
                    closeStudioSearch();
                    return;
                }
                const ctxMenu = document.getElementById('csContextMenu');
                if (ctxMenu && !ctxMenu.classList.contains('hidden')) {
                    closeContextMenu();
                    return;
                }
                const histDropdown = document.getElementById('csHistoryDropdown');
                if (histDropdown && !histDropdown.classList.contains('hidden')) {
                    histDropdown.classList.add('hidden');
                    return;
                }
                if (!isInputActive) {
                    selectParentNode();
                }
                return;
            }

            // Ctrl/Cmd + F: Tìm kiếm trong Code Editor (chặn tìm kiếm mặc định của trình duyệt)
            if ((e.ctrlKey || e.metaKey) && (e.key === 'f' || e.key === 'F')) {
                e.preventDefault();
                e.stopPropagation();
                openStudioSearch(false);
                return;
            }

            // Ctrl/Cmd + H: Tìm kiếm và Thay thế (chặn mở lịch sử trình duyệt)
            if ((e.ctrlKey || e.metaKey) && (e.key === 'h' || e.key === 'H')) {
                e.preventDefault();
                e.stopPropagation();
                openStudioSearch(true);
                return;
            }

            // Ctrl/Cmd + Z: Undo
            if ((e.ctrlKey || e.metaKey) && (e.key === 'z' || e.key === 'Z') && !e.shiftKey) {
                if (!isCodeEditor) {
                    e.preventDefault();
                    studioUndo();
                }
                return;
            }

            // Ctrl/Cmd + Y hoặc Ctrl/Cmd + Shift + Z: Redo
            if ((e.ctrlKey || e.metaKey) && ((e.key === 'y' || e.key === 'Y') || ((e.key === 'z' || e.key === 'Z') && e.shiftKey))) {
                if (!isCodeEditor) {
                    e.preventDefault();
                    studioRedo();
                }
                return;
            }

            // Alt + Z: Toggle Word Wrap
            if (e.altKey && (e.key === 'z' || e.key === 'Z')) {
                e.preventDefault();
                toggleCodeWordWrap();
                return;
            }

            // Khi đang gõ trong input text của Inspector hay ô tìm kiếm thì bỏ qua các phím dưới
            if (isInputActive) return;

            // Ctrl + Alt + C: Sao chép Style CSS
            if ((e.ctrlKey || e.metaKey) && e.altKey && (e.key === 'c' || e.key === 'C')) {
                e.preventDefault();
                copyCurrentNodeStyles();
                return;
            }

            // Ctrl + Alt + V: Dán Style CSS
            if ((e.ctrlKey || e.metaKey) && e.altKey && (e.key === 'v' || e.key === 'V')) {
                e.preventDefault();
                pasteNodeStyles();
                return;
            }

            // Ctrl + C: Sao chép phần tử
            if ((e.ctrlKey || e.metaKey) && (e.key === 'c' || e.key === 'C') && !e.altKey) {
                copyCurrentNode();
                return;
            }

            // Ctrl + X: Cắt phần tử
            if ((e.ctrlKey || e.metaKey) && (e.key === 'x' || e.key === 'X')) {
                cutCurrentNode();
                return;
            }

            // Ctrl + V: Dán phần tử bên dưới
            if ((e.ctrlKey || e.metaKey) && (e.key === 'v' || e.key === 'V') && !e.altKey) {
                pasteCopiedNode();
                return;
            }

            // Ctrl + D: Nhân bản phần tử
            if ((e.ctrlKey || e.metaKey) && (e.key === 'd' || e.key === 'D')) {
                e.preventDefault();
                duplicateCurrentNode();
                return;
            }

            // Shift + Alt + Up: Đưa lên đầu cùng cấp
            if (e.shiftKey && e.altKey && e.key === 'ArrowUp') {
                e.preventDefault();
                moveCurrentNodeToTop();
                return;
            }

            // Shift + Alt + Down: Đưa xuống cuối cùng cấp
            if (e.shiftKey && e.altKey && e.key === 'ArrowDown') {
                e.preventDefault();
                moveCurrentNodeToBottom();
                return;
            }

            // Alt + Up: Di chuyển lên trước
            if (!e.shiftKey && e.altKey && e.key === 'ArrowUp') {
                e.preventDefault();
                moveCurrentNodeUp();
                return;
            }

            // Alt + Down: Di chuyển xuống sau
            if (!e.shiftKey && e.altKey && e.key === 'ArrowDown') {
                e.preventDefault();
                moveCurrentNodeDown();
                return;
            }

            // Delete: Xóa phần tử
            if (e.key === 'Delete') {
                e.preventDefault();
                deleteCurrentNode();
                return;
            }
        });

        // Click ngoài để đóng Context Menu và History Dropdown
        window.addEventListener('click', function(e) {
            const ctxMenu = document.getElementById('csContextMenu');
            if (ctxMenu && !ctxMenu.classList.contains('hidden') && !ctxMenu.contains(e.target)) {
                closeContextMenu();
            }
            const histDropdown = document.getElementById('csHistoryDropdown');
            const histBtn = document.getElementById('btnStudioHistory');
            if (histDropdown && !histDropdown.classList.contains('hidden') && !histDropdown.contains(e.target) && !histBtn?.contains(e.target)) {
                histDropdown.classList.add('hidden');
            }
        });

        // Lắng nghe right-click trên Canvas Overlay và Gizmo Box
        document.addEventListener('DOMContentLoaded', function() {
            const gizmoBox = document.getElementById('csGizmoBox');
            if (gizmoBox) {
                gizmoBox.addEventListener('contextmenu', function(e) {
                    e.preventDefault();
                    e.stopPropagation();
                    if (selectedAstNodeId) {
                        openContextMenu(e.clientX, e.clientY, selectedAstNodeId);
                    }
                });
            }
            const canvas = document.getElementById('csOverlayCanvas');
            if (canvas) {
                canvas.addEventListener('contextmenu', function(e) {
                    e.preventDefault();
                    e.stopPropagation();
                    if (selectedAstNodeId) {
                        openContextMenu(e.clientX, e.clientY, selectedAstNodeId);
                    }
                });
            }
        });

        // Đồng bộ chuỗi nguồn vào Code Editor (Bảo vệ Isolated Scope: Không ghi đè HTML vào file CSS/JS)
        function syncCodeEditorFromAst(newSource) {
            let targetHtmlFile = 'index.html';
            if (activeStudioFile && (activeStudioFile.endsWith('.html') || activeStudioFile.endsWith('.htm'))) {
                targetHtmlFile = activeStudioFile;
            }
            studioFileContents[targetHtmlFile] = newSource;
            if (activeStudioFile === targetHtmlFile) {
                const editor = document.getElementById('csCodeEditor');
                if (editor) editor.value = newSource;
                studioEditorDirty = true;
                document.getElementById('csSaveStatus').innerText = 'Chưa lưu thay đổi';
                document.getElementById('csSaveStatus').className = 'text-[11px] text-amber-400 font-mono ml-2';
                updateEditorStats(false);
            }
        }

        // ============================================================
        // MỞ CODE STUDIO & NẠP DỮ LIỆU
        // ============================================================
        async function openCodeStudio(domain, pageId) {
            try {
                const res = await fetch(`/api/landings/get?page_id=${encodeURIComponent(pageId)}&domain=${encodeURIComponent(domain)}`);
                if (!res.ok) throw new Error('Không thể tải mã nguồn Landing Page');
                const data = await res.json();
                currentStudioLanding = data.landing;
                currentStudioFiles = data.files || [];
                studioFileContents = {};
                studioFolders = [];

                currentStudioFiles.forEach(f => {
                    if (f.file_type === 'folder') {
                        if (!studioFolders.includes(f.file_path)) {
                            studioFolders.push(f.file_path);
                        }
                    } else {
                        studioFileContents[f.file_path] = f.content || '';
                        const parts = f.file_path.split('/').filter(Boolean);
                        if (parts.length > 1) {
                            let acc = '';
                            for (let i = 0; i < parts.length - 1; i++) {
                                acc = acc ? `${acc}/${parts[i]}` : parts[i];
                                if (!studioFolders.includes(acc)) {
                                    studioFolders.push(acc);
                                }
                            }
                        }
                    }
                });

                if (!studioFileContents['index.html']) {
                    studioFileContents['index.html'] = currentStudioLanding.code_html || '';
                }

                activeStudioFile = 'index.html';
                studioEditorDirty = false;
                selectedAstNodeId = null;

                document.getElementById('csLandingBadge').innerText = currentStudioLanding.page_id;
                document.getElementById('csLandingName').innerText = currentStudioLanding.name || '';
                document.getElementById('csActiveFileName').innerText = activeStudioFile;
                document.getElementById('csSaveStatus').innerText = 'Đã đồng bộ';
                document.getElementById('csSaveStatus').className = 'text-[11px] text-slate-400 font-mono ml-2';

                renderStudioFileList();
                loadContentIntoEditor(studioFileContents[activeStudioFile]);
                setStudioDevice('pc');
                setStudioViewMode('split');
                refreshStudioPreview();

                document.getElementById('codeStudioModal').classList.remove('hidden');
            } catch (e) {
                alert('Lỗi mở Studio: ' + e.message);
            }
        }

        function closeCodeStudio() {
            if (studioEditorDirty) {
                if (!confirm('Bạn có thay đổi chưa lưu mã nguồn. Bạn có chắc chắn muốn đóng Code Studio không?')) return;
            }
            document.getElementById('codeStudioModal').classList.add('hidden');
            if (typeof loadLandingPages === 'function') loadLandingPages();
        }

        function toggleFolderExpand(fldPath) {
            folderExpandedState[fldPath] = folderExpandedState[fldPath] === false ? true : false;
            renderStudioFileList();
        }

        function renderStudioFileList() {
            const listEl = document.getElementById('csFileList');
            if (!listEl) return;

            const tree = { name: '', path: '', isFolder: true, children: {} };

            studioFolders.forEach(fld => {
                const parts = fld.split('/').filter(Boolean);
                let curr = tree;
                let curPath = '';
                parts.forEach(p => {
                    curPath = curPath ? `${curPath}/${p}` : p;
                    if (!curr.children[p]) {
                        curr.children[p] = { name: p, path: curPath, isFolder: true, children: {} };
                    }
                    curr = curr.children[p];
                });
            });

            const filePaths = Object.keys(studioFileContents);
            filePaths.forEach(fp => {
                const parts = fp.split('/').filter(Boolean);
                let curr = tree;
                let curPath = '';
                for (let i = 0; i < parts.length - 1; i++) {
                    const p = parts[i];
                    curPath = curPath ? `${curPath}/${p}` : p;
                    if (!curr.children[p]) {
                        curr.children[p] = { name: p, path: curPath, isFolder: true, children: {} };
                    }
                    curr = curr.children[p];
                }
                const fileName = parts[parts.length - 1];
                curr.children[fileName] = {
                    name: fileName,
                    path: fp,
                    isFolder: false
                };
            });

            function renderNode(node, depth = 0) {
                const keys = Object.keys(node.children).sort((a, b) => {
                    const itemA = node.children[a];
                    const itemB = node.children[b];
                    if (itemA.isFolder && !itemB.isFolder) return -1;
                    if (!itemA.isFolder && itemB.isFolder) return 1;
                    if (itemA.path === 'index.html') return -1;
                    if (itemB.path === 'index.html') return 1;
                    return a.localeCompare(b);
                });

                let html = '';
                keys.forEach(k => {
                    const item = node.children[k];
                    const indentPx = depth * 12 + 4;

                    if (item.isFolder) {
                        const isExpanded = folderExpandedState[item.path] !== false;
                        html += `
                            <div class="group/fld my-0.5">
                                <div class="px-2 py-1 rounded-lg flex items-center justify-between cursor-pointer hover:bg-slate-800/80 text-slate-300 hover:text-white transition select-none"
                                     style="padding-left: ${indentPx}px;"
                                     onclick="toggleFolderExpand('${escapeHTML(item.path)}')">
                                    <div class="flex items-center gap-1.5 truncate">
                                        <i class="fa-solid fa-chevron-right text-[8px] text-slate-500 transition-transform ${isExpanded ? 'rotate-90' : ''}"></i>
                                        <i class="fa-solid ${isExpanded ? 'fa-folder-open text-amber-400' : 'fa-folder text-amber-400'} text-xs"></i>
                                        <span class="truncate font-semibold text-slate-200 text-xs">${escapeHTML(item.name)}</span>
                                    </div>
                                    <div class="hidden group-hover/fld:flex items-center gap-0.5 text-[11px]" onclick="event.stopPropagation()">
                                        <button onclick="openStudioUploadModal('${escapeHTML(item.path)}')" class="text-slate-400 hover:text-emerald-400 p-1" title="Tải tệp từ máy vào thư mục này">
                                            <i class="fa-solid fa-cloud-arrow-up"></i>
                                        </button>
                                        <button onclick="openNewFileModal('${escapeHTML(item.path)}')" class="text-slate-400 hover:text-cyan-400 p-1" title="Tạo tệp">
                                            <i class="fa-solid fa-file-circle-plus"></i>
                                        </button>
                                        <button onclick="openNewFolderModal('${escapeHTML(item.path)}')" class="text-slate-400 hover:text-amber-400 p-1" title="Tạo thư mục con">
                                            <i class="fa-solid fa-folder-plus"></i>
                                        </button>
                                        <button onclick="openRenameModal('folder', '${escapeHTML(item.path)}')" class="text-slate-400 hover:text-sky-400 p-1" title="Đổi tên">
                                            <i class="fa-solid fa-pen text-[10px]"></i>
                                        </button>
                                        <button onclick="deleteFolderConfirm('${escapeHTML(item.path)}')" class="text-slate-400 hover:text-rose-400 p-1" title="Xóa">
                                            <i class="fa-solid fa-trash-can text-[10px]"></i>
                                        </button>
                                    </div>
                                </div>
                                <div class="${isExpanded ? '' : 'hidden'}">
                                    ${renderNode(item, depth + 1)}
                                </div>
                            </div>
                        `;
                    } else {
                        const isActive = item.path === activeStudioFile;
                        let icon = '<i class="fa-solid fa-file-code text-slate-400"></i>';
                        if (item.path.endsWith('.html') || item.path.endsWith('.htm')) icon = '<i class="fa-brands fa-html5 text-orange-500"></i>';
                        else if (item.path.endsWith('.css')) icon = '<i class="fa-brands fa-css3-alt text-blue-400"></i>';
                        else if (item.path.endsWith('.js')) icon = '<i class="fa-brands fa-js text-amber-400"></i>';
                        else if (item.path.endsWith('.json')) icon = '<i class="fa-solid fa-code text-emerald-400"></i>';
                        else if (/\.(png|jpe?g|gif|svg|webp|ico)$/i.test(item.path)) icon = '<i class="fa-solid fa-image text-purple-400"></i>';

                        html += `
                            <div class="group/file my-0.5 px-2 py-1 rounded-lg flex items-center justify-between cursor-pointer transition select-none ${isActive ? 'bg-cyan-950 text-cyan-300 border border-cyan-800' : 'text-slate-300 hover:bg-slate-800 hover:text-white'}"
                                 style="padding-left: ${indentPx + 12}px;"
                                 onclick="switchStudioFile('${escapeHTML(item.path)}')">
                                <div class="flex items-center gap-1.5 truncate">
                                    ${icon}
                                    <span class="truncate text-xs ${isActive ? 'font-bold' : ''}">${escapeHTML(item.name)}</span>
                                </div>
                                ${item.path !== 'index.html' ? `
                                <div class="hidden group-hover/file:flex items-center gap-0.5 text-[11px]" onclick="event.stopPropagation()">
                                    <button onclick="openRenameModal('file', '${escapeHTML(item.path)}')" class="text-slate-400 hover:text-sky-400 p-1" title="Đổi tên">
                                        <i class="fa-solid fa-pen text-[10px]"></i>
                                    </button>
                                    <button onclick="deleteStudioFile('${escapeHTML(item.path)}')" class="text-slate-400 hover:text-rose-400 p-1" title="Xóa">
                                        <i class="fa-solid fa-trash-can text-[10px]"></i>
                                    </button>
                                </div>
                                ` : '<span class="text-[9px] text-cyan-500 font-mono pr-1">root</span>'}
                            </div>
                        `;
                    }
                });

                if (keys.length === 0 && depth > 0) {
                    return `<div class="text-[10px] text-slate-500 italic py-1" style="padding-left: ${depth * 12 + 16}px;">(Thư mục trống)</div>`;
                }
                return html;
            }

            listEl.innerHTML = renderNode(tree, 0);
        }

        function switchStudioFile(filePath) {
            if (activeStudioFile === filePath) return;
            const currentEditorValue = document.getElementById('csCodeEditor').value;
            studioFileContents[activeStudioFile] = currentEditorValue;

            activeStudioFile = filePath;
            document.getElementById('csActiveFileName').innerText = activeStudioFile;

            let iconCls = 'fa-brands fa-html5 text-orange-500';
            if (activeStudioFile.endsWith('.css')) iconCls = 'fa-brands fa-css3-alt text-blue-400';
            else if (activeStudioFile.endsWith('.js')) iconCls = 'fa-brands fa-js text-amber-400';
            document.getElementById('csCurrentFileBadge').querySelector('i').className = iconCls;

            renderStudioFileList();
            loadContentIntoEditor(studioFileContents[activeStudioFile] || '');
            selectedAstNodeId = null;
            const gizmoBox = document.getElementById('csGizmoBox');
            if (gizmoBox) gizmoBox.classList.add('hidden');
            refreshStudioPreview();
        }

        function loadContentIntoEditor(content) {
            const editor = document.getElementById('csCodeEditor');
            editor.value = content || '';
            updateEditorStats(false);
            studioEditorDirty = false;
            document.getElementById('csSaveStatus').innerText = 'Đã đồng bộ';
            document.getElementById('csSaveStatus').className = 'text-[11px] text-slate-400 font-mono ml-2';
            initCodeWordWrap();

            if (activeStudioFile.endsWith('.html') || activeStudioFile.endsWith('.htm')) {
                astMapper.parse(content);
                const count = Object.keys(astMapper.astMap).length;
                const nodeCountEl = document.getElementById('csAstNodeCount');
                if (nodeCountEl) nodeCountEl.innerText = `AST Nodes: ${count}`;

                // Khởi tạo lịch sử thao tác Photoshop
                studioHistory = [];
                studioHistoryIndex = -1;
                pushStudioHistory('Mở tệp', content, null);
                renderAstLayerTree();
            }
        }

        function onCodeEditorInput() {
            updateEditorStats(true);

            // Debounce đồng bộ 2 chiều: Code gõ trong editor -> Cập nhật trực tiếp Preview Iframe
            clearTimeout(editorDebounceTimer);
            editorDebounceTimer = setTimeout(() => {
                const content = document.getElementById('csCodeEditor').value;
                studioFileContents[activeStudioFile] = content;

                if (activeStudioFile.endsWith('.html') || activeStudioFile.endsWith('.htm')) {
                    astMapper.parse(content);
                    const count = Object.keys(astMapper.astMap).length;
                    const nodeCountEl = document.getElementById('csAstNodeCount');
                    if (nodeCountEl) nodeCountEl.innerText = `AST Nodes: ${count}`;

                    // Cập nhật DOM Preview mà không phá vỡ trạng thái
                    const iframe = document.getElementById('csPreviewIframe');
                    if (iframe) {
                        iframe.srcdoc = astMapper.generateInstrumentedHTML();
                    }
                    renderAstLayerTree();

                    // Lưu vào lịch sử Photoshop (Debounced)
                    clearTimeout(editorHistoryTimer);
                    editorHistoryTimer = setTimeout(() => {
                        pushStudioHistory('Chỉnh sửa mã nguồn', content, selectedAstNodeId);
                    }, 800);
                }
            }, 250);
        }

        var activeCodeHighlightRange = null;
        function updateEditorStats(isUserTyping, highlightRange = undefined) {
            const editor = document.getElementById('csCodeEditor');
            if (!editor) return;
            const lines = (editor.value.match(/\n/g) || []).length + 1;

            if (highlightRange !== undefined) {
                activeCodeHighlightRange = highlightRange;
            }

            const lineNumEl = document.getElementById('csLineNumbers');
            if (lineNumEl) {
                let numHtml = '';
                const hStart = activeCodeHighlightRange ? activeCodeHighlightRange.startLine : -1;
                const hEnd = activeCodeHighlightRange ? activeCodeHighlightRange.endLine : -1;
                for (let i = 1; i <= lines; i++) {
                    if (i >= hStart && i <= hEnd) {
                        numHtml += `<span class="block bg-cyan-500/25 text-cyan-300 font-bold px-1 rounded-sm border-r-2 border-cyan-400 leading-6">${i}</span>`;
                    } else {
                        numHtml += `<span class="block leading-6 text-slate-600">${i}</span>`;
                    }
                }
                lineNumEl.innerHTML = numHtml;
            }

            document.getElementById('csTotalLines').innerText = `Tổng: ${lines} dòng`;
            const pos = editor.selectionStart || 0;
            const textBefore = editor.value.substring(0, pos);
            const curLine = (textBefore.match(/\n/g) || []).length + 1;
            const curCol = pos - textBefore.lastIndexOf('\n');
            const lineColEl = document.getElementById('csLineCol');
            if (lineColEl && (!activeCodeHighlightRange || isUserTyping)) {
                lineColEl.innerText = `Dòng: ${curLine}, Cột: ${curCol}`;
            }

            if (isUserTyping) {
                activeCodeHighlightRange = null;
                studioEditorDirty = true;
                document.getElementById('csSaveStatus').innerText = 'Chưa lưu thay đổi';
                document.getElementById('csSaveStatus').className = 'text-[11px] text-amber-400 font-mono ml-2';
            }
        }

        function syncEditorScroll() {
            const editor = document.getElementById('csCodeEditor');
            const lineNumEl = document.getElementById('csLineNumbers');
            lineNumEl.scrollTop = editor.scrollTop;
        }

        function handleEditorKeyDown(e) {
            const editor = document.getElementById('csCodeEditor');

            if (e.key === 'Tab') {
                e.preventDefault();
                const start = editor.selectionStart;
                const end = editor.selectionEnd;
                editor.value = editor.value.substring(0, start) + '    ' + editor.value.substring(end);
                editor.selectionStart = editor.selectionEnd = start + 4;
                onCodeEditorInput();
            }

            if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
                e.preventDefault();
                saveStudioCode();
            }

            if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
                e.preventDefault();
                deployCodeLive();
            }

            // Ctrl + F: Tìm kiếm trong Code Editor
            if ((e.ctrlKey || e.metaKey) && (e.key === 'f' || e.key === 'F')) {
                e.preventDefault();
                e.stopPropagation();
                openStudioSearch(false);
                return;
            }

            // Ctrl + H: Tìm kiếm và Thay thế
            if ((e.ctrlKey || e.metaKey) && (e.key === 'h' || e.key === 'H')) {
                e.preventDefault();
                e.stopPropagation();
                openStudioSearch(true);
                return;
            }

            // Alt + Z: Toggle Word Wrap
            if (e.altKey && (e.key === 'z' || e.key === 'Z')) {
                e.preventDefault();
                e.stopPropagation();
                toggleCodeWordWrap();
            }
        }

        // Tự động ngắt dòng trong Code Editor (Alt + Z)
        function toggleCodeWordWrap() {
            const editor = document.getElementById('csCodeEditor');
            if (!editor) return;
            const isWrapped = editor.classList.contains('whitespace-pre-wrap');
            if (isWrapped) {
                editor.classList.remove('whitespace-pre-wrap', 'break-words');
                editor.classList.add('whitespace-pre');
                editor.setAttribute('wrap', 'off');
                localStorage.setItem('studio_code_word_wrap', 'off');
                updateWordWrapButtonState(false);
            } else {
                editor.classList.remove('whitespace-pre');
                editor.classList.add('whitespace-pre-wrap', 'break-words');
                editor.setAttribute('wrap', 'soft');
                localStorage.setItem('studio_code_word_wrap', 'on');
                updateWordWrapButtonState(true);
            }
        }

        function updateWordWrapButtonState(isWrapped) {
            const btn = document.getElementById('btnCodeWordWrap');
            if (btn) {
                btn.innerHTML = `<i class="fa-solid fa-text-width text-[10px]"></i> <span>Wrap: ${isWrapped ? '<b class="text-cyan-400">BẬT</b>' : 'TẮT'}</span>`;
                btn.title = `Tự động ngắt dòng (Alt + Z) - Hiện tại: ${isWrapped ? 'BẬT' : 'TẮT'}`;
            }
        }

        function initCodeWordWrap() {
            const saved = localStorage.getItem('studio_code_word_wrap');
            const editor = document.getElementById('csCodeEditor');
            if (saved === 'on' && editor) {
                editor.classList.remove('whitespace-pre');
                editor.classList.add('whitespace-pre-wrap', 'break-words');
                editor.setAttribute('wrap', 'soft');
                updateWordWrapButtonState(true);
            } else {
                updateWordWrapButtonState(false);
            }
        }

        // ============================================================
        // IN-EDITOR SEARCH & REPLACE (Ctrl + F / Ctrl + H)
        // ============================================================
        let studioSearchResults = [];
        let studioSearchIndex = -1;
        let studioSearchMatchCase = false;

        function openStudioSearch(showReplace = false) {
            const widget = document.getElementById('csSearchReplaceWidget');
            const searchInput = document.getElementById('csSearchInput');
            const replaceRow = document.getElementById('csReplaceRow');
            const iconToggle = document.getElementById('iconToggleReplace');
            if (!widget || !searchInput) return;

            widget.classList.remove('hidden');

            if (showReplace) {
                if (replaceRow) replaceRow.classList.remove('hidden');
                if (iconToggle) {
                    iconToggle.classList.remove('fa-chevron-right');
                    iconToggle.classList.add('fa-chevron-down');
                }
            }

            // Lấy văn bản đang được bôi đen trong Code Editor (nếu có) để điền vào ô tìm kiếm
            const editor = document.getElementById('csCodeEditor');
            if (editor && editor.selectionStart !== editor.selectionEnd) {
                const selectedText = editor.value.substring(editor.selectionStart, editor.selectionEnd).trim();
                if (selectedText && selectedText.indexOf('\n') === -1) {
                    searchInput.value = selectedText;
                }
            }

            if (showReplace && document.getElementById('csReplaceInput') && searchInput.value) {
                document.getElementById('csReplaceInput').focus();
                document.getElementById('csReplaceInput').select();
            } else {
                searchInput.focus();
                searchInput.select();
            }

            runStudioSearch(0);
        }

        function closeStudioSearch() {
            const widget = document.getElementById('csSearchReplaceWidget');
            if (widget) widget.classList.add('hidden');
            const editor = document.getElementById('csCodeEditor');
            if (editor) editor.focus();
        }

        function toggleStudioReplace(show) {
            const replaceRow = document.getElementById('csReplaceRow');
            const iconToggle = document.getElementById('iconToggleReplace');
            if (!replaceRow) return;

            const isHidden = replaceRow.classList.contains('hidden');
            const shouldShow = (show !== undefined) ? show : isHidden;

            if (shouldShow) {
                replaceRow.classList.remove('hidden');
                if (iconToggle) {
                    iconToggle.classList.remove('fa-chevron-right');
                    iconToggle.classList.add('fa-chevron-down');
                }
                const replaceInput = document.getElementById('csReplaceInput');
                if (replaceInput) {
                    replaceInput.focus();
                    replaceInput.select();
                }
            } else {
                replaceRow.classList.add('hidden');
                if (iconToggle) {
                    iconToggle.classList.remove('fa-chevron-down');
                    iconToggle.classList.add('fa-chevron-right');
                }
            }
        }

        function toggleSearchMatchCase() {
            studioSearchMatchCase = !studioSearchMatchCase;
            const btn = document.getElementById('btnSearchMatchCase');
            if (btn) {
                if (studioSearchMatchCase) {
                    btn.className = 'px-1.5 py-1 text-cyan-300 bg-cyan-950/80 border border-cyan-800 rounded text-[11px] font-mono font-bold transition';
                } else {
                    btn.className = 'px-1.5 py-1 text-slate-400 hover:text-slate-200 border border-transparent rounded hover:bg-slate-800 text-[11px] font-mono transition';
                }
            }
            runStudioSearch(0);
        }

        function runStudioSearch(navDirection = 0) {
            const searchInput = document.getElementById('csSearchInput');
            const countEl = document.getElementById('csSearchCount');
            const editor = document.getElementById('csCodeEditor');
            if (!searchInput || !countEl || !editor) return;

            const query = searchInput.value;
            if (!query) {
                studioSearchResults = [];
                studioSearchIndex = -1;
                countEl.innerText = '0/0';
                countEl.className = 'absolute right-2 text-[10px] text-slate-400 font-mono select-none';
                return;
            }

            const text = editor.value;
            const textToSearch = studioSearchMatchCase ? text : text.toLowerCase();
            const queryToSearch = studioSearchMatchCase ? query : query.toLowerCase();
            const qLen = query.length;

            const results = [];
            let pos = 0;
            while ((pos = textToSearch.indexOf(queryToSearch, pos)) !== -1) {
                results.push({ start: pos, end: pos + qLen });
                pos += qLen;
            }

            studioSearchResults = results;

            if (results.length === 0) {
                studioSearchIndex = -1;
                countEl.innerText = '0/0';
                countEl.className = 'absolute right-2 text-[10px] text-rose-400 font-mono select-none font-semibold';
                return;
            }

            countEl.className = 'absolute right-2 text-[10px] text-cyan-400 font-mono select-none font-bold';

            if (navDirection === 0) {
                const curStart = editor.selectionStart || 0;
                let closestIdx = 0;
                for (let i = 0; i < results.length; i++) {
                    if (results[i].start >= curStart) {
                        closestIdx = i;
                        break;
                    }
                }
                studioSearchIndex = closestIdx;
            } else {
                navigateSearchResult(navDirection);
                return;
            }

            highlightAndScrollMatch(studioSearchIndex);
        }

        function navigateSearchResult(direction) {
            if (studioSearchResults.length === 0) return;
            const total = studioSearchResults.length;
            studioSearchIndex = (studioSearchIndex + direction + total) % total;
            highlightAndScrollMatch(studioSearchIndex);
        }

        function highlightAndScrollMatch(index) {
            const editor = document.getElementById('csCodeEditor');
            const countEl = document.getElementById('csSearchCount');
            if (!editor || index < 0 || index >= studioSearchResults.length) return;

            const match = studioSearchResults[index];
            if (countEl) {
                countEl.innerText = `${index + 1}/${studioSearchResults.length}`;
            }

            editor.focus();
            try {
                editor.setSelectionRange(match.start, match.end);
            } catch(e) {}

            const textBefore = editor.value.substring(0, match.start);
            const line = (textBefore.match(/\n/g) || []).length;
            const lineHeight = 24;
            const targetScroll = Math.max(0, line * lineHeight - (editor.clientHeight / 2) + 24);
            editor.scrollTop = targetScroll;
            syncEditorScroll();
        }

        function handleSearchInputKeyDown(e) {
            if (e.key === 'Enter') {
                e.preventDefault();
                e.stopPropagation();
                if (e.shiftKey) {
                    navigateSearchResult(-1);
                } else {
                    navigateSearchResult(1);
                }
            } else if (e.key === 'Escape') {
                e.preventDefault();
                e.stopPropagation();
                closeStudioSearch();
            }
        }

        function handleReplaceInputKeyDown(e) {
            if (e.key === 'Enter') {
                e.preventDefault();
                e.stopPropagation();
                executeReplaceCurrent();
            } else if (e.key === 'Escape') {
                e.preventDefault();
                e.stopPropagation();
                closeStudioSearch();
            }
        }

        function executeReplaceCurrent() {
            const editor = document.getElementById('csCodeEditor');
            const replaceInput = document.getElementById('csReplaceInput');
            if (!editor || !replaceInput || studioSearchIndex < 0 || studioSearchIndex >= studioSearchResults.length) return;

            const replaceVal = replaceInput.value;
            const match = studioSearchResults[studioSearchIndex];

            const before = editor.value.substring(0, match.start);
            const after = editor.value.substring(match.end);
            editor.value = before + replaceVal + after;
            studioFileContents[activeStudioFile] = editor.value;

            if (activeStudioFile.endsWith('.html') || activeStudioFile.endsWith('.htm')) {
                astMapper.parse(editor.value);
            }
            onCodeEditorInput();
            pushStudioHistory(`Thay thế '${document.getElementById('csSearchInput')?.value || ''}'`, editor.value);

            runStudioSearch(0);
        }

        function executeReplaceAll() {
            const editor = document.getElementById('csCodeEditor');
            const searchInput = document.getElementById('csSearchInput');
            const replaceInput = document.getElementById('csReplaceInput');
            if (!editor || !searchInput || !replaceInput) return;

            const query = searchInput.value;
            const replaceVal = replaceInput.value;
            if (!query) return;

            const totalMatches = studioSearchResults.length;
            if (totalMatches === 0) return;

            let updatedText = '';
            if (studioSearchMatchCase) {
                updatedText = editor.value.split(query).join(replaceVal);
            } else {
                const regex = new RegExp(query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'gi');
                updatedText = editor.value.replace(regex, replaceVal);
            }

            editor.value = updatedText;
            studioFileContents[activeStudioFile] = updatedText;

            if (activeStudioFile.endsWith('.html') || activeStudioFile.endsWith('.htm')) {
                astMapper.parse(updatedText);
            }
            onCodeEditorInput();
            pushStudioHistory(`Thay thế tất cả (${totalMatches} chỗ) '${query}'`, updatedText);
            refreshStudioPreview();

            runStudioSearch(0);
            if (typeof showToastNotification === 'function') {
                showToastNotification(`Đã thay thế thành công ${totalMatches} kết quả`);
            }
        }

        // Bắt sự kiện click chuột hoặc di chuyển con trỏ trong Code Editor để đồng bộ ngược chọn Layer và Gizmo
        function onCodeEditorClick(e) {
            // Chỉ đồng bộ ngược AST khi đang mở file HTML (tránh offset nhảy bừa bãi khi đang xem CSS / JS)
            if (activeStudioFile && !activeStudioFile.endsWith('.html') && !activeStudioFile.endsWith('.htm')) return;
            const editor = document.getElementById('csCodeEditor');
            if (!editor || typeof findNodeAtOffset !== 'function' || typeof selectAstNode !== 'function') return;
            const offset = editor.selectionStart;
            const node = findNodeAtOffset(offset);
            if (node && node.id && node.id !== selectedAstNodeId) {
                // fromCodeEditor = true để TUYỆT ĐỐI không làm nhảy trỏ chuột, không bôi đen đè lên vị trí con trỏ của người dùng và không cuộn editor đi chỗ khác!
                selectAstNode(node.id, true, true);
            }
        }

        function onCodeEditorKeyUp(e) {
            if (['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'Home', 'End', 'PageUp', 'PageDown'].includes(e.key)) {
                onCodeEditorClick(e);
            }
        }

        // Gom toàn bộ nội dung file đang có trong File Manager thành mảng payload.
        // Bao gồm cả file đang chỉnh sửa dở trong editor (đảm bảo không mất thay đổi).
        //
        // CHỈ gửi file VĂN BẢN (html/css/js/json/txt...).
        // File nhị phân (ảnh, video, âm thanh, font, pdf) đã được lưu thẳng vào
        // bảng `landing_files` lúc upload → không cần (và không nên) gửi lại base64
        // trong JSON, vì có thể lên tới 100MB/tệp.
        // Backend tự đồng bộ file nhị phân sang domain đích khi deploy.
        function collectStudioFilesPayload() {
            const editor = document.getElementById('csCodeEditor');
            if (editor && activeStudioFile) {
                studioFileContents[activeStudioFile] = editor.value;
            }
            return Object.keys(studioFileContents)
                .filter(fp => !isBinaryStudioPath(fp))
                .map(fp => ({
                    file_path: fp,
                    file_type: '',
                    content: studioFileContents[fp] ?? ''
                }));
        }

        // Kiểm tra 1 đường dẫn file có phải file nhị phân (ảnh/video/audio/font/pdf) hay không
        function isBinaryStudioPath(filePath) {
            const ext = (filePath.match(/\.[a-z0-9]+$/i) || [''])[0].toLowerCase();
            return [
                '.png', '.jpg', '.jpeg', '.webp', '.gif', '.ico', '.bmp', '.avif',
                '.mp4', '.webm', '.ogg', '.mov', '.avi', '.mkv', '.m4v',
                '.mp3', '.wav', '.m4a', '.flac', '.aac',
                '.woff', '.woff2', '.ttf', '.otf', '.eot',
                '.pdf', '.zip', '.rar', '.7z', '.gz'
            ].includes(ext);
        }

        // Lưu mã nguồn (100% nguyên bản từ Shadow Source / AST, KHÔNG lấy từ DOM outerHTML)
        //
        // Yêu cầu 2026-11-04: lưu TOÀN BỘ file trong cây File Manager (không chỉ file
        // đang mở) — bao gồm ảnh/video/âm thanh đã upload vào thư mục, để khi Deploy
        // không bị thiếu tài nguyên.
        async function saveStudioCode() {
            if (!currentStudioLanding) return;
            const content = document.getElementById('csCodeEditor').value;
            studioFileContents[activeStudioFile] = content;

            const files = collectStudioFilesPayload();

            const btn = document.getElementById('btnSaveCode') || document.getElementById('btnSaveDraft');
            const origText = btn ? btn.innerHTML : '';
            if (btn) btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin mr-1"></i> Đang lưu mã...';

            try {
                const res = await fetch('/api/landings/files/save', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        domain: currentStudioLanding.domain,
                        page_id: currentStudioLanding.page_id,
                        files: files
                    })
                });
                if (!res.ok) throw new Error('Không thể lưu mã nguồn');
                const result = await res.json();

                studioEditorDirty = false;
                const n = result.saved != null ? result.saved : files.length;
                document.getElementById('csSaveStatus').innerText = `Đã lưu ${n} tệp lúc ${new Date().toLocaleTimeString('vi-VN')}`;
                document.getElementById('csSaveStatus').className = 'text-[11px] text-emerald-400 font-bold font-mono ml-2';
                refreshStudioPreview();
            } catch (e) {
                alert('Lỗi lưu mã nguồn: ' + e.message);
            } finally {
                if (btn) btn.innerHTML = origText;
            }
        }
        const saveCodeDraft = saveStudioCode;

        // Xuất bản chính thức (Deploy)
        //
        // Yêu cầu 2026-11-04: Deploy phải đưa TOÀN BỘ file trong thư mục lên domain
        // đích (ảnh, video, css, js...), không chỉ index.html.
        async function deployCodeLive() {
            if (!currentStudioLanding) return;
            const content = document.getElementById('csCodeEditor').value;
            studioFileContents[activeStudioFile] = content;

            let mainHtml = studioFileContents['index.html'] || content;
            if (activeStudioFile === 'index.html') {
                mainHtml = content;
            }

            const files = collectStudioFilesPayload();

            const btn = document.getElementById('btnDeployLive');
            const origText = btn.innerHTML;
            btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin mr-1"></i> Đang Deploy...';

            try {
                // 1. Lưu TOÀN BỘ file vào DB trước khi deploy (1 request duy nhất)
                const saveRes = await fetch('/api/landings/files/save', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        domain: currentStudioLanding.domain || '',
                        page_id: currentStudioLanding.page_id,
                        files: files
                    })
                });
                if (!saveRes.ok) throw new Error('Không thể lưu file trước khi deploy');

                // 2. Kích hoạt Deploy chính thức lên hệ thống Multi-Domain & Edge Router
                //    (kèm toàn bộ file để backend đồng bộ lên domain đích)
                const res = await fetch('/api/landings/deploy', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        domain: currentStudioLanding.domain || '',
                        page_id: currentStudioLanding.page_id,
                        code_html: mainHtml,
                        files: files
                    })
                });
                if (!res.ok) throw new Error('Deploy thất bại');
                const result = await res.json();

                studioEditorDirty = false;
                const finalUrl = (result.public_url && result.public_url.startsWith('http'))
                    ? result.public_url
                    : `${window.location.origin}${result.public_url || ('/p/' + currentStudioLanding.page_id)}`;

                const deployedCount = result.deployed_files != null ? result.deployed_files : files.length;
                const statusLabel = result.is_custom_domain
                    ? `🚀 ĐÃ DEPLOY ${deployedCount} TỆP LÊN TÊN MIỀN: ${result.domain}${result.path_prefix || ''}`
                    : `🚀 ĐÃ XUẤT BẢN ${deployedCount} TỆP THÀNH CÔNG (${new Date().toLocaleTimeString('vi-VN')})`;

                document.getElementById('csSaveStatus').innerText = statusLabel;
                document.getElementById('csSaveStatus').className = 'text-[11px] text-emerald-300 font-bold font-mono ml-2';

                refreshStudioPreview();
                if (confirm(`🎉 ${result.message}\n\nĐã deploy ${deployedCount} tệp tin.\nĐường dẫn trực tiếp: ${finalUrl}\n\nBạn có muốn mở trang web đã deploy trong tab mới để kiểm tra ngay không?`)) {
                    window.open(finalUrl, '_blank');
                }
            } catch (e) {
                alert('Lỗi Deploy: ' + e.message);
            } finally {
                btn.innerHTML = origText;
            }
        }

        async function restoreTemplateCode() {
            if (!confirm('Bạn có chắc chắn muốn khôi phục mã nguồn file này về giao diện mẫu chuẩn MXV 080 không? Toàn bộ sửa đổi chưa lưu sẽ bị ghi đè.')) return;
            try {
                const res = await fetch('/ladi.html');
                if (!res.ok) throw new Error('Không thể đọc file ladi.html mẫu');
                const html = await res.text();
                loadContentIntoEditor(html);
                onCodeEditorInput();
                refreshStudioPreview();
                alert('Đã khôi phục mã nguồn mẫu MXV 080. Hãy bấm "Lưu Code" hoặc "Deploy" để áp dụng.');
            } catch (e) {
                alert('Lỗi: ' + e.message);
            }
        }

        function setStudioDevice(mode) {
            studioDeviceMode = (mode === 'mobile') ? 'mobile' : 'pc';
            const btnPC = document.getElementById('btnDevicePC');
            const btnMobile = document.getElementById('btnDeviceMobile');
            const badge = document.getElementById('csDeviceTargetBadge');
            const wrapper = document.getElementById('csIframeWrapper');

            if (btnPC) {
                btnPC.className = (studioDeviceMode === 'pc')
                    ? 'px-2.5 py-1 rounded font-bold transition text-xs flex items-center gap-1.5 bg-cyan-600 text-white shadow-sm'
                    : 'px-2.5 py-1 rounded font-bold transition text-xs flex items-center gap-1.5 text-slate-300 hover:text-white';
            }
            if (btnMobile) {
                btnMobile.className = (studioDeviceMode === 'mobile')
                    ? 'px-2.5 py-1 rounded font-bold transition text-xs flex items-center gap-1.5 bg-cyan-600 text-white shadow-sm'
                    : 'px-2.5 py-1 rounded font-bold transition text-xs flex items-center gap-1.5 text-slate-300 hover:text-white';
            }

            if (badge) {
                if (studioDeviceMode === 'pc') {
                    badge.className = 'bg-cyan-950 text-cyan-300 border border-cyan-800 text-[10px] font-mono px-2 py-0.5 rounded font-bold hidden sm:inline-flex items-center gap-1';
                    badge.innerHTML = '<i class="fa-solid fa-laptop-code text-[9px]"></i> PC Mode';
                } else {
                    badge.className = 'bg-amber-950 text-amber-300 border border-amber-800 text-[10px] font-mono px-2 py-0.5 rounded font-bold hidden sm:inline-flex items-center gap-1';
                    badge.innerHTML = '<i class="fa-solid fa-mobile-screen text-[9px]"></i> Mobile Mode (Chỉ áp dụng Mobile)';
                }
            }

            if (wrapper) {
                wrapper.style.margin = 'auto';
                if (studioDeviceMode === 'pc') {
                    wrapper.style.width = '1440px'; // Kích thước màn hình PC thật (Real Desktop Resolution)
                    wrapper.style.height = '100%';
                    wrapper.style.maxWidth = 'none';
                    wrapper.style.flexShrink = '0';
                    wrapper.style.borderRadius = '12px';
                    wrapper.style.border = '1px solid #1e293b';
                    wrapper.style.boxShadow = '0 20px 25px -5px rgba(0, 0, 0, 0.5)';
                } else if (studioDeviceMode === 'mobile') {
                    wrapper.style.width = '390px'; // Kích thước màn hình Mobile chuẩn (iPhone 14/15/16 Pro)
                    wrapper.style.height = '844px';
                    wrapper.style.maxWidth = 'none';
                    wrapper.style.flexShrink = '0';
                    wrapper.style.borderRadius = '32px';
                    wrapper.style.border = '4px solid #475569';
                    wrapper.style.boxShadow = '0 25px 50px -12px rgba(0, 0, 0, 0.7)';
                }
                if (typeof canvasPanX !== 'undefined') {
                    canvasPanX = 0;
                    canvasPanY = 0;
                    if (typeof applyCanvasTransform === 'function') applyCanvasTransform();
                }
            }

            // Đồng bộ cập nhật lại Inspector theo đúng giao diện được chọn
            if (selectedAstNodeId && typeof astMapper !== 'undefined' && astMapper.astMap && astMapper.astMap[selectedAstNodeId]) {
                const node = astMapper.astMap[selectedAstNodeId];
                const nodeData = {
                    nodeId: node.id,
                    tagName: node.tag,
                    className: node.attrs['class'] ? node.attrs['class'].value : '',
                    text: node.textContent,
                    attributes: Object.fromEntries(Object.values(node.attrs).map(a => [a.name, a.value]))
                };
                populateInspectorWithNode(nodeData);
                setTimeout(() => requestNodeRect(selectedAstNodeId), 250);
            }
        }

        function toggleSplitPreview() {
            setStudioViewMode(studioViewMode === 'code' ? 'split' : 'code');
        }

        // Tải lại bản xem trước: Sử dụng Instrumented HTML trực tiếp từ AST (bảo toàn vị trí cuộn iframe)
        function refreshStudioPreview(preserveScroll = true) {
            if (!currentStudioLanding) return;
            const iframe = document.getElementById('csPreviewIframe');
            if (!iframe) return;

            let savedScrollTop = 0;
            let savedScrollLeft = 0;
            try {
                if (iframe.contentWindow) {
                    savedScrollTop = iframe.contentWindow.scrollY || (iframe.contentDocument && iframe.contentDocument.documentElement ? iframe.contentDocument.documentElement.scrollTop : 0) || 0;
                    savedScrollLeft = iframe.contentWindow.scrollX || (iframe.contentDocument && iframe.contentDocument.documentElement ? iframe.contentDocument.documentElement.scrollLeft : 0) || 0;
                }
            } catch(e) {}

            const htmlContent = studioFileContents[activeStudioFile] || (currentStudioLanding ? currentStudioLanding.code_html : '');
            if (activeStudioFile.endsWith('.html') || activeStudioFile.endsWith('.htm')) {
                // Truyền thông tin landing hiện tại vào astMapper để ghi đè
                // window.APEX_LANDING_CONFIG trong preview iframe. Nếu không,
                // tracker.js trong HTML template sẽ nạp config của landing khác.
                if (currentStudioLanding) {
                    astMapper.studioLandingId = currentStudioLanding.page_id || '';
                    astMapper.studioDomain = currentStudioLanding.domain || '';
                    astMapper.studioLandingName = currentStudioLanding.name || '';
                }
                astMapper.parse(htmlContent);
                const count = Object.keys(astMapper.astMap).length;
                const nodeCountEl = document.getElementById('csAstNodeCount');
                if (nodeCountEl) nodeCountEl.innerText = `AST Nodes: ${count}`;

                const instrumented = astMapper.generateInstrumentedHTML();
                iframe.srcdoc = instrumented;

                if (preserveScroll && (savedScrollTop > 0 || savedScrollLeft > 0)) {
                    const restoreScroll = () => {
                        try {
                            if (iframe.contentWindow) {
                                iframe.contentWindow.scrollTo(savedScrollLeft, savedScrollTop);
                            }
                        } catch(e) {}
                    };
                    iframe.addEventListener('load', restoreScroll, { once: true });
                }
            } else {
                iframe.src = `/api/landings/preview?page_id=${encodeURIComponent(currentStudioLanding.page_id)}&draft=1&bypass=1&preview=1&v=${Date.now()}`;
            }

            if (selectedAstNodeId) {
                setTimeout(() => requestNodeRect(selectedAstNodeId), 250);
            }
        }

        function openStudioPreviewNewTab() {
            if (!currentStudioLanding) return;
            window.open(`/api/landings/preview?page_id=${encodeURIComponent(currentStudioLanding.page_id)}&draft=1&bypass=1&preview=1`, '_blank');
        }

        // ============================================================
        // 17.14. QUẢN LÝ THƯ MỤC VÀ TỆP TIN (FOLDER & FILE MANAGER)
        // ============================================================
        function populateFolderDropdowns(selectedFolder = '') {
            const opts = ['<option value="">(Thư mục gốc /)</option>'];
            const sorted = [...studioFolders].sort();
            sorted.forEach(fld => {
                opts.push(`<option value="${escapeHTML(fld)}" ${fld === selectedFolder ? 'selected' : ''}>📁 /${escapeHTML(fld)}</option>`);
            });
            const selFolder = document.getElementById('newFolderParent');
            if (selFolder) selFolder.innerHTML = opts.join('');
            const selFile = document.getElementById('newFileParent');
            if (selFile) selFile.innerHTML = opts.join('');
            // Modal "Tải lên" cũng cần danh sách thư mục đích để chọn nơi lưu file.
            const selUpload = document.getElementById('studioUploadFolder');
            if (selUpload) selUpload.innerHTML = opts.join('');
        }

        // --- Quản lý Thư mục (Folder) ---
        function openNewFolderModal(parentFolder = '') {
            populateFolderDropdowns(parentFolder);
            document.getElementById('newFolderName').value = '';
            document.getElementById('newFolderModal').classList.remove('hidden');
            setTimeout(() => document.getElementById('newFolderName').focus(), 50);
        }

        function closeNewFolderModal() {
            document.getElementById('newFolderModal').classList.add('hidden');
        }

        async function createNewFolderConfirm() {
            if (!currentStudioLanding) return;
            const parent = document.getElementById('newFolderParent').value.trim();
            let name = document.getElementById('newFolderName').value.trim();
            if (!name) {
                alert('Vui lòng nhập tên thư mục');
                return;
            }
            name = name.replace(/^\/+|\/+$/g, '');
            const fullPath = parent ? `${parent}/${name}` : name;

            try {
                const res = await fetch('/api/landings/folder', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        domain: currentStudioLanding.domain,
                        page_id: currentStudioLanding.page_id,
                        folder_path: fullPath
                    })
                });
                if (!res.ok) {
                    const err = await res.text();
                    throw new Error(err || 'Không thể tạo thư mục');
                }

                if (!studioFolders.includes(fullPath)) {
                    studioFolders.push(fullPath);
                }
                folderExpandedState[fullPath] = true;
                closeNewFolderModal();
                renderStudioFileList();
            } catch (e) {
                alert('Lỗi tạo thư mục: ' + e.message);
            }
        }

        async function deleteFolderConfirm(folderPath) {
            if (!currentStudioLanding) return;
            if (!confirm(`⚠️ CẢNH BÁO: Bạn có chắc chắn muốn xóa thư mục '${folderPath}' không?\nToàn bộ tệp tin và thư mục con bên trong sẽ bị xóa vĩnh viễn!`)) {
                return;
            }

            try {
                const res = await fetch(`/api/landings/folder?page_id=${encodeURIComponent(currentStudioLanding.page_id)}&domain=${encodeURIComponent(currentStudioLanding.domain)}&folder_path=${encodeURIComponent(folderPath)}`, {
                    method: 'DELETE'
                });
                if (!res.ok) {
                    const err = await res.text();
                    throw new Error(err || 'Không thể xóa thư mục');
                }

                // Xóa thư mục và toàn bộ files con khỏi bộ nhớ client
                studioFolders = studioFolders.filter(f => f !== folderPath && !f.startsWith(folderPath + '/'));
                const prefix = folderPath + '/';
                let activeWasDeleted = false;
                Object.keys(studioFileContents).forEach(fp => {
                    if (fp.startsWith(prefix) || fp === folderPath) {
                        delete studioFileContents[fp];
                        if (activeStudioFile === fp) activeWasDeleted = true;
                    }
                });

                if (activeWasDeleted) {
                    switchStudioFile('index.html');
                } else {
                    renderStudioFileList();
                }
            } catch (e) {
                alert('Lỗi xóa thư mục: ' + e.message);
            }
        }

        // --- Đổi tên Thư mục hoặc Tệp tin ---
        function openRenameModal(type, oldPath) {
            currentRenameTarget = { type, oldPath };
            document.getElementById('renameOldPath').value = oldPath;
            document.getElementById('renameNewPath').value = oldPath;

            const iconEl = document.getElementById('renameModalIcon');
            const titleEl = document.getElementById('renameModalTitle');
            const hintEl = document.getElementById('renameHint');

            if (type === 'folder') {
                iconEl.className = 'fa-solid fa-folder-pen text-amber-400';
                titleEl.innerText = 'Đổi Tên Thư Mục';
                hintEl.innerText = 'Toàn bộ tệp tin và thư mục con bên trong sẽ tự động cập nhật đường dẫn';
            } else {
                iconEl.className = 'fa-solid fa-pen-to-square text-sky-400';
                titleEl.innerText = 'Đổi Tên Tệp Tin';
                hintEl.innerText = 'Tệp tin sẽ được cập nhật sang tên mới';
            }

            document.getElementById('renameItemModal').classList.remove('hidden');
            setTimeout(() => document.getElementById('renameNewPath').focus(), 50);
        }

        function closeRenameModal() {
            document.getElementById('renameItemModal').classList.add('hidden');
        }

        async function renameItemConfirm() {
            if (!currentStudioLanding) return;
            const { type, oldPath } = currentRenameTarget;
            let newPath = document.getElementById('renameNewPath').value.trim();
            if (!newPath) {
                alert('Vui lòng nhập đường dẫn mới');
                return;
            }
            newPath = newPath.replace(/^\/+|\/+$/g, '');
            if (newPath === oldPath) {
                closeRenameModal();
                return;
            }

            try {
                const endpoint = type === 'folder' ? '/api/landings/folder/rename' : '/api/landings/file/rename';
                const res = await fetch(endpoint, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        domain: currentStudioLanding.domain,
                        page_id: currentStudioLanding.page_id,
                        old_path: oldPath,
                        new_path: newPath
                    })
                });
                if (!res.ok) {
                    const err = await res.text();
                    throw new Error(err || 'Không thể đổi tên');
                }

                if (type === 'folder') {
                    // Cập nhật danh sách thư mục
                    studioFolders = studioFolders.map(f => {
                        if (f === oldPath) return newPath;
                        if (f.startsWith(oldPath + '/')) return newPath + f.substring(oldPath.length);
                        return f;
                    });
                    // Cập nhật key tệp tin con
                    const newContents = {};
                    Object.keys(studioFileContents).forEach(fp => {
                        if (fp.startsWith(oldPath + '/')) {
                            const updatedPath = newPath + fp.substring(oldPath.length);
                            newContents[updatedPath] = studioFileContents[fp];
                            if (activeStudioFile === fp) activeStudioFile = updatedPath;
                        } else {
                            newContents[fp] = studioFileContents[fp];
                        }
                    });
                    studioFileContents = newContents;
                } else {
                    // Đổi tên tệp tin đơn
                    studioFileContents[newPath] = studioFileContents[oldPath];
                    delete studioFileContents[oldPath];
                    if (activeStudioFile === oldPath) {
                        activeStudioFile = newPath;
                    }
                }

                closeRenameModal();
                document.getElementById('csActiveFileName').innerText = activeStudioFile;
                renderStudioFileList();
            } catch (e) {
                alert('Lỗi đổi tên: ' + e.message);
            }
        }

        // --- Quản lý Tệp Tin (File) ---
        function openNewFileModal(parentFolder = '') {
            populateFolderDropdowns(parentFolder);
            document.getElementById('newFileName').value = '';
            document.getElementById('newFileModal').classList.remove('hidden');
            setTimeout(() => document.getElementById('newFileName').focus(), 50);
        }

        function closeNewFileModal() {
            document.getElementById('newFileModal').classList.add('hidden');
        }

        async function createNewFileConfirm() {
            if (!currentStudioLanding) return;
            const parent = document.getElementById('newFileParent').value.trim();
            let name = document.getElementById('newFileName').value.trim();
            if (!name) {
                alert('Vui lòng nhập tên tệp tin');
                return;
            }
            name = name.replace(/^\/+|\/+$/g, '');
            const fullPath = parent ? `${parent}/${name}` : name;

            let defaultContent = '';
            if (name.endsWith('.css')) defaultContent = '/* Custom CSS */\n';
            else if (name.endsWith('.js')) defaultContent = '// Custom JS\n';
            else if (name.endsWith('.html') || name.endsWith('.htm')) defaultContent = '<div>Nội dung component</div>\n';
            else if (name.endsWith('.json')) defaultContent = '{\n  \n}\n';

            try {
                const res = await fetch('/api/landings/file', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        domain: currentStudioLanding.domain,
                        page_id: currentStudioLanding.page_id,
                        file_path: fullPath,
                        content: defaultContent
                    })
                });
                if (!res.ok) {
                    const err = await res.text();
                    throw new Error(err || 'Không thể tạo file');
                }

                studioFileContents[fullPath] = defaultContent;
                if (parent && !studioFolders.includes(parent)) {
                    studioFolders.push(parent);
                }
                closeNewFileModal();
                switchStudioFile(fullPath);
            } catch (e) {
                alert('Lỗi tạo file: ' + e.message);
            }
        }

        async function deleteStudioFile(filePath) {
            if (filePath === 'index.html') {
                alert('Không thể xóa tệp tin chính index.html');
                return;
            }
            if (!confirm(`Bạn có chắc chắn muốn xóa tệp tin '${filePath}' không?`)) return;

            try {
                const res = await fetch(`/api/landings/file?page_id=${encodeURIComponent(currentStudioLanding.page_id)}&domain=${encodeURIComponent(currentStudioLanding.domain)}&file_path=${encodeURIComponent(filePath)}`, {
                    method: 'DELETE'
                });
                if (!res.ok) throw new Error('Không thể xóa file');

                delete studioFileContents[filePath];
                if (activeStudioFile === filePath) {
                    switchStudioFile('index.html');
                } else {
                    renderStudioFileList();
                }
            } catch (e) {
                alert('Lỗi xóa file: ' + e.message);
            }
        }

        // ============================================================
        // 17.14b. TẢI TỆP TỪ MÁY LÊN SERVER (Upload Assets)
        // ------------------------------------------------------------
        // Yêu cầu 2026-11-04: nút upload trong File Explorer panel.
        // File được gửi lên endpoint /api/landings/files/upload với thư mục đích,
        // lưu vào bảng `landing_files` (domain rỗng = bản gốc + domain đích).
        // Sau đó Deploy sẽ đồng bộ TOÀN BỘ file lên domain đích.
        // ============================================================
        function openStudioUploadModal(parentFolder = '') {
            if (!currentStudioLanding) {
                alert('Vui lòng mở một Landing Page trước khi tải tệp lên.');
                return;
            }
            populateFolderDropdowns(parentFolder);
            const sel = document.getElementById('studioUploadFolder');
            if (sel) {
                sel.value = parentFolder || '';
            }
            const input = document.getElementById('studioUploadInput');
            if (input) input.value = '';
            const prog = document.getElementById('studioUploadProgress');
            if (prog) prog.classList.add('hidden');
            const btn = document.getElementById('btnStudioUploadConfirm');
            if (btn) {
                btn.disabled = false;
                btn.innerHTML = 'Tải Lên';
            }
            document.getElementById('studioUploadModal').classList.remove('hidden');
        }

        function closeStudioUploadModal() {
            document.getElementById('studioUploadModal').classList.add('hidden');
        }

        /** Đồng bộ cây thư mục + nội dung file từ server sau khi upload. */
        async function reloadStudioFilesAfterMutation() {
            try {
                const res = await fetch(`/api/landings/files?page_id=${encodeURIComponent(currentStudioLanding.page_id)}&domain=${encodeURIComponent(currentStudioLanding.domain || '')}`);
                if (!res.ok) return;
                const data = await res.json();
                const files = data.files || [];

                // Giữ lại nội dung đang chỉnh sửa trong editor (tránh mất dữ liệu)
                const pending = {};
                if (studioEditorDirty && activeStudioFile) {
                    const editor = document.getElementById('csCodeEditor');
                    if (editor) pending[activeStudioFile] = editor.value;
                }

                studioFolders = [];
                const nextContents = {};
                files.forEach(f => {
                    if (f.file_type === 'folder') {
                        if (!studioFolders.includes(f.file_path)) studioFolders.push(f.file_path);
                    } else {
                        nextContents[f.file_path] = f.content || '';
                    }
                });

                // Ghi đè bằng nội dung đang sửa dở (nếu có)
                Object.keys(pending).forEach(k => { nextContents[k] = pending[k]; });

                studioFileContents = nextContents;
                if (!studioFileContents['index.html'] && currentStudioLanding.code_html) {
                    studioFileContents['index.html'] = currentStudioLanding.code_html;
                }
                renderStudioFileList();
            } catch (_) { /* bỏ qua lỗi refresh phụ */ }
        }

        async function submitStudioUpload() {
            if (!currentStudioLanding) return;

            const input = document.getElementById('studioUploadInput');
            const folder = (document.getElementById('studioUploadFolder')?.value || '').trim();
            if (!input || !input.files || input.files.length === 0) {
                alert('Vui lòng chọn ít nhất một tệp tin để tải lên.');
                return;
            }

            const btn = document.getElementById('btnStudioUploadConfirm');
            const origBtnText = btn ? btn.innerHTML : '';
            const prog = document.getElementById('studioUploadProgress');
            const bar = document.getElementById('studioUploadBar');
            const status = document.getElementById('studioUploadStatus');

            if (btn) { btn.disabled = true; btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin mr-1"></i> Đang tải...'; }
            if (prog) prog.classList.remove('hidden');
            if (bar) bar.style.width = '0%';
            if (status) status.innerText = 'Đang tải lên...';

            const formData = new FormData();
            formData.append('page_id', currentStudioLanding.page_id);
            formData.append('domain', currentStudioLanding.domain || '');
            formData.append('folder', folder);
            for (let i = 0; i < input.files.length; i++) {
                formData.append('files', input.files[i]);
            }

            try {
                const res = await fetch('/api/landings/files/upload', {
                    method: 'POST',
                    body: formData
                });
                if (!res.ok) {
                    const errText = await res.text();
                    throw new Error(errText || 'Không thể tải tệp lên');
                }
                const result = await res.json();

                if (bar) bar.style.width = '100%';
                if (status) status.innerText = result.message || 'Tải lên thành công';

                await reloadStudioFilesAfterMutation();

                closeStudioUploadModal();
                if (typeof showToastNotification === 'function') {
                    showToastNotification(result.message || 'Đã tải lên tệp tin', 'success');
                } else {
                    alert(result.message || 'Đã tải lên tệp tin');
                }
            } catch (e) {
                if (status) status.innerText = 'Lỗi: ' + e.message;
                alert('Lỗi tải tệp lên: ' + e.message);
            } finally {
                if (btn) {
                    btn.disabled = false;
                    btn.innerHTML = origBtnText;
                }
            }
        }

        // 17.15. Standalone Preview Modal (3 Kiểu xem)
        let currentModalPreviewPageId = '';
        let currentModalPreviewDomain = '';
