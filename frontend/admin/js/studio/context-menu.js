/* ==============================================================
   APEX VISUAL WEB BUILDER - 20-ACTION CONTEXT MENU (RIGHT CLICK)
   Cross-Boundary Iframe Event Propagation & Viewport Clamping
   ============================================================== */

        // ============================================================
        // 2. CONTEXT MENU CHUỘT PHẢI CHUYÊN NGHIỆP
        // ============================================================
        function openContextMenu(clientX, clientY, nodeId) {
            if (!nodeId) return;
            selectAstNode(nodeId);

            const menu = document.getElementById('csContextMenu');
            if (!menu) return;

            const node = astMapper.astMap[nodeId];
            if (!node) return;

            // 1. Cập nhật thông tin Header
            const tagIcon = document.getElementById('ctxMenuTagIcon');
            const tagBadge = document.getElementById('ctxMenuTagBadge');
            const idBadge = document.getElementById('ctxMenuIdBadge');
            const typeBadge = document.getElementById('ctxMenuTypeBadge');

            const info = getNodeCategory(node);
            if (tagIcon) tagIcon.innerHTML = `<i class="${info.icon}"></i>`;
            if (tagBadge) tagBadge.innerText = node.tag.toUpperCase();
            if (idBadge) idBadge.innerText = node.attrs['id'] ? `#${node.attrs['id'].value}` : (node.attrs['class'] ? `.${node.attrs['class'].value.trim().split(/\s+/)[0]}` : `#${node.id}`);
            if (typeBadge) typeBadge.innerText = info.typeLabel;

            // 2. Hiện / Ẩn đổi ảnh hoặc đổi link
            const itemChangeImg = document.getElementById('ctxItemChangeImg');
            const itemChangeLink = document.getElementById('ctxItemChangeLink');
            if (itemChangeImg) {
                if (node.tag === 'img') itemChangeImg.classList.remove('hidden');
                else itemChangeImg.classList.add('hidden');
            }
            if (itemChangeLink) {
                if (node.tag === 'a') itemChangeLink.classList.remove('hidden');
                else itemChangeLink.classList.add('hidden');
            }

            // 3. Trạng thái Paste & Paste Style
            const itemPaste = document.getElementById('ctxItemPaste');
            const itemPasteStyle = document.getElementById('ctxItemPasteStyle');
            if (itemPaste) {
                if (studioClipboardNode) {
                    itemPaste.classList.remove('opacity-40', 'pointer-events-none');
                } else {
                    itemPaste.classList.add('opacity-40', 'pointer-events-none');
                }
            }
            if (itemPasteStyle) {
                if (studioClipboardStyle !== null && studioClipboardStyle !== undefined) {
                    itemPasteStyle.classList.remove('opacity-40', 'pointer-events-none');
                } else {
                    itemPasteStyle.classList.add('opacity-40', 'pointer-events-none');
                }
            }

            // 4. Trạng thái Ẩn/Hiện
            const isHidden = checkNodeHidden(node);
            const iconVis = document.getElementById('ctxIconVisibility');
            const textVis = document.getElementById('ctxTextVisibility');
            if (iconVis) iconVis.className = isHidden ? 'fa-solid fa-eye text-emerald-400 text-xs w-4' : 'fa-solid fa-eye-slash text-amber-400 text-xs w-4';
            if (textVis) textVis.innerText = isHidden ? 'Hiện phần tử (hiển thị lại)' : 'Ẩn phần tử (display: none)';

            // 5. Trạng thái Khóa
            const isLocked = studioLockedNodes.has(nodeId);
            const iconLock = document.getElementById('ctxIconLock');
            const textLock = document.getElementById('ctxTextLock');
            if (iconLock) iconLock.className = isLocked ? 'fa-solid fa-lock-open text-emerald-400 text-xs w-4' : 'fa-solid fa-lock text-yellow-400 text-xs w-4';
            if (textLock) textLock.innerText = isLocked ? 'Mở khóa layer này' : 'Khóa layer này';

            // 6. Định vị tránh tràn màn hình (Viewport Clamping)
            menu.classList.remove('hidden');
            const menuW = 265;
            const menuH = 480;
            let posX = clientX;
            let posY = clientY;

            if (posX + menuW > window.innerWidth - 12) {
                posX = window.innerWidth - menuW - 12;
            }
            if (posY + menuH > window.innerHeight - 12) {
                posY = window.innerHeight - menuH - 12;
            }
            posX = Math.max(12, posX);
            posY = Math.max(12, posY);

            menu.style.left = posX + 'px';
            menu.style.top = posY + 'px';
        }

        function closeContextMenu() {
            const menu = document.getElementById('csContextMenu');
            if (menu) menu.classList.add('hidden');
        }

        function handleContextMenuAction(action, arg = null) {
            closeContextMenu();
            switch (action) {
                case 'inline-edit':
                    startInlineTextEdit();
                    break;
                case 'change-tag':
                    changeCurrentNodeTag(arg);
                    break;
                case 'change-img':
                    promptChangeImage();
                    break;
                case 'change-link':
                    promptChangeLink();
                    break;
                case 'cut':
                    cutCurrentNode();
                    break;
                case 'copy':
                    copyCurrentNode();
                    break;
                case 'paste':
                    pasteCopiedNode();
                    break;
                case 'copy-style':
                    copyCurrentNodeStyles();
                    break;
                case 'paste-style':
                    pasteNodeStyles();
                    break;
                case 'duplicate':
                    duplicateCurrentNode();
                    break;
                case 'move-up':
                    moveCurrentNodeUp();
                    break;
                case 'move-down':
                    moveCurrentNodeDown();
                    break;
                case 'move-top':
                    moveCurrentNodeToTop();
                    break;
                case 'move-bottom':
                    moveCurrentNodeToBottom();
                    break;
                case 'select-parent':
                    selectParentNode();
                    break;
                case 'toggle-visibility':
                    toggleCurrentNodeVisibility();
                    break;
                case 'toggle-lock':
                    toggleCurrentNodeLock();
                    break;
                case 'undo':
                    studioUndo();
                    break;
                case 'redo':
                    studioRedo();
                    break;
                case 'delete':
                    deleteCurrentNode();
                    break;
            }
        }

        function changeCurrentNodeTag(newTag) {
            if (!selectedAstNodeId || !newTag) return;
            const node = astMapper.astMap[selectedAstNodeId];
            const oldTag = node ? node.tag : 'thẻ';
            const updated = astMapper.changeNodeTag(selectedAstNodeId, newTag);
            syncCodeEditorFromAst(updated);
            pushStudioHistory(`Đổi thẻ <${oldTag}> thành <${newTag}>`, updated, selectedAstNodeId);
            refreshStudioPreview();
            setTimeout(() => {
                selectAstNode(selectedAstNodeId);
            }, 250);
            renderAstLayerTree();
            showToastNotification(`Đã đổi thẻ <${oldTag}> thành <${newTag}>`);
        }

        function copyCurrentNode() {
            if (!selectedAstNodeId) return;
            const node = astMapper.astMap[selectedAstNodeId];
            if (!node) return;
            studioClipboardNode = astMapper.rawHTML.substring(node.startOffset, node.endOffset);
            showToastNotification(`Đã sao chép <${node.tag}> vào clipboard`);
        }

        function cutCurrentNode() {
            if (!selectedAstNodeId) return;
            const node = astMapper.astMap[selectedAstNodeId];
            if (!node) return;
            studioClipboardNode = astMapper.rawHTML.substring(node.startOffset, node.endOffset);
            deleteCurrentNode(false);
            showToastNotification(`Đã cắt <${node.tag}> vào clipboard`);
        }

        function pasteCopiedNode() {
            if (!selectedAstNodeId || !studioClipboardNode) return;
            const updated = astMapper.insertNodeAfter(selectedAstNodeId, studioClipboardNode);
            syncCodeEditorFromAst(updated);
            pushStudioHistory('Dán phần tử bên dưới', updated, selectedAstNodeId);
            refreshStudioPreview();
            renderAstLayerTree();
            showToastNotification('Đã dán phần tử vào mã nguồn');
        }

        function copyCurrentNodeStyles() {
            if (!selectedAstNodeId) return;
            const node = astMapper.astMap[selectedAstNodeId];
            studioClipboardStyle = (node && node.attrs['style']) ? node.attrs['style'].value : '';
            showToastNotification('Đã sao chép Style CSS của layer');
        }

        function pasteNodeStyles() {
            if (!selectedAstNodeId || studioClipboardStyle === null || studioClipboardStyle === undefined) return;
            const updated = astMapper.spliceAttribute(selectedAstNodeId, 'style', studioClipboardStyle);
            syncCodeEditorFromAst(updated);
            pushStudioHistory('Dán Style CSS', updated, selectedAstNodeId);
            refreshStudioPreview();
            selectAstNode(selectedAstNodeId);
            showToastNotification('Đã dán Style CSS vào layer');
        }

        function moveCurrentNodeToTop() {
            if (!selectedAstNodeId) return;
            const node = astMapper.astMap[selectedAstNodeId];
            const updated = astMapper.moveNodeToTop(selectedAstNodeId);
            syncCodeEditorFromAst(updated);
            pushStudioHistory(`Đưa <${node ? node.tag : 'thẻ'}> lên đầu`, updated, selectedAstNodeId);
            refreshStudioPreview();
            setTimeout(() => {
                selectAstNode(selectedAstNodeId);
            }, 250);
            renderAstLayerTree();
            showToastNotification('Đã đưa phần tử lên đầu cùng cấp');
        }

        function moveCurrentNodeToBottom() {
            if (!selectedAstNodeId) return;
            const node = astMapper.astMap[selectedAstNodeId];
            const updated = astMapper.moveNodeToBottom(selectedAstNodeId);
            syncCodeEditorFromAst(updated);
            pushStudioHistory(`Đưa <${node ? node.tag : 'thẻ'}> xuống cuối`, updated, selectedAstNodeId);
            refreshStudioPreview();
            setTimeout(() => {
                selectAstNode(selectedAstNodeId);
            }, 250);
            renderAstLayerTree();
            showToastNotification('Đã đưa phần tử xuống cuối cùng cấp');
        }

        function toggleCurrentNodeVisibility(nodeId = null) {
            const targetId = nodeId || selectedAstNodeId;
            if (!targetId) return;
            const node = astMapper.astMap[targetId];
            if (!node) return;

            const sVal = node.attrs['style'] ? node.attrs['style'].value : '';
            const isHidden = checkNodeHidden(node);
            let newStyle = '';

            if (isHidden) {
                newStyle = sVal.replace(/display:\s*none;?/ig, '').trim();
            } else {
                newStyle = (sVal ? sVal.replace(/;?\s*$/, '; ') : '') + 'display: none;';
            }

            const updated = astMapper.spliceAttribute(targetId, 'style', newStyle);
            syncCodeEditorFromAst(updated);
            pushStudioHistory(isHidden ? `Hiện <${node.tag}>` : `Ẩn <${node.tag}>`, updated, targetId);
            refreshStudioPreview();
            if (selectedAstNodeId === targetId) {
                selectAstNode(targetId);
            }
            renderAstLayerTree();
            showToastNotification(isHidden ? 'Đã hiển thị layer' : 'Đã ẩn layer (display: none)');
        }

        function toggleCurrentNodeLock(nodeId = null) {
            const targetId = nodeId || selectedAstNodeId;
            if (!targetId) return;
            const node = astMapper.astMap[targetId];

            if (studioLockedNodes.has(targetId)) {
                studioLockedNodes.delete(targetId);
                showToastNotification(`Đã mở khóa <${node ? node.tag : 'layer'}>`);
            } else {
                studioLockedNodes.add(targetId);
                showToastNotification(`Đã khóa <${node ? node.tag : 'layer'}>`);
            }
            if (selectedAstNodeId === targetId) {
                requestNodeRect(targetId);
            }
            renderAstLayerTree();
        }

        // Nhân bản phần tử (Duplicate)
        function duplicateCurrentNode() {
            if (!selectedAstNodeId) return;
            const node = astMapper.astMap[selectedAstNodeId];
            const updated = astMapper.duplicateNode(selectedAstNodeId);
            syncCodeEditorFromAst(updated);
            pushStudioHistory(`Nhân bản <${node ? node.tag : 'thẻ'}>`, updated, selectedAstNodeId);
            refreshStudioPreview();
            renderAstLayerTree();
            showToastNotification(`Đã nhân bản <${node ? node.tag : 'thẻ'}>`);
        }

        // Xóa phần tử (Delete)
        function deleteCurrentNode(askConfirm = true) {
            if (!selectedAstNodeId) return;
            const node = astMapper.astMap[selectedAstNodeId];
            if (askConfirm && !confirm('Bạn có chắc chắn muốn xóa phần tử này khỏi mã nguồn không?')) return;
            const updated = astMapper.deleteNode(selectedAstNodeId);
            syncCodeEditorFromAst(updated);
            pushStudioHistory(`Xóa <${node ? node.tag : 'thẻ'}>`, updated, null);
            selectedAstNodeId = null;
            document.getElementById('csGizmoBox').classList.add('hidden');
            refreshStudioPreview();
            renderAstLayerTree();
            showToastNotification('Đã xóa phần tử khỏi mã nguồn');
        }

        // Chọn thẻ cha (Select Parent)
        function selectParentNode() {
            if (!selectedAstNodeId) return;
            const node = astMapper.astMap[selectedAstNodeId];
            if (node && node.parentId) {
                selectAstNode(node.parentId);
            }
        }

        function promptChangeImage() {
            if (!selectedAstNodeId) return;
            const node = astMapper.astMap[selectedAstNodeId];
            const currentSrc = node && node.attrs['src'] ? node.attrs['src'].value : '';
            const newSrc = prompt('Nhập đường dẫn ảnh mới (src):', currentSrc);
            if (newSrc !== null && newSrc !== currentSrc) {
                onInspectorAttrChange('src', newSrc);
            }
        }

        function promptChangeLink() {
            if (!selectedAstNodeId) return;
            const node = astMapper.astMap[selectedAstNodeId];
            const currentHref = node && node.attrs['href'] ? node.attrs['href'].value : '';
            const newHref = prompt('Nhập liên kết mới (href):', currentHref);
            if (newHref !== null && newHref !== currentHref) {
                onInspectorAttrChange('href', newHref);
            }
        }

        // Chọn node từ Cây Layer hoặc Breadcrumb
        function selectAstNode(nodeId) {
            selectedAstNodeId = nodeId;
            const node = astMapper.astMap[nodeId];
            if (node) {
                const nodeData = {
                    nodeId: node.id,
                    tagName: node.tag,
                    className: node.attrs['class'] ? node.attrs['class'].value : '',
                    text: node.textContent,
                    attributes: Object.fromEntries(Object.values(node.attrs).map(a => [a.name, a.value]))
                };
                selectedNodeMeta = nodeData;
                populateInspectorWithNode(nodeData);
                highlightCodeAtOffset(node.startOffset);
            }
            requestNodeRect(nodeId);
            renderAstLayerTree();
        }

