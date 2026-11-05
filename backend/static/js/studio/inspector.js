/* ==============================================================
   APEX VISUAL WEB BUILDER - 6 INSPECTOR TABS & PROPERTY CONTROLLERS
   (Layout, Typography, Style, Spacing, Classes, Attributes)
   ============================================================== */

        // ============================================================
        // GLOBAL VISUAL BUILDER CONTROLLER & STATE
        // ============================================================
        var astMapper = new ApexASTSourceMapper();
        window.astMapper = astMapper;
        var studioFolders = [];
        var folderExpandedState = {};
        var currentRenameTarget = { type: 'file', oldPath: '' };
        var studioViewMode = 'split'; // 'code' | 'split' | 'visual'
        var studioFreezeMode = true; // true = Design Freeze, false = Live Interactive
        var studioInspectorOpen = true;
        var studioDeviceMode = 'pc'; // 'pc' | 'mobile' (Responsive Scope)
        window.studioDeviceMode = studioDeviceMode;
        var selectedAstNodeId = null;
        var selectedNodeMeta = null;
        var currentInspTab = 'content';
        window.currentInspTab = currentInspTab;
        window.currentInspectorTab = currentInspTab;
        var isDraggingGizmo = false;
        var isMovingGizmo = false;
        var currentResizeDir = '';
        var resizeStartRect = null;
        var resizeStartX = 0, resizeStartY = 0;
        var moveStartX = 0, moveStartY = 0;
        var moveStartLeft = 0, moveStartTop = 0;
        var moveInitialStyleLeft = 0, moveInitialStyleTop = 0;
        var activeCapturedElement = null;
        var activePointerId = null;
        var editorDebounceTimer = null;

        // Photoshop History Stack Architecture & Clipboard (declared in history.js)
        let editorHistoryTimer = null;

        let studioClipboardNode = null;
        let studioClipboardStyle = null;
        let studioLockedNodes = new Set();

        let layerFilterKeyword = '';
        let layerFilterCategory = 'all';
        let layerExpandedState = {};

        // Lắng nghe sự kiện truyền tin postMessage từ Sandboxed Iframe Preview
        window.addEventListener('message', function(ev) {
            if (!ev.data || typeof ev.data !== 'object') return;
            const msg = ev.data;

            if (msg.type === 'APEX_BUILDER_CONTEXT_MENU') {
                const iframe = document.getElementById('csPreviewIframe');
                if (iframe) {
                    const rect = iframe.getBoundingClientRect();
                    // clientX/clientY gửi từ trong iframe là CSS px nội bộ ⇒ nhân tỉ lệ scale
                    const zoom = (typeof window.canvasRenderScale === 'number' && window.canvasRenderScale > 0) ? window.canvasRenderScale : 1.0;
                    const absX = rect.left + (msg.clientX * zoom);
                    const absY = rect.top + (msg.clientY * zoom);
                    if (msg.nodeId) {
                        selectAstNode(msg.nodeId);
                        openContextMenu(absX, absY, msg.nodeId);
                    } else if (selectedAstNodeId) {
                        openContextMenu(absX, absY, selectedAstNodeId);
                    }
                }
            }

            if (msg.type === 'APEX_BUILDER_READY') {
                const iframe = document.getElementById('csPreviewIframe');
                if (iframe && iframe.contentWindow) {
                    iframe.contentWindow.postMessage({ type: 'APEX_CMD_SET_FREEZE_MODE', enabled: studioFreezeMode }, '*');
                }
                if (selectedAstNodeId) {
                    requestNodeRect(selectedAstNodeId);
                }
            }

            if (msg.type === 'APEX_BUILDER_NODE_HOVERED') {
                if (isDraggingGizmo || isMovingGizmo || !studioFreezeMode || window.isSpaceKeyPressed || window.isCanvasPanning) return;
                const hoverBox = document.getElementById('csHoverBox');
                const hoverTag = document.getElementById('csHoverTag');
                if (hoverBox && hoverTag && msg.rect) {
                    // Tọa độ trong iframe là CSS px nội bộ ⇒ nhân tỉ lệ scale cho khớp khung hiển thị
                    const zoom = (typeof window.canvasRenderScale === 'number' && window.canvasRenderScale > 0) ? window.canvasRenderScale : 1.0;
                    hoverBox.style.top = Math.round(msg.rect.top * zoom) + 'px';
                    hoverBox.style.left = Math.round(msg.rect.left * zoom) + 'px';
                    hoverBox.style.width = Math.round(msg.rect.width * zoom) + 'px';
                    hoverBox.style.height = Math.round(msg.rect.height * zoom) + 'px';
                    const classSummary = msg.className ? '.' + msg.className.trim().split(/\s+/).slice(0, 2).join('.') : '';
                    hoverTag.innerText = `<${msg.tagName}${classSummary}>`;
                    hoverBox.classList.remove('hidden');
                }
            }

            if (msg.type === 'APEX_BUILDER_HOVER_LEAVE') {
                const hoverBox = document.getElementById('csHoverBox');
                if (hoverBox) hoverBox.classList.add('hidden');
            }

            if (msg.type === 'APEX_BUILDER_NODE_CLICKED') {
                // Tuyệt đối không chọn bất kỳ phần tử/lớp nào khi đang ấn phím Space hoặc đang kéo Pan
                if (window.isSpaceKeyPressed || window.isCanvasPanning) {
                    return;
                }
                const shield = document.getElementById('csCanvasPanShield');
                if (shield && typeof isCanvasPanning !== 'undefined' && !isCanvasPanning) {
                    shield.classList.add('hidden');
                    shield.style.pointerEvents = 'none';
                    shield.style.cursor = '';
                }
                handleNodeSelection(msg.data);
            }

            if (msg.type === 'APEX_BUILDER_PAN_START') {
                if (typeof startCanvasPanFromIframe === 'function') {
                    startCanvasPanFromIframe(msg);
                }
            }

            if (msg.type === 'APEX_BUILDER_PAN_MOVE') {
                const viewport = document.getElementById('csCanvasViewport');
                if (viewport && (viewport.scrollWidth > viewport.clientWidth || viewport.scrollHeight > viewport.clientHeight)) {
                    viewport.scrollBy({ left: -msg.dx, top: -msg.dy, behavior: 'instant' });
                }
            }

            if (msg.type === 'APEX_BUILDER_PAN_END') {
                if (typeof endCanvasPan === 'function') {
                    endCanvasPan();
                }
            }

            if (msg.type === 'APEX_BUILDER_NODE_DBLCLICKED') {
                if (window.isSpaceKeyPressed || window.isCanvasPanning) {
                    return;
                }
                handleNodeSelection(msg.data);
                startInlineTextEdit();
            }

            if (msg.type === 'APEX_BUILDER_VIEWPORT_CHANGED') {
                if (selectedAstNodeId && !window.isCanvasPanning && !window.isSpaceKeyPressed) {
                    requestNodeRect(selectedAstNodeId);
                }
            }

            if (msg.type === 'APEX_BUILDER_RECT_RESPONSE') {
                if (msg.data && msg.data.rect) {
                    repositionGizmoBox(msg.data.rect);
                }
            }

            if (msg.type === 'APEX_BUILDER_INLINE_EDIT_FINISHED') {
                if (msg.nodeId && msg.newText !== undefined) {
                    applyAstTextChange(msg.nodeId, msg.newText);
                }
            }

            if (msg.type === 'APEX_BUILDER_IFRAME_MOUSEUP') {
                if (isDraggingGizmo || isMovingGizmo) {
                    endGizmoDragOrResize();
                }
                if (typeof endCanvasPan === 'function' && isCanvasPanning) {
                    endCanvasPan();
                }
            }

            if (msg.type === 'APEX_BUILDER_KEY_SPACE') {
                if (typeof setSpacePanState === 'function') {
                    setSpacePanState(!!msg.pressed);
                }
            }

            if (msg.type === 'APEX_BUILDER_KEY_CTRL') {
                if (typeof setCtrlPanState === 'function') {
                    setCtrlPanState(!!msg.pressed);
                }
            }

            if (msg.type === 'APEX_BUILDER_WHEEL_ZOOM') {
                if (typeof stepCanvasZoom === 'function') {
                    stepCanvasZoom(msg.deltaY < 0 ? 0.08 : -0.08);
                }
            }
        });

        function expandAncestorsInLayerTree(nodeId) {
            window.layerExpandedState = window.layerExpandedState || {};
            if (!astMapper || !astMapper.astMap || !nodeId) return;
            let cur = astMapper.astMap[nodeId];
            while (cur && cur.parentId) {
                window.layerExpandedState[cur.parentId] = true;
                layerExpandedState[cur.parentId] = true;
                cur = astMapper.astMap[cur.parentId];
            }
        }

        function requestNodeRect(nodeId, scrollIntoView = false) {
            const iframe = document.getElementById('csPreviewIframe');
            if (iframe && iframe.contentWindow) {
                iframe.contentWindow.postMessage({ type: 'APEX_CMD_GET_NODE_RECT', nodeId: nodeId, scrollIntoView: scrollIntoView }, '*');
            }
            try {
                if (iframe && iframe.contentDocument) {
                    const el = iframe.contentDocument.querySelector('[data-ve-id="' + nodeId + '"]');
                    if (el) {
                        if (scrollIntoView) {
                            try { el.scrollIntoView({ behavior: 'smooth', block: 'center', inline: 'nearest' }); } catch(e) {}
                        }
                        const r = el.getBoundingClientRect();
                        repositionGizmoBox({ top: r.top, left: r.left, width: r.width, height: r.height });
                    }
                }
            } catch(e) {}
        }

        function handleNodeSelection(data) {
            if (!data || !data.nodeId) return;
            selectedAstNodeId = data.nodeId;
            selectedNodeMeta = data;

            // Ẩn hover box
            const hoverBox = document.getElementById('csHoverBox');
            if (hoverBox) hoverBox.classList.add('hidden');

            // Cập nhật vị trí Gizmo
            if (data.rect) {
                repositionGizmoBox(data.rect);
            }

            // Cập nhật bảng thuộc tính Visual Inspector theo loại thẻ
            populateInspectorWithNode(data);

            // Đồng bộ cây Layer DOM: mở bung toàn bộ cây cha và cuộn đến dòng layer tương ứng
            expandAncestorsInLayerTree(data.nodeId);
            renderAstLayerTree();
            setTimeout(() => {
                const row = document.getElementById('layer-row-' + data.nodeId);
                if (row) {
                    row.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
                }
            }, 50);

            // Giữ nguyên tab nếu người dùng đang ở tab 'layers' hoặc 'css', ngược lại mới tự động chuyển sang 'content'
            if (currentInspTab === 'layers') {
                if (typeof renderAstLayerTree === 'function') renderAstLayerTree();
            } else if (currentInspTab === 'css') {
                if (typeof renderCssInspectorTab === 'function') renderCssInspectorTab();
            } else {
                switchInspectorTab('content');
            }

            // Mở tự động Inspector nếu đang ở Visual Builder
            if (studioViewMode === 'visual' && !studioInspectorOpen) {
                toggleInspectorPanel();
            }

            // Highlight dòng code và nhảy mượt đến thẻ trong Code Editor
            syncAndHighlightCodeForNode(data.nodeId);
        }

        function repositionGizmoBox(rect) {
            const gizmoBox = document.getElementById('csGizmoBox');
            const gizmoTag = document.getElementById('csGizmoTag');
            const gizmoSize = document.getElementById('csGizmoSize');
            const btnImg = document.getElementById('btnGizmoChangeImg');
            const btnLink = document.getElementById('btnGizmoChangeLink');

            if (!gizmoBox || !rect || isDraggingGizmo || isMovingGizmo) return;

            // ⭐ Zoom thật (2026-11-05): iframe được `transform: scale(...)`,
            // nên tọa độ lấy từ trong iframe là CSS px NỘI BỘ (chưa scale).
            // ⇒ Phải nhân với tỉ lệ scale thực tế mới khớp với khung hiển thị bên ngoài.
            const zoom = (typeof window.canvasRenderScale === 'number' && window.canvasRenderScale > 0) ? window.canvasRenderScale : 1.0;

            gizmoBox.style.top = Math.round(rect.top * zoom) + 'px';
            gizmoBox.style.left = Math.round(rect.left * zoom) + 'px';
            gizmoBox.style.width = Math.max(10, Math.round(rect.width * zoom)) + 'px';
            gizmoBox.style.height = Math.max(10, Math.round(rect.height * zoom)) + 'px';

            const tag = selectedNodeMeta ? selectedNodeMeta.tagName : 'div';
            const classSummary = selectedNodeMeta && selectedNodeMeta.className ? '.' + selectedNodeMeta.className.trim().split(/\s+/).slice(0, 2).join('.') : '';
            const isLocked = studioLockedNodes.has(selectedAstNodeId);
            if (gizmoTag) {
                gizmoTag.innerHTML = `${isLocked ? '<i class="fa-solid fa-lock text-yellow-400 mr-1"></i>' : ''}${tag.toUpperCase()} ${classSummary}`;
            }
            // rect đã là CSS px thật của landing ⇒ hiển thị thẳng, không chia zoom
            if (gizmoSize) gizmoSize.innerText = `${Math.round(rect.width)} × ${Math.round(rect.height)}px`;

            const handles = gizmoBox.querySelectorAll('.cs-gizmo-handle');
            handles.forEach(h => h.style.display = isLocked ? 'none' : '');

            if (btnImg) btnImg.classList.toggle('hidden', tag !== 'img');
            if (btnLink) btnLink.classList.toggle('hidden', tag !== 'a');

            gizmoBox.classList.remove('hidden');
        }

        function highlightCodeAtOffset(offset) {
            const editor = document.getElementById('csCodeEditor');
            if (!editor) return;
            const textBefore = editor.value.substring(0, offset);
            const line = (textBefore.match(/\n/g) || []).length;
            const lineHeight = 24; // 24px line-height
            editor.scrollTop = Math.max(0, line * lineHeight - 100);
            updateEditorStats(false);
        }

        // Tự động nhảy và highlight vùng code chính xác của thẻ trong Code Editor
        function syncAndHighlightCodeForNode(nodeId) {
            // Chỉ highlight khi đang xem file HTML (tránh lệch offset khi đang xem CSS / JS)
            if (typeof activeStudioFile !== 'undefined' && activeStudioFile && !activeStudioFile.endsWith('.html') && !activeStudioFile.endsWith('.htm')) {
                return;
            }
            const editor = document.getElementById('csCodeEditor');
            if (!editor) return;
            const node = (typeof astMapper !== 'undefined' && astMapper.astMap) ? astMapper.astMap[nodeId] : null;
            if (!node || node.startOffset === undefined) return;

            const code = editor.value;
            const startOffset = Math.min(node.startOffset, code.length);
            // QUAN TRỌNG: Chỉ chọn/highlight phần thẻ mở <tag ...> chứ KHÔNG bôi đen đến tận thẻ đóng
            // để tránh nhảy vọt xuống cuối khối làm lệch vị trí hiển thị code
            const endOffset = Math.min(node.openTagEnd || (node.startOffset + 15), code.length);

            // Tính số dòng bắt đầu
            const textBefore = code.substring(0, startOffset);
            const startLine = (textBefore.match(/\n/g) || []).length + 1;

            // Highlight vùng code thẻ mở trong textarea
            try {
                editor.setSelectionRange(startOffset, endOffset);
            } catch(e) {}

            // Cuộn mượt code editor đến đúng dòng của thẻ (cách đỉnh 3 dòng)
            const lineHeight = 24; // 24px line-height
            const targetScrollTop = Math.max(0, (startLine - 3) * lineHeight);
            editor.scrollTop = targetScrollTop;

            // Đồng bộ thanh số dòng với dải màu cyan nổi bật
            if (typeof updateEditorStats === 'function') {
                updateEditorStats(false, { startLine, endLine: startLine, startOffset, endOffset });
            }

            // Cập nhật thông tin dòng trên status bar
            const lineColEl = document.getElementById('csLineCol');
            if (lineColEl) {
                lineColEl.innerHTML = `<span class="text-cyan-300 font-bold">Dòng ${startLine}</span> | <span class="text-amber-300 font-mono">&lt;${node.tag || ''}&gt;</span>`;
            }

            // Nháy sáng nhẹ viền editor container để báo hiệu đã nhảy đến code thẻ
            const editorContainer = document.getElementById('csEditorContainer');
            if (editorContainer) {
                editorContainer.classList.add('ring-1', 'ring-cyan-400');
                setTimeout(() => {
                    editorContainer.classList.remove('ring-1', 'ring-cyan-400');
                }, 800);
            }
        }

        // Tìm node AST chính xác bao bọc vị trí con trỏ chuột trong Code Editor
        function findNodeAtOffset(offset) {
            if (!astMapper || !astMapper.astMap || offset === undefined || offset === null) return null;
            let target = null;
            let minLen = Infinity;
            for (let id in astMapper.astMap) {
                const node = astMapper.astMap[id];
                if (node.startOffset !== undefined) {
                    const end = node.endOffset !== undefined ? node.endOffset : (node.openTagEnd || (node.startOffset + 10));
                    if (node.startOffset <= offset && offset <= end) {
                        const len = end - node.startOffset;
                        if (len < minLen) {
                            minLen = len;
                            target = node;
                        }
                    }
                }
            }
            return target;
        }

        // Chuyển đổi 3 chế độ xem Studio: Code Only | Split 2-Way | Visual Builder
        function setStudioViewMode(mode) {
            studioViewMode = mode;
            const btnCode = document.getElementById('btnModeCode');
            const btnSplit = document.getElementById('btnModeSplit');
            const btnVisual = document.getElementById('btnModeVisual');
            const editorPanel = document.getElementById('csEditorContainer');
            const previewPanel = document.getElementById('csPreviewPanel');
            const inspectorPanel = document.getElementById('csVisualInspector');

            [btnCode, btnSplit, btnVisual].forEach(b => {
                if (b) b.className = 'px-2.5 py-1 rounded-lg font-bold transition flex items-center gap-1.5 text-slate-400 hover:text-white';
            });

            if (mode === 'code') {
                if (btnCode) btnCode.className = 'px-2.5 py-1 rounded-lg font-bold transition flex items-center gap-1.5 bg-cyan-600 text-white shadow';
                if (editorPanel) editorPanel.classList.remove('hidden');
                if (previewPanel) previewPanel.classList.add('hidden');
                if (inspectorPanel) inspectorPanel.classList.add('hidden');
            } else if (mode === 'split') {
                if (btnSplit) btnSplit.className = 'px-2.5 py-1 rounded-lg font-bold transition flex items-center gap-1.5 bg-cyan-600 text-white shadow';
                if (editorPanel) {
                    editorPanel.classList.remove('hidden');
                    editorPanel.style.flex = '1';
                }
                if (previewPanel) {
                    previewPanel.classList.remove('hidden');
                    previewPanel.style.flex = '1';
                }
                if (inspectorPanel) {
                    inspectorPanel.classList.toggle('hidden', !studioInspectorOpen);
                }
            } else if (mode === 'visual') {
                if (btnVisual) btnVisual.className = 'px-2.5 py-1 rounded-lg font-bold transition flex items-center gap-1.5 bg-cyan-600 text-white shadow';
                if (editorPanel) editorPanel.classList.add('hidden');
                if (previewPanel) {
                    previewPanel.classList.remove('hidden');
                    previewPanel.style.flex = '1';
                }
                if (inspectorPanel) {
                    inspectorPanel.classList.remove('hidden');
                    studioInspectorOpen = true;
                }
            }

            if (selectedAstNodeId) {
                setTimeout(() => requestNodeRect(selectedAstNodeId), 100);
            }
        }

        // Bật/tắt Chế độ Thiết Kế (Design Freeze vs Interactive Mode)
        function toggleStudioFreezeMode() {
            studioFreezeMode = !studioFreezeMode;
            const btn = document.getElementById('btnToggleFreeze');
            const icon = document.getElementById('iconToggleFreeze');
            const text = document.getElementById('textToggleFreeze');
            const badge = document.getElementById('csFreezeStatusBadge');
            const gizmoBox = document.getElementById('csGizmoBox');
            const hoverBox = document.getElementById('csHoverBox');

            if (studioFreezeMode) {
                if (btn) btn.className = 'bg-indigo-950/80 hover:bg-indigo-900 text-indigo-300 border border-indigo-800 px-2.5 py-1 rounded-xl text-xs font-bold transition flex items-center gap-1.5 shadow-sm';
                if (icon) icon.className = 'fa-solid fa-snowflake text-cyan-400';
                if (text) text.innerText = 'Thiết Kế (Freeze)';
                if (badge) {
                    badge.innerText = '🧊 Freeze Active';
                    badge.className = 'bg-indigo-950 text-indigo-300 border border-indigo-800 text-[10px] font-mono font-bold px-2 py-0.5 rounded-md hidden lg:inline';
                }
            } else {
                if (btn) btn.className = 'bg-emerald-950/80 hover:bg-emerald-900 text-emerald-300 border border-emerald-800 px-2.5 py-1 rounded-xl text-xs font-bold transition flex items-center gap-1.5 shadow-sm';
                if (icon) icon.className = 'fa-solid fa-bolt text-amber-400';
                if (text) text.innerText = 'Tương Tác (Live)';
                if (badge) {
                    badge.innerText = '⚡ Live Interactive';
                    badge.className = 'bg-emerald-950 text-emerald-300 border border-emerald-800 text-[10px] font-mono font-bold px-2 py-0.5 rounded-md hidden lg:inline';
                }
                if (gizmoBox) gizmoBox.classList.add('hidden');
                if (hoverBox) hoverBox.classList.add('hidden');
            }

            const iframe = document.getElementById('csPreviewIframe');
            if (iframe && iframe.contentWindow) {
                iframe.contentWindow.postMessage({ type: 'APEX_CMD_SET_FREEZE_MODE', enabled: studioFreezeMode }, '*');
            }
        }

        // Bật/tắt Panel Thuộc Tính
        function toggleInspectorPanel() {
            const panel = document.getElementById('csVisualInspector');
            if (!panel) return;
            studioInspectorOpen = !studioInspectorOpen;
            panel.classList.toggle('hidden', !studioInspectorOpen);
        }

        // Chuyển đổi Tab trong Visual Inspector (Hỗ trợ 5 Tab: Content, CSS, Styles, Classes, Layers)
        function switchInspectorTab(tab) {
            currentInspTab = tab;
            window.currentInspTab = tab;
            window.currentInspectorTab = tab;
            const tabDefs = [
                { id: 'content', btn: 'btnInspTabContent', sec: 'inspSectionContent' },
                { id: 'css', btn: 'btnInspTabCss', sec: 'inspSectionCss' },
                { id: 'styles', btn: 'btnInspTabStyles', sec: 'inspSectionStyles' },
                { id: 'classes', btn: 'btnInspTabClasses', sec: 'inspSectionClasses' },
                { id: 'layers', btn: 'btnInspTabLayers', sec: 'inspSectionLayers' }
            ];
            tabDefs.forEach(t => {
                const btn = document.getElementById(t.btn);
                const sec = document.getElementById(t.sec);
                if (btn) {
                    if (t.id === tab) {
                        btn.className = 'flex-1 py-2 font-bold text-center border-b-2 border-cyan-400 text-white transition flex items-center justify-center gap-1';
                    } else {
                        btn.className = 'flex-1 py-2 font-semibold text-center text-slate-400 hover:text-white transition flex items-center justify-center gap-1';
                    }
                }
                if (sec) sec.classList.toggle('hidden', t.id !== tab);
            });

            if (tab === 'layers') {
                if (selectedAstNodeId && typeof expandAncestorsInLayerTree === 'function') {
                    expandAncestorsInLayerTree(selectedAstNodeId);
                }
                renderAstLayerTree();
                if (selectedAstNodeId) {
                    setTimeout(() => {
                        const row = document.getElementById('layer-row-' + selectedAstNodeId);
                        if (row) row.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
                    }, 50);
                }
            } else if (tab === 'css') {
                renderCssInspectorTab();
            }
        }
        const openVisualInspectorTab = switchInspectorTab;

        // Nhận diện kiểu loại thẻ HTML để hiển thị giao diện tùy chỉnh phù hợp
        function detectElementType(tagName, attrs, className) {
            const tag = (tagName || '').toLowerCase().trim();
            const cls = (className || '').toLowerCase();
            const role = (attrs && attrs.role ? attrs.role : '').toLowerCase();

            if (tag === 'img') return 'image';
            if (tag === 'button' || role === 'button' || (tag === 'a' && /\bbtn\b|\bbutton\b/.test(cls))) return 'button';
            if (tag === 'a') return 'link';
            if (['input', 'textarea', 'select', 'form'].includes(tag)) return 'input';
            if (['video', 'iframe'].includes(tag)) return 'video';
            if (['h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'p', 'span', 'strong', 'em', 'b', 'i', 'blockquote', 'li', 'label'].includes(tag)) return 'typography';
            if (['div', 'section', 'header', 'footer', 'main', 'article', 'aside', 'nav', 'ul', 'ol', 'figure', 'body'].includes(tag)) return 'container';
            return 'typography';
        }

        // Điền dữ liệu node vào Visual Inspector và hiển thị đúng subpanel theo loại thẻ
        function populateInspectorWithNode(data) {
            const tag = (data.tagName || 'element').toLowerCase();
            const attrs = data.attributes || {};
            const className = data.className || '';
            const nodeId = data.nodeId || '';

            const badgeTag = document.getElementById('inspTagBadge');
            if (badgeTag) badgeTag.innerText = tag;
            const badgeId = document.getElementById('inspIdBadge');
            if (badgeId) badgeId.innerText = '#' + nodeId;

            // Breadcrumb navigation
            const breadcrumbEl = document.getElementById('inspBreadcrumb');
            if (breadcrumbEl && nodeId) {
                const path = astMapper.getNodePath(nodeId);
                let bHtml = '';
                path.forEach((p, idx) => {
                    const isLast = idx === path.length - 1;
                    const tagCls = isLast ? 'text-cyan-400 font-bold' : 'text-slate-400 hover:text-white cursor-pointer';
                    bHtml += `<span class="${tagCls}" onclick="selectAstNode('${p.id}')">${p.tag}</span>`;
                    if (!isLast) bHtml += '<span class="text-slate-600">/</span>';
                });
                breadcrumbEl.innerHTML = bHtml || '<span>body</span>';
            }

            // 1. Phân loại phần tử và kích hoạt đúng subpanel trong Tab Nội Dung
            const elType = detectElementType(tag, attrs, className);
            const panels = [
                { id: 'typography', el: document.getElementById('inspPanelTypography') },
                { id: 'button', el: document.getElementById('inspPanelButton') },
                { id: 'link', el: document.getElementById('inspPanelLink') },
                { id: 'image', el: document.getElementById('inspPanelImage') },
                { id: 'input', el: document.getElementById('inspPanelInput') },
                { id: 'video', el: document.getElementById('inspPanelVideo') },
                { id: 'container', el: document.getElementById('inspPanelContainer') }
            ];

            panels.forEach(p => {
                if (p.el) p.el.classList.toggle('hidden', p.id !== elType);
            });

            // Tiêu đề, Subtitle và Badge của loại phần tử
            const typeTitleEl = document.getElementById('inspTypeTitle');
            const typeSubEl = document.getElementById('inspTypeSubtitle');
            const typeBadgeEl = document.getElementById('inspTypeTagBadge');
            if (typeSubEl) typeSubEl.innerText = `<${tag}> • #${nodeId}`;
            if (typeBadgeEl) typeBadgeEl.innerText = tag.toUpperCase();

            // Điền dữ liệu chi tiết cho từng loại subpanel
            if (elType === 'typography') {
                if (typeTitleEl) typeTitleEl.innerText = 'Văn Bản & Tiêu Đề';
                const textInput = document.getElementById('inspTextContent');
                if (textInput) textInput.value = data.text || '';
                const tagSelect = document.getElementById('inspTypographyTag');
                if (tagSelect) {
                    tagSelect.value = ['h1','h2','h3','h4','p','span','blockquote'].includes(tag) ? tag : 'p';
                }
            } else if (elType === 'button') {
                if (typeTitleEl) typeTitleEl.innerText = 'Nút Bấm & Kêu Gọi Hành Động';
                const btnTextInput = document.getElementById('inspBtnText');
                if (btnTextInput) btnTextInput.value = data.text || '';

                const actionTypeSelect = document.getElementById('inspBtnActionType');
                const targetInput = document.getElementById('inspBtnTargetValue');
                const targetWindowSelect = document.getElementById('inspBtnTargetWindow');

                let actType = 'url';
                const href = attrs['href'] || '';
                if (attrs['type'] === 'submit') {
                    actType = 'submit';
                } else if (href.startsWith('tel:')) {
                    actType = 'tel';
                } else if (href.includes('zalo.me')) {
                    actType = 'zalo';
                } else if (href.startsWith('#')) {
                    actType = 'anchor';
                }

                if (actionTypeSelect) actionTypeSelect.value = actType;
                if (targetInput) targetInput.value = href;
                if (targetWindowSelect) targetWindowSelect.value = attrs['target'] || '_self';
            } else if (elType === 'link') {
                if (typeTitleEl) typeTitleEl.innerText = 'Đường Dẫn & Liên Kết';
                const linkTextInput = document.getElementById('inspLinkText');
                if (linkTextInput) linkTextInput.value = data.text || '';
                if (document.getElementById('inspAttrHref')) document.getElementById('inspAttrHref').value = attrs['href'] || '';
                if (document.getElementById('inspAttrTarget')) document.getElementById('inspAttrTarget').value = attrs['target'] || '';
            } else if (elType === 'image') {
                if (typeTitleEl) typeTitleEl.innerText = 'Hình Ảnh (Media)';
                const imgSrc = attrs['src'] || '';
                if (document.getElementById('inspAttrSrc')) document.getElementById('inspAttrSrc').value = imgSrc;
                if (document.getElementById('inspAttrAlt')) document.getElementById('inspAttrAlt').value = attrs['alt'] || '';
                updateImagePreviewThumbnail(imgSrc);
            } else if (elType === 'input') {
                if (typeTitleEl) typeTitleEl.innerText = 'Ô Nhập Liệu & Form';
                if (document.getElementById('inspInputType')) document.getElementById('inspInputType').value = attrs['type'] || (tag === 'textarea' ? 'textarea' : 'text');
                if (document.getElementById('inspInputName')) document.getElementById('inspInputName').value = attrs['name'] || '';
                if (document.getElementById('inspInputPlaceholder')) document.getElementById('inspInputPlaceholder').value = attrs['placeholder'] || '';
                if (document.getElementById('inspInputRequired')) document.getElementById('inspInputRequired').checked = (attrs['required'] !== undefined && attrs['required'] !== 'false');
            } else if (elType === 'video') {
                if (typeTitleEl) typeTitleEl.innerText = 'Video & Nhúng';
                if (document.getElementById('inspVideoSrc')) document.getElementById('inspVideoSrc').value = attrs['src'] || '';
            } else if (elType === 'container') {
                if (typeTitleEl) typeTitleEl.innerText = 'Khối Khung & Bố Cục';
                const node = astMapper.astMap[nodeId];
                const childIds = (node && node.children) ? node.children : [];
                const childCountEl = document.getElementById('inspContainerChildCount');
                if (childCountEl) childCountEl.innerText = `${childIds.length} thẻ con`;

                const childPillsEl = document.getElementById('inspContainerChildPills');
                if (childPillsEl) {
                    if (childIds.length === 0) {
                        childPillsEl.innerHTML = '<span class="text-[10px] text-slate-500 italic">Không có thẻ con</span>';
                    } else {
                        let pillsHtml = '';
                        childIds.forEach(cId => {
                            const cNode = astMapper.astMap[cId];
                            if (cNode) {
                                pillsHtml += `<button type="button" onclick="selectAstNode('${cId}', true)" class="px-2 py-0.5 rounded bg-slate-900 hover:bg-cyan-600 border border-slate-800 text-[10px] text-slate-300 hover:text-white font-mono transition flex items-center gap-1"><span>&lt;${cNode.tag}&gt;</span><span class="text-slate-500">#${cId}</span></button>`;
                            }
                        });
                        childPillsEl.innerHTML = pillsHtml;
                    }
                }
            }

            // Điền thuộc tính chung (ID, Title)
            if (document.getElementById('inspAttrId')) document.getElementById('inspAttrId').value = attrs['id'] || '';
            if (document.getElementById('inspAttrTitle')) document.getElementById('inspAttrTitle').value = attrs['title'] || '';

            // Cập nhật Tab Classes
            renderInspectorClasses(className);

            // Cập nhật Tab Styles (Bố cục, Kích thước, Màu sắc)
            let style = Object.assign({}, data.style || {});
            if (studioDeviceMode === 'mobile' && typeof astMapper !== 'undefined' && astMapper.getMobileStyles && nodeId) {
                const mobStyles = astMapper.getMobileStyles(nodeId);
                style = Object.assign(style, mobStyles);
            } else if (studioDeviceMode === 'pc' && typeof astMapper !== 'undefined' && astMapper.getPCStyles && nodeId) {
                const pcStyles = astMapper.getPCStyles(nodeId);
                style = Object.assign(style, pcStyles);
            }

            // 1. Bố cục & Flexbox
            if (document.getElementById('inspStyleDisplay')) {
                document.getElementById('inspStyleDisplay').value = style.display || '';
                toggleFlexGroup(style.display || '');
            }
            if (document.getElementById('inspStyleFlexDirection')) document.getElementById('inspStyleFlexDirection').value = style.flexDirection || '';
            if (document.getElementById('inspStyleGap')) document.getElementById('inspStyleGap').value = style.gap || '';
            if (document.getElementById('inspStyleJustifyContent')) document.getElementById('inspStyleJustifyContent').value = style.justifyContent || '';
            if (document.getElementById('inspStyleAlignItems')) document.getElementById('inspStyleAlignItems').value = style.alignItems || '';
            if (document.getElementById('inspStyleOrder')) document.getElementById('inspStyleOrder').value = style.order || '';

            // 2. Khoảng cách (Box Model)
            if (document.getElementById('inspStyleMarginTop')) document.getElementById('inspStyleMarginTop').value = style.marginTop || '';
            if (document.getElementById('inspStyleMarginRight')) document.getElementById('inspStyleMarginRight').value = style.marginRight || '';
            if (document.getElementById('inspStyleMarginBottom')) document.getElementById('inspStyleMarginBottom').value = style.marginBottom || '';
            if (document.getElementById('inspStyleMarginLeft')) document.getElementById('inspStyleMarginLeft').value = style.marginLeft || '';

            if (document.getElementById('inspStylePaddingTop')) document.getElementById('inspStylePaddingTop').value = style.paddingTop || '';
            if (document.getElementById('inspStylePaddingRight')) document.getElementById('inspStylePaddingRight').value = style.paddingRight || '';
            if (document.getElementById('inspStylePaddingBottom')) document.getElementById('inspStylePaddingBottom').value = style.paddingBottom || '';
            if (document.getElementById('inspStylePaddingLeft')) document.getElementById('inspStylePaddingLeft').value = style.paddingLeft || '';

            // 3. Kích thước & Vị trí
            if (document.getElementById('inspStyleWidth')) document.getElementById('inspStyleWidth').value = style.width || '';
            if (document.getElementById('inspStyleHeight')) document.getElementById('inspStyleHeight').value = style.height || '';
            if (document.getElementById('inspStyleMaxWidth')) document.getElementById('inspStyleMaxWidth').value = style.maxWidth || '';
            if (document.getElementById('inspStyleMinWidth')) document.getElementById('inspStyleMinWidth').value = style.minWidth || '';
            if (document.getElementById('inspStylePosition')) document.getElementById('inspStylePosition').value = style.position || '';
            if (document.getElementById('inspStyleTop')) document.getElementById('inspStyleTop').value = style.top || '';
            if (document.getElementById('inspStyleLeft')) document.getElementById('inspStyleLeft').value = style.left || '';
            if (document.getElementById('inspStyleZIndex')) document.getElementById('inspStyleZIndex').value = style.zIndex || '';

            // 4. Typography
            if (document.getElementById('inspStyleFontSize')) document.getElementById('inspStyleFontSize').value = style.fontSize || '';
            if (document.getElementById('inspStyleFontWeight')) document.getElementById('inspStyleFontWeight').value = style.fontWeight || '';
            if (document.getElementById('inspStyleTextAlign')) document.getElementById('inspStyleTextAlign').value = style.textAlign || '';
            if (document.getElementById('inspStyleTextTransform')) document.getElementById('inspStyleTextTransform').value = style.textTransform || '';
            if (document.getElementById('inspStyleLineHeight')) document.getElementById('inspStyleLineHeight').value = style.lineHeight || '';
            if (document.getElementById('inspStyleColor')) document.getElementById('inspStyleColor').value = style.color || '';
            const hexColor = rgbToHex(style.color);
            if (hexColor && document.getElementById('inspStyleColorPicker')) {
                document.getElementById('inspStyleColorPicker').value = hexColor;
            }

            // 5. Nền & Hiệu ứng
            if (document.getElementById('inspStyleBgColor')) document.getElementById('inspStyleBgColor').value = style.backgroundColor || '';
            const hexBg = rgbToHex(style.backgroundColor);
            if (hexBg && document.getElementById('inspStyleBgColorPicker')) {
                document.getElementById('inspStyleBgColorPicker').value = hexBg;
            }
            if (document.getElementById('inspStyleBgImage')) {
                const bgImg = (style.backgroundImage || '').replace(/^url\(["']?/, '').replace(/["']?\)$/, '');
                document.getElementById('inspStyleBgImage').value = bgImg === 'none' ? '' : bgImg;
            }
            const opVal = style.opacity !== undefined && style.opacity !== '' ? Math.round(parseFloat(style.opacity) * 100) : 100;
            if (document.getElementById('inspStyleOpacity')) document.getElementById('inspStyleOpacity').value = opVal;
            if (document.getElementById('inspStyleOpacityVal')) document.getElementById('inspStyleOpacityVal').innerText = opVal + '%';
            if (document.getElementById('inspStyleBoxShadow')) document.getElementById('inspStyleBoxShadow').value = style.boxShadow || '';

            // 6. Viền & Bo góc
            if (document.getElementById('inspStyleBorderRadius')) document.getElementById('inspStyleBorderRadius').value = style.borderRadius || '';
            if (document.getElementById('inspStyleBorderWidth')) document.getElementById('inspStyleBorderWidth').value = style.borderWidth || '';
            if (document.getElementById('inspStyleBorderStyle')) document.getElementById('inspStyleBorderStyle').value = style.borderStyle || '';
            if (document.getElementById('inspStyleBorderColor')) document.getElementById('inspStyleBorderColor').value = style.borderColor || '';
            const hexBorder = rgbToHex(style.borderColor);
            if (hexBorder && document.getElementById('inspStyleBorderColorPicker')) {
                document.getElementById('inspStyleBorderColorPicker').value = hexBorder;
            }

            // Cập nhật tab CSS nếu đang mở
            if (currentInspTab === 'css') {
                renderCssInspectorTab();
            }
        }

        function rgbToHex(rgbStr) {
            if (!rgbStr || rgbStr === 'transparent' || rgbStr === 'rgba(0, 0, 0, 0)') return '';
            if (rgbStr.startsWith('#')) return rgbStr;
            const match = rgbStr.match(/rgba?\((\d+),\s*(\d+),\s*(\d+)/i);
            if (!match) return '';
            const r = parseInt(match[1]).toString(16).padStart(2, '0');
            const g = parseInt(match[2]).toString(16).padStart(2, '0');
            const b = parseInt(match[3]).toString(16).padStart(2, '0');
            return `#${r}${g}${b}`;
        }

        function toggleFlexGroup(displayVal) {
            const flexCtrl = document.getElementById('inspFlexControls');
            if (!flexCtrl) return;
            if (displayVal === 'flex' || displayVal === 'inline-flex') {
                flexCtrl.classList.remove('hidden');
            } else {
                flexCtrl.classList.add('hidden');
            }
        }

        function setDimensionPreset(prop, val) {
            if (!selectedAstNodeId) return;
            if (prop === 'width') {
                const el = document.getElementById('inspStyleWidth');
                if (el) el.value = val;
            } else if (prop === 'height') {
                const el = document.getElementById('inspStyleHeight');
                if (el) el.value = val;
            }
            onInspectorStyleChange(prop, val);
        }

        function setQuickFontSize(size) {
            if (!selectedAstNodeId) return;
            const el = document.getElementById('inspStyleFontSize');
            if (el) el.value = size;
            onInspectorStyleChange('font-size', size);
        }

        function setQuickRadius(val) {
            if (!selectedAstNodeId) return;
            const el = document.getElementById('inspStyleBorderRadius');
            if (el) el.value = val;
            onInspectorStyleChange('border-radius', val);
        }

        function applyCenterMargin() {
            if (!selectedAstNodeId) return;
            const ml = document.getElementById('inspStyleMarginLeft');
            const mr = document.getElementById('inspStyleMarginRight');
            if (ml) ml.value = 'auto';
            if (mr) mr.value = 'auto';
            applyAstStyleChange(selectedAstNodeId, { 'margin-left': 'auto', 'margin-right': 'auto' });
        }

        function onInspectorOpacitySlide(val) {
            if (!selectedAstNodeId) return;
            const pct = parseInt(val) || 0;
            const lbl = document.getElementById('inspStyleOpacityVal');
            if (lbl) lbl.innerText = pct + '%';
            onInspectorStyleChange('opacity', (pct / 100).toFixed(2));
        }

        function moveCurrentNodeUp() {
            if (!selectedAstNodeId) return;
            const node = astMapper.astMap[selectedAstNodeId];
            const updated = astMapper.moveNodeUp(selectedAstNodeId);
            const nextNodeId = (updated && updated.newNodeId) ? updated.newNodeId : (astMapper.lastMovedNodeId || selectedAstNodeId);
            selectedAstNodeId = nextNodeId;
            const updatedHtml = (typeof updated === 'object' && updated.html) ? updated.html : updated;
            syncCodeEditorFromAst(updatedHtml);
            pushStudioHistory(`Di chuyển <${node ? node.tag : 'thẻ'}> lên trước`, updatedHtml, nextNodeId);

            // Di chuyển DOM trực tiếp trong iframe để UI phản hồi tức thì và không bị nháy
            try {
                const iframe = document.getElementById('csPreviewIframe');
                if (iframe && iframe.contentDocument) {
                    const el = iframe.contentDocument.querySelector(`[data-ve-id="${node ? node.id : ''}"]`);
                    if (el && el.previousElementSibling) {
                        el.parentNode.insertBefore(el, el.previousElementSibling);
                    }
                }
            } catch(e) {}

            refreshStudioPreview(true);
            renderAstLayerTree();
            setTimeout(() => {
                if (selectedAstNodeId) selectAstNode(selectedAstNodeId);
            }, 250);
        }

        function moveCurrentNodeDown() {
            if (!selectedAstNodeId) return;
            const node = astMapper.astMap[selectedAstNodeId];
            const updated = astMapper.moveNodeDown(selectedAstNodeId);
            const nextNodeId = (updated && updated.newNodeId) ? updated.newNodeId : (astMapper.lastMovedNodeId || selectedAstNodeId);
            selectedAstNodeId = nextNodeId;
            const updatedHtml = (typeof updated === 'object' && updated.html) ? updated.html : updated;
            syncCodeEditorFromAst(updatedHtml);
            pushStudioHistory(`Di chuyển <${node ? node.tag : 'thẻ'}> xuống sau`, updatedHtml, nextNodeId);

            // Di chuyển DOM trực tiếp trong iframe để UI phản hồi tức thì và không bị nháy
            try {
                const iframe = document.getElementById('csPreviewIframe');
                if (iframe && iframe.contentDocument) {
                    const el = iframe.contentDocument.querySelector(`[data-ve-id="${node ? node.id : ''}"]`);
                    if (el && el.nextElementSibling) {
                        el.parentNode.insertBefore(el.nextElementSibling, el);
                    }
                }
            } catch(e) {}

            refreshStudioPreview(true);
            renderAstLayerTree();
            setTimeout(() => {
                if (selectedAstNodeId) selectAstNode(selectedAstNodeId);
            }, 250);
        }

        function quickAlignNode(align) {
            if (!selectedAstNodeId) return;
            if (align === 'left') {
                applyAstStyleChange(selectedAstNodeId, { 'margin-left': '0px', 'margin-right': 'auto' });
                const ml = document.getElementById('inspStyleMarginLeft');
                const mr = document.getElementById('inspStyleMarginRight');
                if (ml) ml.value = '0px';
                if (mr) mr.value = 'auto';
            } else if (align === 'center') {
                applyCenterMargin();
            } else if (align === 'right') {
                applyAstStyleChange(selectedAstNodeId, { 'margin-left': 'auto', 'margin-right': '0px' });
                const ml = document.getElementById('inspStyleMarginLeft');
                const mr = document.getElementById('inspStyleMarginRight');
                if (ml) ml.value = 'auto';
                if (mr) mr.value = '0px';
            } else if (align === 'full') {
                applyAstStyleChange(selectedAstNodeId, { 'width': '100%', 'max-width': '100%' });
                const w = document.getElementById('inspStyleWidth');
                const mw = document.getElementById('inspStyleMaxWidth');
                if (w) w.value = '100%';
                if (mw) mw.value = '100%';
            }
            setTimeout(() => {
                if (selectedAstNodeId) requestNodeRect(selectedAstNodeId);
            }, 100);
        }

        function renderInspectorClasses(classString) {
            const container = document.getElementById('inspActiveClasses');
            if (!container) return;
            const classes = (classString || '').trim().split(/\s+/).filter(Boolean);
            if (classes.length === 0) {
                container.innerHTML = '<span class="text-[10px] text-slate-500 italic">Chưa có class</span>';
                return;
            }
            let html = '';
            classes.forEach(c => {
                html += `
                    <span class="inline-flex items-center gap-1 bg-slate-800 text-slate-200 border border-slate-700 font-mono text-[10px] px-2 py-0.5 rounded-lg group">
                        <span>${escapeHTML(c)}</span>
                        <button type="button" onclick="removeNodeClass('${escapeHTML(c)}')" class="text-slate-400 hover:text-rose-400 ml-0.5">×</button>
                    </span>
                `;
            });
            container.innerHTML = html;
        }

        // Thao tác sửa Text trực tiếp qua AST Splicer
        function onInspectorTextInput() {
            if (!selectedAstNodeId) return;
            const newText = document.getElementById('inspTextContent').value;
            applyAstTextChange(selectedAstNodeId, newText);
        }

        function applyAstTextChange(nodeId, newText) {
            const updated = astMapper.spliceText(nodeId, newText);
            syncCodeEditorFromAst(updated);
            pushStudioHistory('Sửa chữ trực tiếp', updated, nodeId);

            // Gửi lệnh cập nhật tạm vào iframe preview
            const iframe = document.getElementById('csPreviewIframe');
            if (iframe && iframe.contentWindow) {
                iframe.contentWindow.postMessage({ type: 'APEX_CMD_UPDATE_TEXT_TEMP', nodeId: nodeId, text: newText }, '*');
            }
            requestNodeRect(nodeId);
            renderAstLayerTree();
        }

        let attrChangeDebounceTimer = null;
        function onInspectorAttrChange(attrName, val) {
            if (!selectedAstNodeId) return;
            const updated = astMapper.spliceAttribute(selectedAstNodeId, attrName, val);
            syncCodeEditorFromAst(updated);
            pushStudioHistory(`Sửa thuộc tính ${attrName}`, updated, selectedAstNodeId);

            // Gửi lệnh cập nhật trực tiếp vào DOM iframe preview để thấy thay đổi ngay lập tức (không cần chờ blur hay reload)
            const iframe = document.getElementById('csPreviewIframe');
            if (iframe && iframe.contentWindow) {
                iframe.contentWindow.postMessage({
                    type: 'APEX_CMD_UPDATE_ATTR_TEMP',
                    nodeId: selectedAstNodeId,
                    attr: attrName,
                    val: val
                }, '*');
            }
            requestNodeRect(selectedAstNodeId);

            // Debounce nhẹ để cập nhật lại AST & Layer tree mà không gây giật lag khi gõ nhanh
            clearTimeout(attrChangeDebounceTimer);
            attrChangeDebounceTimer = setTimeout(() => {
                refreshStudioPreview(true);
                renderAstLayerTree();
            }, 350);
        }

        function onInspectorStyleChange(prop, val) {
            if (!selectedAstNodeId) return;
            const styleObj = {};
            styleObj[prop] = val;
            applyAstStyleChange(selectedAstNodeId, styleObj, `Đổi style ${prop}`);
        }

        function onInspectorColorPick(prop, hexVal) {
            if (!selectedAstNodeId) return;
            if (prop === 'color') document.getElementById('inspStyleColor').value = hexVal;
            else if (prop === 'background-color') document.getElementById('inspStyleBgColor').value = hexVal;
            onInspectorStyleChange(prop, hexVal);
        }

        function applyAstStyleChange(nodeId, styleObj, actionTitle = 'Đổi kiểu Style') {
            const isMobile = (studioDeviceMode === 'mobile');
            const res = isMobile
                ? astMapper.setMobileStyles(nodeId, styleObj)
                : astMapper.setPCStyles(nodeId, styleObj);

            const prefix = isMobile ? '[Mobile] ' : '[PC] ';
            syncCodeEditorFromAst(res.html);
            pushStudioHistory(`${prefix}${actionTitle}`, res.html, nodeId);

            const iframe = document.getElementById('csPreviewIframe');
            if (iframe && iframe.contentWindow) {
                iframe.contentWindow.postMessage({ type: 'APEX_CMD_UPDATE_RESPONSIVE_CSS', cssText: res.css }, '*');
                iframe.contentWindow.postMessage({ type: 'APEX_CMD_UPDATE_STYLE_TEMP', nodeId: nodeId, styles: styleObj }, '*');
                // Xóa inline styles tạm thời khỏi DOM iframe ngay lập tức để CSS responsive media query làm chủ 100%
                iframe.contentWindow.postMessage({ type: 'APEX_CMD_CLEAR_TEMP_STYLES', nodeId: nodeId, props: Object.keys(styleObj) }, '*');
            }
            requestNodeRect(nodeId);
        }

        function addCustomClassFromInput() {
            const input = document.getElementById('inspAddClassInput');
            if (!input) return;
            const val = input.value.trim();
            if (!val) return;
            addQuickClass(val);
            input.value = '';
        }

        function addQuickClass(cls) {
            if (!selectedAstNodeId) return;
            const node = astMapper.astMap[selectedAstNodeId];
            if (!node) return;
            const currentClasses = node.classes || [];
            const newClassesToAdd = cls.split(/\s+/).filter(Boolean);
            newClassesToAdd.forEach(c => {
                if (!currentClasses.includes(c)) currentClasses.push(c);
            });
            const classStr = currentClasses.join(' ');
            const updated = astMapper.spliceClasses(selectedAstNodeId, classStr);
            syncCodeEditorFromAst(updated);
            pushStudioHistory(`Thêm class ${cls}`, updated, selectedAstNodeId);
            renderInspectorClasses(classStr);

            const iframe = document.getElementById('csPreviewIframe');
            if (iframe && iframe.contentWindow) {
                iframe.contentWindow.postMessage({ type: 'APEX_CMD_UPDATE_CLASS_TEMP', nodeId: selectedAstNodeId, className: classStr }, '*');
            }
            requestNodeRect(selectedAstNodeId);
            renderAstLayerTree();
        }

        function removeNodeClass(cls) {
            if (!selectedAstNodeId) return;
            const node = astMapper.astMap[selectedAstNodeId];
            if (!node) return;
            const currentClasses = (node.classes || []).filter(c => c !== cls);
            const classStr = currentClasses.join(' ');
            const updated = astMapper.spliceClasses(selectedAstNodeId, classStr);
            syncCodeEditorFromAst(updated);
            pushStudioHistory(`Xóa class ${cls}`, updated, selectedAstNodeId);
            renderInspectorClasses(classStr);

            const iframe = document.getElementById('csPreviewIframe');
            if (iframe && iframe.contentWindow) {
                iframe.contentWindow.postMessage({ type: 'APEX_CMD_UPDATE_CLASS_TEMP', nodeId: selectedAstNodeId, className: classStr }, '*');
            }
            requestNodeRect(selectedAstNodeId);
            renderAstLayerTree();
        }

        // ============================================================
        // CÁC HÀM XỬ LÝ NỘI DUNG THEO LOẠI THẺ (TAB NỘI DUNG NÂNG CAO)
        // ============================================================

        // 1. Chuyển đổi Tag không làm hỏng AST (H1 <-> H2 <-> P...)
        function convertSelectedTag(newTagName) {
            if (!selectedAstNodeId || !newTagName) return;
            const updated = astMapper.spliceTagName(selectedAstNodeId, newTagName);
            syncCodeEditorFromAst(updated);
            pushStudioHistory(`Đổi thẻ thành <${newTagName}>`, updated, selectedAstNodeId);
            refreshStudioPreview();
            setTimeout(() => {
                selectAstNode(selectedAstNodeId, true);
            }, 100);
        }

        // 2. Căn lề, cỡ chữ, định dạng nhanh cho Typography & Container
        function applyQuickTypographyClass(className) {
            if (!selectedAstNodeId || !className) return;

            // Nếu là căn lề
            if (['text-left', 'text-center', 'text-right', 'text-justify'].includes(className)) {
                ['text-left', 'text-center', 'text-right', 'text-justify'].forEach(c => {
                    astMapper.removeClass(selectedAstNodeId, c);
                });
                astMapper.addClass(selectedAstNodeId, className);
            }
            // Nếu là cỡ chữ
            else if (/^text-(xs|sm|base|lg|xl|2xl|3xl|4xl)$/.test(className)) {
                ['text-xs', 'text-sm', 'text-base', 'text-lg', 'text-xl', 'text-2xl', 'text-3xl', 'text-4xl'].forEach(s => {
                    astMapper.removeClass(selectedAstNodeId, s);
                });
                astMapper.addClass(selectedAstNodeId, className);
            }
            // Nếu là định dạng toggle (bold, italic, underline, uppercase)
            else if (['font-bold', 'italic', 'underline', 'uppercase'].includes(className)) {
                const node = astMapper.astMap[selectedAstNodeId];
                const hasCls = (node && node.classes) ? node.classes.includes(className) : false;
                if (hasCls) {
                    astMapper.removeClass(selectedAstNodeId, className);
                } else {
                    astMapper.addClass(selectedAstNodeId, className);
                }
            }
            // Nếu là bo góc nhanh cho ảnh
            else if (['rounded-xl', 'rounded-2xl', 'rounded-full'].includes(className)) {
                ['rounded-xl', 'rounded-2xl', 'rounded-full'].forEach(r => astMapper.removeClass(selectedAstNodeId, r));
                astMapper.addClass(selectedAstNodeId, className);
            }
            // Nếu là nhóm class (như layout flex col...)
            else {
                className.split(/\s+/).filter(Boolean).forEach(c => {
                    astMapper.addClass(selectedAstNodeId, c);
                });
            }

            const updated = astMapper.rawHTML;
            syncCodeEditorFromAst(updated);
            pushStudioHistory(`Áp dụng class ${className}`, updated, selectedAstNodeId);
            refreshStudioPreview();
            const node = astMapper.astMap[selectedAstNodeId];
            renderInspectorClasses((node && node.attrs['class'] ? node.attrs['class'].value : '') || '');
        }

        // 3. Xử lý Nút Bấm (Button CTA)
        function onInspectorButtonTextInput(newText) {
            if (!selectedAstNodeId) return;
            applyAstTextChange(selectedAstNodeId, newText);
        }

        function onInspectorButtonActionTypeChange(actionType) {
            if (!selectedAstNodeId) return;
            const targetLabel = document.getElementById('inspBtnTargetLabel');
            const targetInput = document.getElementById('inspBtnTargetValue');
            const targetWindow = document.getElementById('inspBtnTargetWindowContainer');

            if (actionType === 'submit') {
                onInspectorAttrChange('type', 'submit');
                if (targetLabel) targetLabel.innerText = 'Hành động Form';
                if (targetInput) targetInput.placeholder = 'Gửi toàn bộ dữ liệu form';
                if (targetWindow) targetWindow.classList.add('hidden');
            } else if (actionType === 'anchor') {
                if (targetLabel) targetLabel.innerText = 'Mã phần tử cuộn tới (#id)';
                if (targetInput) {
                    targetInput.placeholder = '#form-dang-ky';
                    if (targetInput.value && !targetInput.value.startsWith('#')) targetInput.value = '#' + targetInput.value;
                }
                if (targetWindow) targetWindow.classList.add('hidden');
                onInspectorAttrChange('href', targetInput ? targetInput.value : '#');
            } else if (actionType === 'tel') {
                if (targetLabel) targetLabel.innerText = 'Số điện thoại gọi (tel:...)';
                if (targetInput) {
                    targetInput.placeholder = 'tel:0900000000';
                    if (targetInput.value && !targetInput.value.startsWith('tel:')) targetInput.value = 'tel:' + targetInput.value;
                }
                if (targetWindow) targetWindow.classList.add('hidden');
                onInspectorAttrChange('href', targetInput ? targetInput.value : 'tel:');
            } else if (actionType === 'zalo') {
                if (targetLabel) targetLabel.innerText = 'Đường dẫn Zalo';
                if (targetInput) {
                    targetInput.placeholder = 'https://zalo.me/0900000000';
                    if (targetInput.value && !targetInput.value.includes('zalo.me')) targetInput.value = 'https://zalo.me/' + targetInput.value.replace(/[^0-9]/g, '');
                }
                if (targetWindow) targetWindow.classList.remove('hidden');
                onInspectorAttrChange('href', targetInput ? targetInput.value : 'https://zalo.me/');
                onInspectorAttrChange('target', '_blank');
            } else { // URL
                if (targetLabel) targetLabel.innerText = 'Địa chỉ URL đích';
                if (targetInput) targetInput.placeholder = 'https://...';
                if (targetWindow) targetWindow.classList.remove('hidden');
                if (targetInput && targetInput.value) onInspectorAttrChange('href', targetInput.value);
            }
        }

        function onInspectorButtonTargetChange(val) {
            if (!selectedAstNodeId) return;
            onInspectorAttrChange('href', val);
        }

        function applyButtonVariant(variant) {
            if (!selectedAstNodeId) return;
            const colorClasses = [
                'bg-cyan-500', 'hover:bg-cyan-400', 'text-slate-950',
                'bg-amber-500', 'hover:bg-amber-400',
                'bg-emerald-500', 'hover:bg-emerald-400',
                'bg-slate-900/80', 'hover:bg-slate-800', 'border-slate-700', 'border', 'backdrop-blur-md'
            ];
            colorClasses.forEach(c => astMapper.removeClass(selectedAstNodeId, c));

            let variantClass = '';
            if (variant === 'cyan') {
                variantClass = 'bg-cyan-500 hover:bg-cyan-400 text-slate-950 font-bold shadow-lg shadow-cyan-500/30';
            } else if (variant === 'gold') {
                variantClass = 'bg-amber-500 hover:bg-amber-400 text-slate-950 font-bold shadow-lg shadow-amber-500/30';
            } else if (variant === 'emerald') {
                variantClass = 'bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-bold shadow-lg shadow-emerald-500/30';
            } else if (variant === 'glass') {
                variantClass = 'bg-slate-900/80 hover:bg-slate-800 border border-slate-700 text-white font-semibold backdrop-blur-md';
            }

            variantClass.split(/\s+/).filter(Boolean).forEach(c => astMapper.addClass(selectedAstNodeId, c));
            const updated = astMapper.rawHTML;
            syncCodeEditorFromAst(updated);
            pushStudioHistory(`Đổi kiểu nút ${variant}`, updated, selectedAstNodeId);
            refreshStudioPreview();
            const node = astMapper.astMap[selectedAstNodeId];
            renderInspectorClasses((node && node.attrs['class'] ? node.attrs['class'].value : '') || '');
        }

        // 4. Xử lý Link
        function onInspectorLinkTextInput(newText) {
            if (!selectedAstNodeId) return;
            applyAstTextChange(selectedAstNodeId, newText);
        }

        // 5. Xử lý Ảnh (Image Preview & Dimensions)
        function updateImagePreviewThumbnail(src) {
            const preview = document.getElementById('inspImgPreview');
            const dimLabel = document.getElementById('inspImgNaturalDim');
            if (!preview) return;
            if (!src) {
                preview.src = '';
                if (dimLabel) dimLabel.innerText = '-- × -- px';
                return;
            }
            preview.src = src;
            preview.onload = function() {
                if (dimLabel) dimLabel.innerText = `${preview.naturalWidth} × ${preview.naturalHeight} px`;
            };
            preview.onerror = function() {
                if (dimLabel) dimLabel.innerText = 'Lỗi tải ảnh';
            };
        }

        // 6. Xử lý Form Input & Video
        function onInspectorRequiredToggle(isChecked) {
            if (!selectedAstNodeId) return;
            if (isChecked) {
                onInspectorAttrChange('required', '');
            } else {
                const updated = astMapper.removeAttribute(selectedAstNodeId, 'required');
                syncCodeEditorFromAst(updated);
                pushStudioHistory('Bỏ thuộc tính required', updated, selectedAstNodeId);
                refreshStudioPreview();
            }
        }

        function onInspectorVideoSrcChange(src) {
            if (!selectedAstNodeId) return;
            onInspectorAttrChange('src', src);
        }

        // ============================================================
        // TAB CSS: TOÀN BỘ CSS ĐANG ÁP DỤNG & THƯ VIỆN HIỆU ỨNG (18+ PRESETS)
        // ============================================================

        var cssEffectCurrentCategory = 'all';
        var cssEffectSearchKeyword = '';

        var CSS_EFFECT_PRESETS = [
            // Animation
            {
                id: 'anim-pulse',
                name: 'Nhịp Đập Pulse',
                category: 'animation',
                description: 'Hiệu ứng nhịp đập phóng to nhẹ thu hút ánh nhìn',
                styles: { 'animation': 'pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite' }
            },
            {
                id: 'anim-bounce-soft',
                name: 'Nhún Nhảy Bounce',
                category: 'animation',
                description: 'Nhún nhảy nhẹ nhàng cho nút CTA hoặc huy hiệu',
                styles: { 'animation': 'bounce 1.5s infinite' }
            },
            {
                id: 'anim-spin-slow',
                name: 'Xoay Tròn 360°',
                category: 'animation',
                description: 'Xoay tròn liên tục vô tận 8 giây',
                styles: { 'animation': 'spin 8s linear infinite' }
            },
            // Glow & Neon
            {
                id: 'glow-neon-cyan',
                name: 'Neon Cyan Công Nghệ',
                category: 'glow',
                description: 'Phát sáng xanh Neon viền và bóng đổ hiện đại',
                styles: {
                    'box-shadow': '0 0 20px rgba(6, 182, 212, 0.55), inset 0 0 10px rgba(6, 182, 212, 0.25)',
                    'border': '1px solid #06b6d4'
                }
            },
            {
                id: 'glow-gold-luxury',
                name: 'Gold Hoàng Kim Sang Trọng',
                category: 'glow',
                description: 'Hào quang vàng gold tài chính cao cấp',
                styles: {
                    'box-shadow': '0 0 25px rgba(245, 158, 11, 0.6), inset 0 0 10px rgba(245, 158, 11, 0.25)',
                    'border': '1px solid #f59e0b'
                }
            },
            {
                id: 'glow-emerald-active',
                name: 'Emerald Xanh Thắng Lợi',
                category: 'glow',
                description: 'Phát sáng ngọc bích biểu thị thành công và bảo mật',
                styles: {
                    'box-shadow': '0 0 20px rgba(16, 185, 129, 0.55), inset 0 0 10px rgba(16, 185, 129, 0.25)',
                    'border': '1px solid #10b981'
                }
            },
            {
                id: 'glow-text-cyan',
                name: 'Chữ Phát Sáng Cyan',
                category: 'glow',
                description: 'Đổ bóng chữ neon cyan nổi bật trên nền tối',
                styles: {
                    'text-shadow': '0 0 12px rgba(6, 182, 212, 0.8), 0 0 24px rgba(6, 182, 212, 0.4)'
                }
            },
            // Glassmorphism
            {
                id: 'glass-dark-card',
                name: 'Kính Mờ Tối (Dark Glass)',
                category: 'glass',
                description: 'Kính mờ xuyên thấu blur 16px viền trắng tinh tế',
                styles: {
                    'background': 'rgba(15, 23, 42, 0.75)',
                    'backdrop-filter': 'blur(16px)',
                    '-webkit-backdrop-filter': 'blur(16px)',
                    'border': '1px solid rgba(255, 255, 255, 0.12)'
                }
            },
            {
                id: 'glass-frosted-ultra',
                name: 'Kính Băng (Frosted Ice)',
                category: 'glass',
                description: 'Độ mờ cao blur 24px sang trọng kiểu iOS / macOS',
                styles: {
                    'background': 'rgba(255, 255, 255, 0.06)',
                    'backdrop-filter': 'blur(24px)',
                    '-webkit-backdrop-filter': 'blur(24px)',
                    'border': '1px solid rgba(255, 255, 255, 0.2)'
                }
            },
            {
                id: 'glass-tint-cyan',
                name: 'Kính Ánh Xanh Cyan',
                category: 'glass',
                description: 'Nền kính phớt xanh cyan công nghệ nhẹ nhàng',
                styles: {
                    'background': 'rgba(6, 182, 212, 0.08)',
                    'backdrop-filter': 'blur(14px)',
                    '-webkit-backdrop-filter': 'blur(14px)',
                    'border': '1px solid rgba(6, 182, 212, 0.3)'
                }
            },
            // Gradient
            {
                id: 'gradient-cyan-to-blue',
                name: 'Dải Màu Cyan Đậm Đà',
                category: 'gradient',
                description: 'Chuyển màu mượt mà từ cyan 500 sang blue 600',
                styles: {
                    'background': 'linear-gradient(135deg, #06b6d4 0%, #2563eb 100%)',
                    'color': '#ffffff'
                }
            },
            {
                id: 'gradient-gold-sunset',
                name: 'Hoàng Kim Gradient',
                category: 'gradient',
                description: 'Chuyển màu vàng cam rực rỡ thu hút khách tài chính',
                styles: {
                    'background': 'linear-gradient(135deg, #f59e0b 0%, #ea580c 100%)',
                    'color': '#ffffff'
                }
            },
            {
                id: 'gradient-text-clip',
                name: 'Chữ Cầu Vồng (Gradient Text)',
                category: 'gradient',
                description: 'Dải màu phủ trực tiếp lên chữ xuyên thấu',
                styles: {
                    'background': 'linear-gradient(90deg, #38bdf8, #818cf8, #c084fc)',
                    '-webkit-background-clip': 'text',
                    '-webkit-text-fill-color': 'transparent'
                }
            },
            // Shadow 3D
            {
                id: 'shadow-elevation-xl',
                name: 'Đổ Bóng Nổi 3D Sâu',
                category: 'shadow',
                description: 'Nâng phần tử lên cao khỏi bề mặt trang web',
                styles: {
                    'box-shadow': '0 25px 50px -12px rgba(0, 0, 0, 0.65)'
                }
            },
            {
                id: 'shadow-cyan-glow',
                name: 'Bóng Đổ Xanh Cyan',
                category: 'shadow',
                description: 'Bóng đổ mềm mại có màu xanh tạo chiều sâu',
                styles: {
                    'box-shadow': '0 12px 32px -4px rgba(6, 182, 212, 0.4)'
                }
            },
            {
                id: 'shadow-inset-carve',
                name: 'Khắc Chìm (Inner Shadow)',
                category: 'shadow',
                description: 'Đổ bóng ngược vào bên trong tạo cảm giác lõm sâu',
                styles: {
                    'box-shadow': 'inset 0 3px 10px 0 rgba(0, 0, 0, 0.7)'
                }
            },
            // Hover
            {
                id: 'hover-lift-soft',
                name: 'Bay Lên Khi Hover (Lift)',
                category: 'hover',
                description: 'Nổi lên 4px mượt mà kèm bóng đổ sâu',
                styles: {
                    'transition': 'all 0.25s cubic-bezier(0.4, 0, 0.2, 1)',
                    'transform': 'translateY(-4px)',
                    'box-shadow': '0 15px 30px -5px rgba(0, 0, 0, 0.4)'
                }
            },
            {
                id: 'hover-scale-zoom',
                name: 'Phóng To Nhẹ (Scale Zoom)',
                category: 'hover',
                description: 'Phóng to 3% với transition êm dịu',
                styles: {
                    'transition': 'transform 0.25s ease-in-out',
                    'transform': 'scale(1.03)'
                }
            }
        ];

        function setCssEffectFilter(cat) {
            cssEffectCurrentCategory = cat || 'all';
            const categories = ['all', 'animation', 'glow', 'glass', 'gradient', 'shadow', 'hover'];
            categories.forEach(c => {
                const btn = document.getElementById('btnEffCat_' + c);
                if (btn) {
                    if (c === cssEffectCurrentCategory) {
                        btn.className = 'px-2 py-0.5 rounded-full bg-cyan-600 text-white font-bold transition';
                    } else {
                        btn.className = 'px-2 py-0.5 rounded-full bg-slate-850 hover:bg-slate-750 text-slate-300 transition';
                    }
                }
            });
            renderCssPresetsGrid();
        }

        function filterCssEffects(keyword) {
            cssEffectSearchKeyword = (keyword || '').toLowerCase().trim();
            renderCssPresetsGrid();
        }

        function renderCssPresetsGrid() {
            const grid = document.getElementById('cssEffectPresetsGrid');
            if (!grid) return;

            const filtered = CSS_EFFECT_PRESETS.filter(p => {
                const matchCat = (cssEffectCurrentCategory === 'all' || p.category === cssEffectCurrentCategory);
                const matchSearch = (!cssEffectSearchKeyword ||
                    p.name.toLowerCase().includes(cssEffectSearchKeyword) ||
                    p.description.toLowerCase().includes(cssEffectSearchKeyword) ||
                    p.category.toLowerCase().includes(cssEffectSearchKeyword));
                return matchCat && matchSearch;
            });

            if (filtered.length === 0) {
                grid.innerHTML = '<div class="col-span-2 text-center text-slate-500 text-[11px] py-4 italic">Không tìm thấy hiệu ứng phù hợp</div>';
                return;
            }

            let html = '';
            filtered.forEach(p => {
                html += `
                    <div class="bg-slate-900 border border-slate-800 rounded-lg p-2 flex flex-col justify-between hover:border-slate-700 transition group">
                        <div class="mb-1.5">
                            <div class="text-[11px] font-bold text-white group-hover:text-cyan-300 transition truncate">${p.name}</div>
                            <div class="text-[9px] text-slate-400 line-clamp-2 leading-tight">${p.description}</div>
                        </div>
                        <button type="button" onclick="applyCssEffectPreset('${p.id}')" class="w-full bg-slate-850 hover:bg-cyan-600 text-cyan-400 hover:text-white border border-cyan-800/50 hover:border-transparent text-[10px] font-bold py-1 rounded transition flex items-center justify-center gap-1 shadow-sm">
                            <i class="fa-solid fa-wand-magic-sparkles text-[9px]"></i>
                            <span>Áp dụng</span>
                        </button>
                    </div>
                `;
            });
            grid.innerHTML = html;
        }

        function applyCssEffectPreset(presetId) {
            if (!selectedAstNodeId) return;
            const preset = CSS_EFFECT_PRESETS.find(p => p.id === presetId);
            if (!preset) return;
            applyAstStyleChange(selectedAstNodeId, preset.styles, 'Áp dụng hiệu ứng ' + preset.name);
            renderCssInspectorTab();
        }

        function renderCssInspectorTab() {
            renderCssPresetsGrid();
            renderActiveCssProperties();
        }

        // Trích xuất toàn bộ CSS thực tế đang áp dụng trên thẻ (Từ Responsive AST, Inline Style, StyleSheet Rules & Live Computed Styles)
        function extractAppliedCssForNode(nodeId) {
            const activeStyles = {};
            if (!nodeId) return activeStyles;

            // 1. Responsive Styles (AST Mapper PC / Mobile)
            if (typeof studioDeviceMode !== 'undefined' && studioDeviceMode === 'mobile' && typeof astMapper !== 'undefined' && astMapper.getMobileStyles) {
                Object.assign(activeStyles, astMapper.getMobileStyles(nodeId) || {});
            } else if (typeof astMapper !== 'undefined' && astMapper.getPCStyles) {
                Object.assign(activeStyles, astMapper.getPCStyles(nodeId) || {});
            }

            // 2. Inline Style attribute trên thẻ
            const node = (typeof astMapper !== 'undefined' && astMapper.astMap) ? astMapper.astMap[nodeId] : null;
            if (node && node.attrs && node.attrs['style']) {
                const inlineStr = node.attrs['style'].value || '';
                inlineStr.split(';').forEach(pair => {
                    const idx = pair.indexOf(':');
                    if (idx > -1) {
                        const prop = pair.substring(0, idx).trim().toLowerCase();
                        const val = pair.substring(idx + 1).trim();
                        if (prop && val && !activeStyles[prop]) {
                            activeStyles[prop] = val;
                        }
                    }
                });
            }

            // 3. Truy xuất phần tử DOM thực trong preview Iframe
            const iframe = document.getElementById('csPreviewIframe');
            let el = null;
            let computed = null;
            try {
                if (iframe && iframe.contentDocument) {
                    el = iframe.contentDocument.querySelector('[data-ve-id="' + nodeId + '"]');
                    if (el && iframe.contentWindow) {
                        computed = iframe.contentWindow.getComputedStyle(el);
                    }
                }
            } catch(e) {}

            // A. Quét rules từ styleSheets (Tailwind, <style>, stylesheets)
            if (el && iframe && iframe.contentDocument) {
                try {
                    const sheets = iframe.contentDocument.styleSheets;
                    for (let s = 0; s < sheets.length; s++) {
                        try {
                            const rules = sheets[s].cssRules || sheets[s].rules;
                            if (!rules) continue;
                            for (let r = 0; r < rules.length; r++) {
                                const rule = rules[r];
                                if (rule.selectorText) {
                                    let isMatch = false;
                                    try {
                                        isMatch = el.matches(rule.selectorText);
                                    } catch(eSel) {}

                                    if (isMatch) {
                                        const st = rule.style;
                                        if (st && st.length) {
                                            for (let p = 0; p < st.length; p++) {
                                                const propName = st[p];
                                                if (!propName || propName.startsWith('--tw-')) continue;
                                                if (!activeStyles[propName]) {
                                                    let v = st.getPropertyValue(propName);
                                                    if (v && v.includes('var(') && computed) {
                                                        const cv = computed.getPropertyValue(propName);
                                                        if (cv) v = cv;
                                                    }
                                                    if (v) activeStyles[propName] = v.trim();
                                                }
                                            }
                                        }
                                    }
                                }
                            }
                        } catch(eSheet) {}
                    }
                } catch(e) {}
            }

            // B. Quét và bổ sung các thuộc tính thị giác trực quan thực tế từ getComputedStyle
            if (computed) {
                const visualProps = [
                    'color', 'background-color', 'background-image',
                    'font-size', 'font-weight', 'font-family', 'line-height', 'letter-spacing', 'text-align',
                    'display', 'flex-direction', 'justify-content', 'align-items', 'gap',
                    'width', 'height', 'max-width', 'min-width',
                    'padding-top', 'padding-right', 'padding-bottom', 'padding-left',
                    'margin-top', 'margin-right', 'margin-bottom', 'margin-left',
                    'border-width', 'border-style', 'border-color', 'border-radius',
                    'box-shadow', 'opacity', 'position', 'top', 'right', 'bottom', 'left', 'z-index',
                    'overflow', 'cursor', 'transform', 'transition'
                ];

                visualProps.forEach(prop => {
                    if (activeStyles[prop]) return;
                    const val = computed.getPropertyValue(prop);
                    if (!val) return;

                    const isDefault = (
                        (val === 'none' && ['box-shadow', 'background-image', 'transform', 'transition'].includes(prop)) ||
                        (val === 'normal' && ['letter-spacing'].includes(prop)) ||
                        (val === 'auto' && ['z-index', 'top', 'right', 'bottom', 'left', 'cursor'].includes(prop)) ||
                        (val === 'static' && prop === 'position') ||
                        (val === 'visible' && prop === 'overflow') ||
                        (val === '1' && prop === 'opacity') ||
                        ((val === 'rgba(0, 0, 0, 0)' || val === 'transparent') && prop === 'background-color') ||
                        ((val === '0px' || val === '0') && (prop.startsWith('border') || prop.startsWith('margin') || prop.startsWith('padding') || prop === 'gap')) ||
                        (val === 'none' && prop === 'border-style')
                    );

                    const coreProps = ['color', 'font-size', 'font-weight', 'display'];
                    if (coreProps.includes(prop) || !isDefault) {
                        activeStyles[prop] = val.trim();
                    }
                });
            }

            return activeStyles;
        }

        function renderActiveCssProperties() {
            const listEl = document.getElementById('inspActiveCssList');
            const countEl = document.getElementById('cssActivePropsCount');
            if (!listEl) return;

            if (!selectedAstNodeId) {
                listEl.innerHTML = '<div class="text-[10px] text-slate-500 italic py-2 text-center">Chọn một phần tử để xem toàn bộ CSS</div>';
                if (countEl) countEl.innerText = '0 thuộc tính';
                return;
            }

            const activeStyles = extractAppliedCssForNode(selectedAstNodeId);
            const propKeys = Object.keys(activeStyles);
            if (countEl) countEl.innerText = `${propKeys.length} thuộc tính`;

            if (propKeys.length === 0) {
                listEl.innerHTML = '<div class="text-[10px] text-slate-500 italic py-2 text-center">Chưa có thuộc tính CSS riêng biệt</div>';
                return;
            }

            let customProps = {};
            if (typeof studioDeviceMode !== 'undefined' && studioDeviceMode === 'mobile' && typeof astMapper !== 'undefined' && astMapper.getMobileStyles) {
                customProps = astMapper.getMobileStyles(selectedAstNodeId) || {};
            } else if (typeof astMapper !== 'undefined' && astMapper.getPCStyles) {
                customProps = astMapper.getPCStyles(selectedAstNodeId) || {};
            }

            let html = '';
            propKeys.forEach(prop => {
                const val = activeStyles[prop] || '';
                const isCustom = (customProps[prop] !== undefined && customProps[prop] !== null);
                const isLongVal = (val.length > 28 || val.includes(',') || val.includes('\n') || val.includes('('));
                html += `
                    <div class="flex flex-col gap-1.5 bg-slate-900 border ${isCustom ? 'border-cyan-500/50 bg-slate-900/90' : 'border-slate-800'} rounded-lg p-2 hover:border-slate-700 transition">
                        <div class="flex items-center justify-between gap-1.5">
                            <span class="text-[11px] font-mono text-cyan-400 font-semibold break-all leading-snug select-text" title="${prop}">${escapeHTML(prop)}</span>
                            <div class="flex items-center gap-1.5 flex-shrink-0">
                                ${isCustom ? '<span class="text-[8px] font-mono px-1 py-0.5 rounded bg-amber-500/20 text-amber-300 border border-amber-500/30" title="Tùy biến riêng cho thiết bị này">Tùy biến</span>' : ''}
                                <button type="button" onclick="removeCssProperty('${prop}')" class="text-slate-500 hover:text-rose-400 p-0.5 rounded transition" title="Xóa thuộc tính CSS">
                                    <i class="fa-solid fa-trash-can text-[10px]"></i>
                                </button>
                            </div>
                        </div>
                        <div class="w-full">
                            ${isLongVal ? `
                                <textarea rows="${Math.min(4, Math.max(2, Math.ceil(val.length / 32)))}" onchange="onCssPropertyEdit('${prop}', this.value)" class="w-full bg-slate-950 border border-slate-800 rounded px-2 py-1 text-[11px] text-white font-mono focus:outline-none focus:border-cyan-500 whitespace-pre-wrap break-all resize-y leading-relaxed">${escapeHTML(val)}</textarea>
                            ` : `
                                <input type="text" value="${escapeHTML(val)}" onchange="onCssPropertyEdit('${prop}', this.value)" class="w-full bg-slate-950 border border-slate-800 rounded px-2 py-1 text-[11px] text-white font-mono focus:outline-none focus:border-cyan-500 break-all">
                            `}
                        </div>
                    </div>
                `;
            });
            listEl.innerHTML = html;
        }

        function onCssPropertyEdit(prop, newVal) {
            if (!selectedAstNodeId || !prop) return;
            onInspectorStyleChange(prop, newVal);
        }

        function removeCssProperty(prop) {
            if (!selectedAstNodeId || !prop) return;
            // 1. Xóa khỏi AST responsive styles
            const styleObj = {};
            styleObj[prop] = null;
            applyAstStyleChange(selectedAstNodeId, styleObj, `Xóa CSS ${prop}`);

            // 2. Xóa khỏi inline style nếu có
            const node = (typeof astMapper !== 'undefined' && astMapper.astMap) ? astMapper.astMap[selectedAstNodeId] : null;
            if (node && node.attrs && node.attrs['style']) {
                const sVal = node.attrs['style'].value || '';
                const reg = new RegExp('(^|;)\\s*' + prop.replace(/[-[\]{}()*+?.,\\^$|#\s]/g, '\\$&') + '\\s*:[^;]*(;|$)', 'ig');
                const newStyle = sVal.replace(reg, '$1').replace(/^;\s*/, '').replace(/;\s*;+/g, ';').trim();
                const updated = astMapper.spliceAttribute(selectedAstNodeId, 'style', newStyle);
                syncCodeEditorFromAst(updated);
            }

            renderActiveCssProperties();
        }

        function addCustomCssPropertyFromInput() {
            if (!selectedAstNodeId) return;
            const nameInput = document.getElementById('cssCustomPropName');
            const valInput = document.getElementById('cssCustomPropValue');
            if (!nameInput || !valInput) return;

            const prop = nameInput.value.trim().toLowerCase();
            const val = valInput.value.trim();
            if (!prop || !val) return;

            onInspectorStyleChange(prop, val);
            nameInput.value = '';
            valInput.value = '';
            renderActiveCssProperties();
        }

        function applyRawCssString() {
            if (!selectedAstNodeId) return;
            const rawEl = document.getElementById('cssRawTextarea');
            if (!rawEl) return;
            const rawStr = rawEl.value.trim();
            if (!rawStr) return;

            const parsed = {};
            rawStr.split(';').forEach(statement => {
                const idx = statement.indexOf(':');
                if (idx > -1) {
                    const key = statement.substring(0, idx).trim().toLowerCase();
                    const val = statement.substring(idx + 1).trim();
                    if (key && val) {
                        parsed[key] = val;
                    }
                }
            });

            if (Object.keys(parsed).length > 0) {
                applyAstStyleChange(selectedAstNodeId, parsed, 'Áp dụng mã CSS tùy chỉnh');
                rawEl.value = '';
                renderActiveCssProperties();
            }
        }


