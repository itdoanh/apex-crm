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
                    const absX = rect.left + msg.clientX;
                    const absY = rect.top + msg.clientY;
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
                if (isDraggingGizmo || isMovingGizmo || !studioFreezeMode) return;
                const hoverBox = document.getElementById('csHoverBox');
                const hoverTag = document.getElementById('csHoverTag');
                if (hoverBox && hoverTag && msg.rect) {
                    hoverBox.style.top = msg.rect.top + 'px';
                    hoverBox.style.left = msg.rect.left + 'px';
                    hoverBox.style.width = msg.rect.width + 'px';
                    hoverBox.style.height = msg.rect.height + 'px';
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
                handleNodeSelection(msg.data);
            }

            if (msg.type === 'APEX_BUILDER_NODE_DBLCLICKED') {
                handleNodeSelection(msg.data);
                startInlineTextEdit();
            }

            if (msg.type === 'APEX_BUILDER_VIEWPORT_CHANGED') {
                if (selectedAstNodeId) requestNodeRect(selectedAstNodeId);
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
            if (!astMapper || !astMapper.astMap) return;
            let cur = astMapper.astMap[nodeId];
            while (cur && cur.parentId) {
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

            // Cập nhật bảng thuộc tính Visual Inspector
            populateInspectorWithNode(data);

            // Tự động chọn đúng tab Layer và highlight node trên cây DOM (Đồng bộ 2 chiều Canvas <-> Layer Tree)
            expandAncestorsInLayerTree(data.nodeId);
            switchInspectorTab('layers');
            renderAstLayerTree();
            setTimeout(() => {
                const row = document.getElementById('layer-row-' + data.nodeId);
                if (row) {
                    row.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
                }
            }, 50);

            // Mở tự động Inspector nếu đang ở Visual Builder
            if (studioViewMode === 'visual' && !studioInspectorOpen) {
                toggleInspectorPanel();
            }

            // Highlight dòng code tương ứng trong Code Editor
            const node = astMapper.astMap[data.nodeId];
            if (node && node.startOffset !== undefined) {
                highlightCodeAtOffset(node.startOffset);
            }
        }

        function repositionGizmoBox(rect) {
            const gizmoBox = document.getElementById('csGizmoBox');
            const gizmoTag = document.getElementById('csGizmoTag');
            const gizmoSize = document.getElementById('csGizmoSize');
            const btnImg = document.getElementById('btnGizmoChangeImg');
            const btnLink = document.getElementById('btnGizmoChangeLink');

            if (!gizmoBox || !rect) return;

            gizmoBox.style.top = Math.max(0, rect.top) + 'px';
            gizmoBox.style.left = Math.max(0, rect.left) + 'px';
            gizmoBox.style.width = Math.max(20, rect.width) + 'px';
            gizmoBox.style.height = Math.max(12, rect.height) + 'px';

            const tag = selectedNodeMeta ? selectedNodeMeta.tagName : 'div';
            const classSummary = selectedNodeMeta && selectedNodeMeta.className ? '.' + selectedNodeMeta.className.trim().split(/\s+/).slice(0, 2).join('.') : '';
            const isLocked = studioLockedNodes.has(selectedAstNodeId);
            if (gizmoTag) {
                gizmoTag.innerHTML = `${isLocked ? '<i class="fa-solid fa-lock text-yellow-400 mr-1"></i>' : ''}${tag.toUpperCase()} ${classSummary}`;
            }
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

        // Chuyển đổi Tab trong Visual Inspector
        function switchInspectorTab(tab) {
            currentInspTab = tab;
            window.currentInspTab = tab;
            window.currentInspectorTab = tab;
            const tabs = ['content', 'classes', 'styles', 'layers'];
            tabs.forEach(t => {
                const btn = document.getElementById('btnInspTab' + t.charAt(0).toUpperCase() + t.slice(1));
                const sec = document.getElementById('inspSection' + t.charAt(0).toUpperCase() + t.slice(1));
                if (btn) {
                    if (t === tab) {
                        btn.className = 'flex-1 py-2 font-bold text-center border-b-2 border-cyan-400 text-white transition flex items-center justify-center gap-1';
                    } else {
                        btn.className = 'flex-1 py-2 font-semibold text-center text-slate-400 hover:text-white transition flex items-center justify-center gap-1';
                    }
                }
                if (sec) sec.classList.toggle('hidden', t !== tab);
            });

            if (tab === 'layers') {
                renderAstLayerTree();
            }
        }
        const openVisualInspectorTab = switchInspectorTab;

        // Điền dữ liệu node vào Visual Inspector
        function populateInspectorWithNode(data) {
            document.getElementById('inspTagBadge').innerText = data.tagName || 'element';
            document.getElementById('inspIdBadge').innerText = '#' + (data.nodeId || '');

            // Breadcrumb navigation
            const breadcrumbEl = document.getElementById('inspBreadcrumb');
            if (breadcrumbEl && data.nodeId) {
                const path = astMapper.getNodePath(data.nodeId);
                let bHtml = '';
                path.forEach((p, idx) => {
                    const isLast = idx === path.length - 1;
                    const tagCls = isLast ? 'text-cyan-400 font-bold' : 'text-slate-400 hover:text-white cursor-pointer';
                    bHtml += `<span class="${tagCls}" onclick="selectAstNode('${p.id}')">${p.tag}</span>`;
                    if (!isLast) bHtml += '<span class="text-slate-600">/</span>';
                });
                breadcrumbEl.innerHTML = bHtml || '<span>body</span>';
            }

            // Tab Content
            const textInput = document.getElementById('inspTextContent');
            if (textInput) textInput.value = data.text || '';

            // Attributes
            const attrs = data.attributes || {};
            const linkGroup = document.getElementById('inspAttrLinkGroup');
            const imgGroup = document.getElementById('inspAttrImgGroup');
            if (linkGroup) linkGroup.classList.toggle('hidden', data.tagName !== 'a');
            if (imgGroup) imgGroup.classList.toggle('hidden', data.tagName !== 'img');

            if (document.getElementById('inspAttrHref')) document.getElementById('inspAttrHref').value = attrs['href'] || '';
            if (document.getElementById('inspAttrTarget')) document.getElementById('inspAttrTarget').value = attrs['target'] || '';
            if (document.getElementById('inspAttrSrc')) document.getElementById('inspAttrSrc').value = attrs['src'] || '';
            if (document.getElementById('inspAttrAlt')) document.getElementById('inspAttrAlt').value = attrs['alt'] || '';
            if (document.getElementById('inspAttrId')) document.getElementById('inspAttrId').value = attrs['id'] || '';
            if (document.getElementById('inspAttrTitle')) document.getElementById('inspAttrTitle').value = attrs['title'] || '';

            // Tab Classes
            renderInspectorClasses(data.className);

            // Tab Styles (6 Phân Hệ Chi Tiết - Hỗ Trợ Scope Riêng Mobile vs PC)
            let style = Object.assign({}, data.style || {});
            if (studioDeviceMode === 'mobile' && typeof astMapper !== 'undefined' && astMapper.getMobileStyles && data.nodeId) {
                const mobStyles = astMapper.getMobileStyles(data.nodeId);
                style = Object.assign(style, mobStyles);
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
            syncCodeEditorFromAst(updated);
            pushStudioHistory(`Di chuyển <${node ? node.tag : 'thẻ'}> lên trước`, updated, selectedAstNodeId);
            refreshStudioPreview();
            renderAstLayerTree();
            setTimeout(() => {
                if (selectedAstNodeId) selectAstNode(selectedAstNodeId);
            }, 300);
        }

        function moveCurrentNodeDown() {
            if (!selectedAstNodeId) return;
            const node = astMapper.astMap[selectedAstNodeId];
            const updated = astMapper.moveNodeDown(selectedAstNodeId);
            syncCodeEditorFromAst(updated);
            pushStudioHistory(`Di chuyển <${node ? node.tag : 'thẻ'}> xuống sau`, updated, selectedAstNodeId);
            refreshStudioPreview();
            renderAstLayerTree();
            setTimeout(() => {
                if (selectedAstNodeId) selectAstNode(selectedAstNodeId);
            }, 300);
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

        function onInspectorAttrChange(attrName, val) {
            if (!selectedAstNodeId) return;
            const updated = astMapper.spliceAttribute(selectedAstNodeId, attrName, val);
            syncCodeEditorFromAst(updated);
            pushStudioHistory(`Sửa thuộc tính ${attrName}`, updated, selectedAstNodeId);
            refreshStudioPreview();
            renderAstLayerTree();
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
            if (studioDeviceMode === 'mobile') {
                const res = astMapper.setMobileStyles(nodeId, styleObj);
                syncCodeEditorFromAst(res.html);
                pushStudioHistory(`[Mobile] ${actionTitle}`, res.html, nodeId);

                const iframe = document.getElementById('csPreviewIframe');
                if (iframe && iframe.contentWindow) {
                    iframe.contentWindow.postMessage({ type: 'APEX_CMD_UPDATE_RESPONSIVE_CSS', cssText: res.css }, '*');
                    iframe.contentWindow.postMessage({ type: 'APEX_CMD_UPDATE_STYLE_TEMP', nodeId: nodeId, styles: styleObj }, '*');
                }
                requestNodeRect(nodeId);
                return;
            }

            const updated = astMapper.spliceStyle(nodeId, styleObj);
            syncCodeEditorFromAst(updated);
            pushStudioHistory(actionTitle, updated, nodeId);

            const iframe = document.getElementById('csPreviewIframe');
            if (iframe && iframe.contentWindow) {
                iframe.contentWindow.postMessage({ type: 'APEX_CMD_UPDATE_STYLE_TEMP', nodeId: nodeId, styles: styleObj }, '*');
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

