/* ==============================================================
   APEX VISUAL WEB BUILDER - AST SOURCE MAPPER & 2-WAY SYNC
   Non-Destructive Tokenizer, DOM Tree Mutator & String Splicing
   ============================================================== */

        // ============================================================
        class ApexASTSourceMapper {
            constructor() {
                this.rawHTML = '';
                this.astMap = {};
                this.tree = [];
                this.nodeCounter = 0;
                this.voidElements = new Set([
                    'area', 'base', 'br', 'col', 'embed', 'hr', 'img', 'input',
                    'link', 'meta', 'param', 'source', 'track', 'wbr'
                ]);
            }

            parse(htmlString) {
                this.rawHTML = htmlString || '';
                this.astMap = {};
                this.tree = [];
                this.nodeCounter = 0;

                const stack = [];
                let i = 0;
                const len = this.rawHTML.length;

                while (i < len) {
                    const nextLt = this.rawHTML.indexOf('<', i);
                    if (nextLt === -1) break;

                    // 1. Comment <!-- ... -->
                    if (this.rawHTML.startsWith('<!--', nextLt)) {
                        const commentEnd = this.rawHTML.indexOf('-->', nextLt + 4);
                        i = commentEnd === -1 ? len : commentEnd + 3;
                        continue;
                    }

                    // 2. DOCTYPE <!DOCTYPE ...>
                    if (this.rawHTML.substring(nextLt, nextLt + 9).toUpperCase() === '<!DOCTYPE') {
                        const docEnd = this.rawHTML.indexOf('>', nextLt + 9);
                        i = docEnd === -1 ? len : docEnd + 1;
                        continue;
                    }

                    // 3. Closing tag </tag>
                    if (this.rawHTML.startsWith('</', nextLt)) {
                        const closeTagEnd = this.rawHTML.indexOf('>', nextLt + 2);
                        if (closeTagEnd === -1) {
                            i = len;
                            break;
                        }
                        const closeTagName = this.rawHTML.substring(nextLt + 2, closeTagEnd).trim().split(/\s+/)[0].toLowerCase();

                        let matchIdx = -1;
                        for (let s = stack.length - 1; s >= 0; s--) {
                            if (stack[s].tag === closeTagName) {
                                matchIdx = s;
                                break;
                            }
                        }
                        if (matchIdx !== -1) {
                            const node = stack[matchIdx];
                            node.closeTagStart = nextLt;
                            node.closeTagEnd = closeTagEnd + 1;
                            node.endOffset = closeTagEnd + 1;
                            node.innerStart = node.openTagEnd;
                            node.innerEnd = node.closeTagStart;
                            node.innerHTML = this.rawHTML.substring(node.innerStart, node.innerEnd);
                            node.textContent = node.innerHTML.replace(/<[^>]*>/g, '').trim();
                            stack.splice(matchIdx);
                        }
                        i = closeTagEnd + 1;
                        continue;
                    }

                    // 4. Opening tag <tag ...>
                    const tagChar = this.rawHTML[nextLt + 1];
                    if (!tagChar || !/[a-zA-Z]/.test(tagChar)) {
                        i = nextLt + 1;
                        continue;
                    }

                    let inSingleQuote = false;
                    let inDoubleQuote = false;
                    let openTagEnd = -1;
                    for (let j = nextLt + 1; j < len; j++) {
                        const ch = this.rawHTML[j];
                        if (ch === '"' && !inSingleQuote) inDoubleQuote = !inDoubleQuote;
                        else if (ch === "'" && !inDoubleQuote) inSingleQuote = !inSingleQuote;
                        else if (ch === '>' && !inSingleQuote && !inDoubleQuote) {
                            openTagEnd = j + 1;
                            break;
                        }
                    }

                    if (openTagEnd === -1) {
                        i = len;
                        break;
                    }

                    const openTagRaw = this.rawHTML.substring(nextLt, openTagEnd);
                    const isSelfClosing = openTagRaw.endsWith('/>');

                    const tagMatch = openTagRaw.match(/^<([a-zA-Z0-9\-]+)/);
                    if (!tagMatch) {
                        i = openTagEnd;
                        continue;
                    }
                    const tagName = tagMatch[1].toLowerCase();

                    this.nodeCounter++;
                    const nodeId = 'node-' + this.nodeCounter;

                    const attrs = {};
                    const attrList = [];
                    const attrRegex = /([a-zA-Z0-9_:\-]+)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+)))?/g;
                    attrRegex.lastIndex = tagMatch[0].length;
                    let attrM;
                    while ((attrM = attrRegex.exec(openTagRaw)) !== null) {
                        const attrName = attrM[1].toLowerCase();
                        if (attrName === '/' || attrName === '') continue;
                        const attrVal = attrM[2] !== undefined ? attrM[2] : (attrM[3] !== undefined ? attrM[3] : (attrM[4] !== undefined ? attrM[4] : ''));
                        const rawAttrStr = attrM[0];
                        const attrStartOffset = nextLt + attrM.index;
                        const attrEndOffset = attrStartOffset + rawAttrStr.length;

                        const attrObj = {
                            name: attrName,
                            value: attrVal,
                            raw: rawAttrStr,
                            startOffset: attrStartOffset,
                            endOffset: attrEndOffset
                        };
                        attrs[attrName] = attrObj;
                        attrList.push(attrObj);
                    }

                    const node = {
                        id: nodeId,
                        tag: tagName,
                        startOffset: nextLt,
                        endOffset: openTagEnd,
                        openTagStart: nextLt,
                        openTagEnd: openTagEnd,
                        openTagRaw: openTagRaw,
                        closeTagStart: -1,
                        closeTagEnd: -1,
                        innerStart: openTagEnd,
                        innerEnd: openTagEnd,
                        innerHTML: '',
                        textContent: '',
                        attrs: attrs,
                        attrList: attrList,
                        classes: attrs['class'] ? attrs['class'].value.split(/\s+/).filter(Boolean) : [],
                        parentId: stack.length > 0 ? stack[stack.length - 1].id : null,
                        children: []
                    };

                    this.astMap[nodeId] = node;

                    if (node.parentId && this.astMap[node.parentId]) {
                        this.astMap[node.parentId].children.push(nodeId);
                    } else {
                        this.tree.push(nodeId);
                    }

                    // Special tags: <script> and <style>
                    if (tagName === 'script' || tagName === 'style') {
                        const endTagStr = '</' + tagName + '>';
                        const endTagIdx = this.rawHTML.toLowerCase().indexOf(endTagStr, openTagEnd);
                        if (endTagIdx !== -1) {
                            node.closeTagStart = endTagIdx;
                            node.closeTagEnd = endTagIdx + endTagStr.length;
                            node.endOffset = node.closeTagEnd;
                            node.innerStart = openTagEnd;
                            node.innerEnd = endTagIdx;
                            node.innerHTML = this.rawHTML.substring(node.innerStart, node.innerEnd);
                            node.textContent = node.innerHTML;
                            i = node.closeTagEnd;
                        } else {
                            i = openTagEnd;
                        }
                        continue;
                    }

                    if (this.voidElements.has(tagName) || isSelfClosing) {
                        node.endOffset = openTagEnd;
                        i = openTagEnd;
                        continue;
                    }

                    stack.push(node);
                    i = openTagEnd;
                }

                return this;
            }

            // Tiêm data-ve-id vào bản Preview Iframe (Không ghi vào file đĩa hay CSDL!)
            generateInstrumentedHTML() {
                if (!this.rawHTML) return '';
                let result = '';
                let lastPos = 0;

                const sortedNodes = Object.values(this.astMap).sort((a, b) => a.openTagStart - b.openTagStart);

                for (const node of sortedNodes) {
                    result += this.rawHTML.substring(lastPos, node.openTagStart);
                    let openTag = node.openTagRaw;
                    const insertIdStr = ` data-ve-id="${node.id}"`;
                    if (openTag.endsWith('/>')) {
                        openTag = openTag.substring(0, openTag.length - 2) + insertIdStr + '/>';
                    } else if (openTag.endsWith('>')) {
                        openTag = openTag.substring(0, openTag.length - 1) + insertIdStr + '>';
                    }
                    result += openTag;
                    lastPos = node.openTagEnd;
                }

                result += this.rawHTML.substring(lastPos);

                const helperScript = this.getHelperScript();
                const headLower = result.toLowerCase().indexOf('<head>');
                if (headLower !== -1) {
                    result = result.substring(0, headLower + 6) + helperScript + result.substring(headLower + 6);
                } else {
                    result = helperScript + result;
                }

                return result;
            }

            getHelperScript() {
                return `\n<base href="/">\n<script id="__APEX_BUILDER_HELPER_SCRIPT__">\n` +
                    `(function() {\n` +
                    `    window.__APEX_BUILDER_ACTIVE__ = true;\n` +
                    `    window.__APEX_FREEZE_MODE__ = true;\n` +
                    `    document.addEventListener('DOMContentLoaded', function() {\n` +
                    `        var observer = new MutationObserver(function(mutations) {\n` +
                    `            mutations.forEach(function(m) {\n` +
                    `                m.addedNodes.forEach(function(n) {\n` +
                    `                    if (n.nodeType === 1 && !n.getAttribute('data-ve-id')) {\n` +
                    `                        n.setAttribute('data-ve-ephemeral', 'true');\n` +
                    `                        var desc = n.querySelectorAll('*');\n` +
                    `                        for (var i = 0; i < desc.length; i++) {\n` +
                    `                            if (!desc[i].getAttribute('data-ve-id')) desc[i].setAttribute('data-ve-ephemeral', 'true');\n` +
                    `                        }\n` +
                    `                    }\n` +
                    `                });\n` +
                    `            });\n` +
                    `        });\n` +
                    `        if (document.body) observer.observe(document.body, { childList: true, subtree: true });\n` +
                    `    });\n` +
                    `    function getValidTarget(target) {\n` +
                    `        var curr = target;\n` +
                    `        while (curr && curr !== document.body && curr !== document.documentElement) {\n` +
                    `            if (curr.hasAttribute('data-ve-id') && !curr.hasAttribute('data-ve-ephemeral')) return curr;\n` +
                    `            curr = curr.parentElement;\n` +
                    `        }\n` +
                    `        return (curr && curr.hasAttribute('data-ve-id')) ? curr : null;\n` +
                    `    }\n` +
                    `    function getNodeData(el) {\n` +
                    `        if (!el) return null;\n` +
                    `        var rect = el.getBoundingClientRect();\n` +
                    `        var style = window.getComputedStyle(el);\n` +
                    `        var attrs = {};\n` +
                    `        for (var i = 0; i < el.attributes.length; i++) {\n` +
                    `            attrs[el.attributes[i].name] = el.attributes[i].value;\n` +
                    `        }\n` +
                    `        return {\n` +
                    `            nodeId: el.getAttribute('data-ve-id'),\n` +
                    `            tagName: el.tagName.toLowerCase(),\n` +
                    `            className: el.className || '',\n` +
                    `            text: el.innerText || el.textContent || '',\n` +
                    `            rect: { top: rect.top, left: rect.left, width: rect.width, height: rect.height },\n` +
                    `            style: {\n` +
                    `                display: style.display,\n` +
                    `                flexDirection: style.flexDirection,\n` +
                    `                justifyContent: style.justifyContent,\n` +
                    `                alignItems: style.alignItems,\n` +
                    `                gap: style.gap,\n` +
                    `                position: style.position,\n` +
                    `                top: style.top,\n` +
                    `                right: style.right,\n` +
                    `                bottom: style.bottom,\n` +
                    `                left: style.left,\n` +
                    `                zIndex: style.zIndex,\n` +
                    `                fontSize: style.fontSize,\n` +
                    `                fontWeight: style.fontWeight,\n` +
                    `                color: style.color,\n` +
                    `                textAlign: style.textAlign,\n` +
                    `                textTransform: style.textTransform,\n` +
                    `                lineHeight: style.lineHeight,\n` +
                    `                backgroundColor: style.backgroundColor,\n` +
                    `                backgroundImage: style.backgroundImage,\n` +
                    `                opacity: style.opacity,\n` +
                    `                boxShadow: style.boxShadow,\n` +
                    `                width: style.width,\n` +
                    `                height: style.height,\n` +
                    `                maxWidth: style.maxWidth,\n` +
                    `                minWidth: style.minWidth,\n` +
                    `                marginTop: style.marginTop,\n` +
                    `                marginRight: style.marginRight,\n` +
                    `                marginBottom: style.marginBottom,\n` +
                    `                marginLeft: style.marginLeft,\n` +
                    `                paddingTop: style.paddingTop,\n` +
                    `                paddingRight: style.paddingRight,\n` +
                    `                paddingBottom: style.paddingBottom,\n` +
                    `                paddingLeft: style.paddingLeft,\n` +
                    `                padding: style.padding,\n` +
                    `                margin: style.margin,\n` +
                    `                borderRadius: style.borderRadius,\n` +
                    `                borderWidth: style.borderWidth,\n` +
                    `                borderStyle: style.borderStyle,\n` +
                    `                borderColor: style.borderColor\n` +
                    `            },\n` +
                    `            attributes: attrs\n` +
                    `        };\n` +
                    `    }\n` +
                    `    document.addEventListener('click', function(e) {\n` +
                    `        if (window.__APEX_FREEZE_MODE__) { e.preventDefault(); e.stopPropagation(); }\n` +
                    `        var t = getValidTarget(e.target);\n` +
                    `        if (t) window.parent.postMessage({ type: 'APEX_BUILDER_NODE_CLICKED', data: getNodeData(t) }, '*');\n` +
                    `    }, true);\n` +
                    `    document.addEventListener('dblclick', function(e) {\n` +
                    `        if (window.__APEX_FREEZE_MODE__) { e.preventDefault(); e.stopPropagation(); }\n` +
                    `        var t = getValidTarget(e.target);\n` +
                    `        if (t) window.parent.postMessage({ type: 'APEX_BUILDER_NODE_DBLCLICKED', data: getNodeData(t) }, '*');\n` +
                    `    }, true);\n` +
                    `    document.addEventListener('mouseover', function(e) {\n` +
                    `        if (!window.__APEX_BUILDER_ACTIVE__ || !window.__APEX_FREEZE_MODE__) return;\n` +
                    `        var t = getValidTarget(e.target);\n` +
                    `        if (t) {\n` +
                    `            var r = t.getBoundingClientRect();\n` +
                    `            window.parent.postMessage({ type: 'APEX_BUILDER_NODE_HOVERED', nodeId: t.getAttribute('data-ve-id'), tagName: t.tagName.toLowerCase(), className: t.className || '', rect: { top: r.top, left: r.left, width: r.width, height: r.height } }, '*');\n` +
                    `        }\n` +
                    `    }, true);\n` +
                    `    document.addEventListener('mouseleave', function() {\n` +
                    `        window.parent.postMessage({ type: 'APEX_BUILDER_HOVER_LEAVE' }, '*');\n` +
                    `    });\n` +
                    `    function notifyVp() {\n` +
                    `        window.parent.postMessage({ type: 'APEX_BUILDER_VIEWPORT_CHANGED' }, '*');\n` +
                    `    }\n` +
                    `    window.addEventListener('scroll', notifyVp, { passive: true });\n` +
                    `    window.addEventListener('resize', notifyVp, { passive: true });\n` +
                    `    window.addEventListener('mouseup', function() {\n` +
                    `        window.parent.postMessage({ type: 'APEX_BUILDER_IFRAME_MOUSEUP' }, '*');\n` +
                    `    });\n` +
                    `    window.addEventListener('pointerup', function() {\n` +
                    `        window.parent.postMessage({ type: 'APEX_BUILDER_IFRAME_MOUSEUP' }, '*');\n` +
                    `    });\n` +
                    `    document.addEventListener('contextmenu', function(e) {\n` +
                    `        if (window.__APEX_FREEZE_MODE__ || window.__APEX_BUILDER_ACTIVE__) {\n` +
                    `            e.preventDefault();\n` +
                    `            e.stopPropagation();\n` +
                    `            var t = getValidTarget(e.target);\n` +
                    `            window.parent.postMessage({\n` +
                    `                type: 'APEX_BUILDER_CONTEXT_MENU',\n` +
                    `                nodeId: t ? t.getAttribute('data-ve-id') : null,\n` +
                    `                clientX: e.clientX,\n` +
                    `                clientY: e.clientY,\n` +
                    `                nodeData: t ? getNodeData(t) : null\n` +
                    `            }, '*');\n` +
                    `        }\n` +
                    `    }, true);\n` +
                    `    window.addEventListener('message', function(ev) {\n` +
                    `        if (!ev.data || typeof ev.data !== 'object') return;\n` +
                    `        var m = ev.data;\n` +
                    `        if (m.type === 'APEX_CMD_SET_FREEZE_MODE') {\n` +
                    `            window.__APEX_FREEZE_MODE__ = !!m.enabled;\n` +
                    `            var st = document.getElementById('__apex_freeze_css__');\n` +
                    `            if (m.enabled) {\n` +
                    `                if (!st) {\n` +
                    `                    st = document.createElement('style');\n` +
                    `                    st.id = '__apex_freeze_css__';\n` +
                    `                    st.innerHTML = '[data-aos] { opacity: 1 !important; transform: none !important; transition: none !important; animation: none !important; } a { cursor: pointer; }';\n` +
                    `                    document.head.appendChild(st);\n` +
                    `                }\n` +
                    `            } else if (st) st.remove();\n` +
                    `        }\n` +
                    `        if (m.type === 'APEX_CMD_GET_NODE_RECT') {\n` +
                    `            var el = document.querySelector('[data-ve-id=\"' + m.nodeId + '\"]');\n` +
                    `            if (el) window.parent.postMessage({ type: 'APEX_BUILDER_RECT_RESPONSE', data: getNodeData(el) }, '*');\n` +
                    `        }\n` +
                    `        if (m.type === 'APEX_CMD_UPDATE_STYLE_TEMP') {\n` +
                    `            var el = document.querySelector('[data-ve-id=\"' + m.nodeId + '\"]');\n` +
                    `            if (el && m.styles) {\n` +
                    `                for (var k in m.styles) el.style[k] = m.styles[k];\n` +
                    `            }\n` +
                    `        }\n` +
                    `        if (m.type === 'APEX_CMD_UPDATE_CLASS_TEMP') {\n` +
                    `            var el = document.querySelector('[data-ve-id=\"' + m.nodeId + '\"]');\n` +
                    `            if (el) el.className = m.className;\n` +
                    `        }\n` +
                    `        if (m.type === 'APEX_CMD_UPDATE_TEXT_TEMP') {\n` +
                    `            var el = document.querySelector('[data-ve-id=\"' + m.nodeId + '\"]');\n` +
                    `            if (el) el.innerText = m.text;\n` +
                    `        }\n` +
                    `        if (m.type === 'APEX_CMD_START_INLINE_EDIT') {\n` +
                    `            var el = document.querySelector('[data-ve-id=\"' + m.nodeId + '\"]');\n` +
                    `            if (el) {\n` +
                    `                el.setAttribute('contenteditable', 'true');\n` +
                    `                el.style.outline = '2px dashed #06b6d4';\n` +
                    `                el.style.outlineOffset = '2px';\n` +
                    `                el.focus();\n` +
                    `                function onBlur() {\n` +
                    `                    el.removeAttribute('contenteditable');\n` +
                    `                    el.style.outline = '';\n` +
                    `                    el.style.outlineOffset = '';\n` +
                    `                    el.removeEventListener('blur', onBlur);\n` +
                    `                    window.parent.postMessage({ type: 'APEX_BUILDER_INLINE_EDIT_FINISHED', nodeId: m.nodeId, newText: el.innerText }, '*');\n` +
                    `                }\n` +
                    `                el.addEventListener('blur', onBlur);\n` +
                    `            }\n` +
                    `        }\n` +
                    `    });\n` +
                    `    window.parent.postMessage({ type: 'APEX_BUILDER_READY' }, '*');\n` +
                    `})();\n` +
                    '<' + '/script>\n';
            }

            // --- THUẬT TOÁN NON-DESTRUCTIVE STRING SPLICING ---
            spliceText(nodeId, newText) {
                const node = this.astMap[nodeId];
                if (!node) return this.rawHTML;

                const before = this.rawHTML.substring(0, node.innerStart);
                const after = this.rawHTML.substring(node.innerEnd);
                this.rawHTML = before + newText + after;
                this.parse(this.rawHTML);
                return this.rawHTML;
            }

            spliceAttribute(nodeId, attrName, attrValue) {
                const node = this.astMap[nodeId];
                if (!node) return this.rawHTML;

                attrName = attrName.toLowerCase();
                const existingAttr = node.attrs[attrName];

                if (existingAttr) {
                    const before = this.rawHTML.substring(0, existingAttr.startOffset);
                    const after = this.rawHTML.substring(existingAttr.endOffset);
                    let replacement = '';
                    if (attrValue !== null && attrValue !== undefined && attrValue !== false && attrValue !== '') {
                        replacement = `${attrName}="${String(attrValue).replace(/"/g, '&quot;')}"`;
                    }
                    this.rawHTML = before + replacement + after;
                } else {
                    if (attrValue !== null && attrValue !== undefined && attrValue !== false && attrValue !== '') {
                        const openRaw = node.openTagRaw;
                        let insertRelPos = openRaw.length - 1;
                        if (openRaw.endsWith('/>')) insertRelPos = openRaw.length - 2;
                        const insertAbsPos = node.openTagStart + insertRelPos;

                        const before = this.rawHTML.substring(0, insertAbsPos);
                        const after = this.rawHTML.substring(insertAbsPos);
                        const insertion = ` ${attrName}="${String(attrValue).replace(/"/g, '&quot;')}"`;
                        this.rawHTML = before + insertion + after;
                    }
                }
                this.parse(this.rawHTML);
                return this.rawHTML;
            }

            spliceClasses(nodeId, classListOrString) {
                const classStr = Array.isArray(classListOrString) ? classListOrString.join(' ') : String(classListOrString);
                return this.spliceAttribute(nodeId, 'class', classStr.trim());
            }

            spliceStyle(nodeId, styleObj) {
                const node = this.astMap[nodeId];
                if (!node) return this.rawHTML;

                const existingStyleStr = node.attrs['style'] ? node.attrs['style'].value : '';
                const styles = {};
                existingStyleStr.split(';').forEach(s => {
                    const parts = s.split(':');
                    if (parts.length >= 2) {
                        styles[parts[0].trim()] = parts.slice(1).join(':').trim();
                    }
                });

                for (const k in styleObj) {
                    if (styleObj[k] === null || styleObj[k] === '') {
                        delete styles[k];
                    } else {
                        styles[k] = styleObj[k];
                    }
                }

                const newStyleStr = Object.keys(styles).map(k => `${k}: ${styles[k]}`).join('; ');
                return this.spliceAttribute(nodeId, 'style', newStyleStr || null);
            }

            deleteNode(nodeId) {
                const node = this.astMap[nodeId];
                if (!node) return this.rawHTML;

                this.rawHTML = this.rawHTML.substring(0, node.startOffset) + this.rawHTML.substring(node.endOffset);
                this.parse(this.rawHTML);
                return this.rawHTML;
            }

            duplicateNode(nodeId) {
                const node = this.astMap[nodeId];
                if (!node) return this.rawHTML;

                const nodeStr = this.rawHTML.substring(node.startOffset, node.endOffset);
                this.rawHTML = this.rawHTML.substring(0, node.endOffset) + '\n' + nodeStr + this.rawHTML.substring(node.endOffset);
                this.parse(this.rawHTML);
                return this.rawHTML;
            }

            moveNodeUp(nodeId) {
                const node = this.astMap[nodeId];
                if (!node) return this.rawHTML;

                const siblings = (node.parentId && this.astMap[node.parentId])
                    ? this.astMap[node.parentId].children
                    : this.tree;
                const idx = siblings.indexOf(nodeId);
                if (idx <= 0) return this.rawHTML;

                const prevNode = this.astMap[siblings[idx - 1]];
                if (!prevNode) return this.rawHTML;

                const pStart = prevNode.startOffset;
                const pEnd = prevNode.endOffset;
                const cStart = node.startOffset;
                const cEnd = node.endOffset;

                const prevText = this.rawHTML.substring(pStart, pEnd);
                const inBetween = this.rawHTML.substring(pEnd, cStart);
                const currText = this.rawHTML.substring(cStart, cEnd);

                this.rawHTML = this.rawHTML.substring(0, pStart) + currText + inBetween + prevText + this.rawHTML.substring(cEnd);
                this.parse(this.rawHTML);
                return this.rawHTML;
            }

            moveNodeDown(nodeId) {
                const node = this.astMap[nodeId];
                if (!node) return this.rawHTML;

                const siblings = (node.parentId && this.astMap[node.parentId])
                    ? this.astMap[node.parentId].children
                    : this.tree;
                const idx = siblings.indexOf(nodeId);
                if (idx === -1 || idx >= siblings.length - 1) return this.rawHTML;

                const nextNode = this.astMap[siblings[idx + 1]];
                if (!nextNode) return this.rawHTML;

                const cStart = node.startOffset;
                const cEnd = node.endOffset;
                const nStart = nextNode.startOffset;
                const nEnd = nextNode.endOffset;

                const currText = this.rawHTML.substring(cStart, cEnd);
                const inBetween = this.rawHTML.substring(cEnd, nStart);
                const nextText = this.rawHTML.substring(nStart, nEnd);

                this.rawHTML = this.rawHTML.substring(0, cStart) + nextText + inBetween + currText + this.rawHTML.substring(nEnd);
                this.parse(this.rawHTML);
                return this.rawHTML;
            }

            moveNodeToTop(nodeId) {
                const node = this.astMap[nodeId];
                if (!node) return this.rawHTML;

                const siblings = (node.parentId && this.astMap[node.parentId])
                    ? this.astMap[node.parentId].children
                    : this.tree;
                const idx = siblings.indexOf(nodeId);
                if (idx <= 0) return this.rawHTML;

                const firstNode = this.astMap[siblings[0]];
                if (!firstNode) return this.rawHTML;

                const cStart = node.startOffset;
                const cEnd = node.endOffset;
                const fStart = firstNode.startOffset;

                const currText = this.rawHTML.substring(cStart, cEnd);
                const beforeFirst = this.rawHTML.substring(0, fStart);
                const between = this.rawHTML.substring(fStart, cStart);
                const afterCurr = this.rawHTML.substring(cEnd);

                this.rawHTML = beforeFirst + currText + '\n' + between + afterCurr;
                this.parse(this.rawHTML);
                return this.rawHTML;
            }

            moveNodeToBottom(nodeId) {
                const node = this.astMap[nodeId];
                if (!node) return this.rawHTML;

                const siblings = (node.parentId && this.astMap[node.parentId])
                    ? this.astMap[node.parentId].children
                    : this.tree;
                const idx = siblings.indexOf(nodeId);
                if (idx === -1 || idx >= siblings.length - 1) return this.rawHTML;

                const lastNode = this.astMap[siblings[siblings.length - 1]];
                if (!lastNode) return this.rawHTML;

                const cStart = node.startOffset;
                const cEnd = node.endOffset;
                const lEnd = lastNode.endOffset;

                const currText = this.rawHTML.substring(cStart, cEnd);
                const beforeCurr = this.rawHTML.substring(0, cStart);
                const between = this.rawHTML.substring(cEnd, lEnd);
                const afterLast = this.rawHTML.substring(lEnd);

                this.rawHTML = beforeCurr + between + '\n' + currText + afterLast;
                this.parse(this.rawHTML);
                return this.rawHTML;
            }

            changeNodeTag(nodeId, newTagName) {
                const node = this.astMap[nodeId];
                if (!node || !newTagName) return this.rawHTML;
                newTagName = newTagName.toLowerCase().trim();
                const oldTag = node.tag.toLowerCase();
                if (oldTag === newTagName) return this.rawHTML;

                let html = this.rawHTML;

                // 1. Đổi thẻ đóng trước (từ phải sang trái để không lệch offset phía trước)
                if (node.closeTagStart !== undefined && node.closeTagEnd !== undefined) {
                    const closeBefore = html.substring(0, node.closeTagStart);
                    const closeAfter = html.substring(node.closeTagEnd);
                    html = closeBefore + '</' + newTagName + '>' + closeAfter;
                }

                // 2. Đổi tên thẻ mở
                const tagStart = node.openTagStart + 1; // sau dấu '<'
                const tagEnd = tagStart + oldTag.length;
                const openBefore = html.substring(0, tagStart);
                const openAfter = html.substring(tagEnd);
                html = openBefore + newTagName + openAfter;

                this.rawHTML = html;
                this.parse(this.rawHTML);
                return this.rawHTML;
            }

            insertNodeAfter(targetNodeId, htmlSnippet) {
                const node = this.astMap[targetNodeId];
                if (!node || !htmlSnippet) return this.rawHTML;

                const before = this.rawHTML.substring(0, node.endOffset);
                const after = this.rawHTML.substring(node.endOffset);
                this.rawHTML = before + '\n' + htmlSnippet + after;
                this.parse(this.rawHTML);
                return this.rawHTML;
            }

            getNodePath(nodeId) {
                const path = [];
                let curr = this.astMap[nodeId];
                while (curr) {
                    path.unshift(curr);
                    curr = curr.parentId ? this.astMap[curr.parentId] : null;
                }
                return path;
            }
        }

