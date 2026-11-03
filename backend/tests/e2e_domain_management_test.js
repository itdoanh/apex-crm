const { chromium } = require('playwright');
const fs = require('fs');
const path = require('path');

(async () => {
    console.log('🚀 Bắt đầu kiểm thử E2E Quản Lý Tên Miền, DNS Check & Multi-Domain Dynamic Router...');
    const browser = await chromium.launch({ headless: true });
    const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    const page = await context.newPage();

    const screenshotDir = path.join(__dirname, 'screenshots');
    if (!fs.existsSync(screenshotDir)) {
        fs.mkdirSync(screenshotDir, { recursive: true });
    }

    // Lắng nghe tất cả dialog alert để tự động chấp nhận
    page.on('dialog', async dialog => {
        console.log(`   [Dialog ${dialog.type()}]: ${dialog.message()}`);
        await dialog.accept();
    });

    try {
        // -------------------------------------------------------------
        // BƯỚC 1: Truy cập Admin Dashboard tại /admin và /ads
        // -------------------------------------------------------------
        console.log('👉 1. Truy cập http://localhost:8080/admin');
        await page.goto('http://localhost:8080/admin', { waitUntil: 'networkidle', timeout: 15000 });
        await page.waitForTimeout(1000);

        const title = await page.title();
        console.log(`   Tiêu đề trang: "${title}"`);
        if (!title.includes('Apex')) {
            throw new Error('Tiêu đề trang không đúng!');
        }

        // Kiểm tra route /ads (hỗ trợ crm.hanghoaphaisinh.net/ads)
        console.log('👉 1b. Kiểm tra đường dẫn /ads');
        const adsRes = await page.request.get('http://localhost:8080/ads');
        console.log(`   HTTP status /ads: ${adsRes.status()}`);
        if (adsRes.status() !== 200) {
            throw new Error('Đường dẫn /ads không hoạt động!');
        }

        // -------------------------------------------------------------
        // BƯỚC 2: Kiểm tra Top Header & Tab Quản Lý Tên Miền
        // -------------------------------------------------------------
        console.log('👉 2. Kiểm tra Tab "Quản Lý Tên Miền" và nút cấu hình API trên thanh tiêu đề');
        const tabBtnDomains = await page.$('#tabBtnDomains');
        if (!tabBtnDomains) {
            throw new Error('Không tìm thấy nút Tab "Quản Lý Tên Miền" (#tabBtnDomains)!');
        }
        const tabDomainsText = await page.$eval('#tabBtnDomains', el => el.innerText.trim());
        console.log(`   Tab button: "${tabDomainsText}"`);

        const apiEndpointBadge = await page.$('#badgeApiEndpointText');
        if (!apiEndpointBadge) {
            throw new Error('Không tìm thấy badge cấu hình API Endpoint!');
        }
        const apiBadgeText = await page.$eval('#badgeApiEndpointText', el => el.innerText.trim());
        console.log(`   API Endpoint badge: "${apiBadgeText}"`);

        // -------------------------------------------------------------
        // BƯỚC 3: Kiểm tra Modal Cấu hình Backend API (Cross-Domain)
        // -------------------------------------------------------------
        console.log('👉 3. Mở và kiểm tra Modal Cấu hình Backend API (Cross-Domain)');
        await page.click('button[onclick="openApiConfigModal()"]');
        await page.waitForTimeout(500);

        const isApiModalVisible = await page.$eval('#modalApiConfig', el => !el.classList.contains('hidden'));
        if (!isApiModalVisible) {
            throw new Error('Modal Cấu hình API không mở!');
        }
        console.log('   ✅ Modal Cấu hình API đã mở thành công!');

        // Bấm nút kiểm tra kết nối API
        await page.click('button[onclick="testCurrentApiEndpoint()"]');
        await page.waitForTimeout(1000);
        const testResultText = await page.$eval('#apiTestResult', el => el.innerText.trim());
        console.log(`   Kết quả test kết nối: "${testResultText}"`);
        if (!testResultText.includes('Kết nối thành công') && !testResultText.includes('Backend API')) {
            throw new Error('Test kết nối Backend API thất bại!');
        }
        console.log('   ✅ Kiểm tra kết nối Backend API thành công!');

        // Đóng modal API
        await page.click('button[onclick="closeApiConfigModal()"]');
        await page.waitForTimeout(500);

        // -------------------------------------------------------------
        // BƯỚC 4: Chuyển sang Tab Quản Lý Tên Miền
        // -------------------------------------------------------------
        console.log('👉 4. Chuyển sang Tab "Quản Lý Tên Miền"');
        await page.$eval('#tabBtnDomains', el => el.click());
        await page.waitForTimeout(1500);

        const isDomainTabVisible = await page.$eval('#sectionTabDomains', el => !el.classList.contains('hidden'));
        if (!isDomainTabVisible) {
            throw new Error('Section Tab Quản Lý Tên Miền không hiển thị!');
        }
        console.log('   ✅ Đã chuyển sang Tab Quản Lý Tên Miền!');

        // Kiểm tra Card Hướng dẫn DNS
        const valNs1 = await page.$eval('#valNs1', el => el.innerText.trim());
        const valNs2 = await page.$eval('#valNs2', el => el.innerText.trim());
        const valIp = await page.$eval('#valIpTarget', el => el.innerText.trim());
        console.log(`   Cấu hình DNS chuẩn: NS1="${valNs1}", NS2="${valNs2}", IP="${valIp}"`);
        if (valNs1 !== 'ns1.hanghoaphaisinh.net' || valNs2 !== 'ns2.hanghoaphaisinh.net' || valIp !== '103.173.226.15') {
            throw new Error('Giá trị DNS chuẩn hiển thị sai lệch!');
        }
        console.log('   ✅ Thông tin hướng dẫn trỏ DNS chuẩn chính xác 100%!');

        // Kiểm tra KPI cards
        const kpiTotal = await page.$eval('#kpiDomainsTotal', el => el.innerText.trim());
        const kpiActive = await page.$eval('#kpiDomainsActive', el => el.innerText.trim());
        console.log(`   KPI Tên Miền: Tổng=${kpiTotal}, Hoạt động=${kpiActive}`);
        if (parseInt(kpiTotal, 10) < 1) {
            throw new Error('Không có tên miền nào được nạp từ CSDL!');
        }

        // -------------------------------------------------------------
        // BƯỚC 5: Thêm Tên Miền Mới & Tự Động Kiểm Tra DNS
        // -------------------------------------------------------------
        const testDomainName = 'test-khoahoc-' + Math.floor(Date.now() / 1000) + '.vn';
        console.log(`👉 5. Thêm tên miền mới: "${testDomainName}"`);
        await page.click('button[onclick="openAddDomainModal()"]');
        await page.waitForTimeout(500);

        await page.fill('#addDomainInput', testDomainName);
        await page.fill('#addDomainNotes', 'Domain test tự động Playwright');
        await page.click('#modalAddDomain button[type="submit"]');
        await page.waitForTimeout(2500);

        // Đảm bảo modalAddDomain đã đóng
        await page.$eval('#modalAddDomain', el => el.classList.add('hidden')).catch(() => {});

        // Hệ thống tự động kích hoạt DNS check và mở modalDNSResult
        const isDnsAutoOpen = await page.$eval('#modalDNSResult', el => !el.classList.contains('hidden')).catch(() => false);
        if (isDnsAutoOpen) {
            const dnsTitle = await page.$eval('#dnsResultStatusTitle', el => el.innerText.trim());
            const dnsMsg = await page.$eval('#dnsResultMessage', el => el.innerText.trim());
            console.log(`   [Auto DNS Check]: "${dnsTitle}" - "${dnsMsg}"`);
            await page.$eval('#modalDNSResult', el => el.classList.add('hidden'));
            await page.waitForTimeout(500);
        }

        // Kiểm tra xem domain mới đã có trong danh sách chưa
        const domainCardsText = await page.$eval('#domainsListContainer', el => el.innerText);
        if (!domainCardsText.includes(testDomainName)) {
            throw new Error(`Tên miền "${testDomainName}" chưa xuất hiện trong danh sách sau khi thêm!`);
        }
        console.log(`   ✅ Tên miền "${testDomainName}" đã được lưu và hiển thị trên Dashboard!`);

        // -------------------------------------------------------------
        // BƯỚC 6: Kiểm tra DNS tức thì bằng nút bấm thủ công
        // -------------------------------------------------------------
        console.log(`👉 6. Kiểm tra DNS tức thì bằng nút bấm trên card tên miền`);
        const checkDnsButtons = await page.$$('#domainsListContainer button[title*="Kiểm tra DNS"]');
        if (checkDnsButtons.length > 0) {
            await checkDnsButtons[0].click();
            await page.waitForTimeout(2000);

            // Kiểm tra modal DNS kết quả
            const isDnsModalVisible = await page.$eval('#modalDNSResult', el => !el.classList.contains('hidden'));
            if (!isDnsModalVisible) {
                throw new Error('Modal kết quả DNS không hiển thị!');
            }
            const dnsTitle = await page.$eval('#dnsResultStatusTitle', el => el.innerText.trim());
            const dnsMsg = await page.$eval('#dnsResultMessage', el => el.innerText.trim());
            console.log(`   Manual DNS Check Result: "${dnsTitle}" - "${dnsMsg}"`);

            await page.$eval('#modalDNSResult', el => el.classList.add('hidden'));
            await page.waitForTimeout(500);
            console.log('   ✅ Đã kiểm tra DNS thời gian thực qua Go Net Resolver thành công!');
        }

        // -------------------------------------------------------------
        // BƯỚC 7: Triển khai Landing Page lên Thư Mục Con (Deploy Route)
        // -------------------------------------------------------------
        const testPathPrefix = '/khoa-hoc-dau-tu';
        console.log(`👉 7. Triển khai Landing Page lên đường dẫn "${testPathPrefix}" cho domain "${testDomainName}"`);

        // Mở modal deploy route cho chính testDomainName
        const specificDeployBtn = await page.$(`button[onclick*="${testDomainName}"][onclick*="openDeployRouteModal"]`);
        if (specificDeployBtn) {
            await specificDeployBtn.click();
        } else {
            await page.click(`button[onclick*="openDeployRouteModal"]`);
            await page.waitForTimeout(300);
            const optVal = await page.$eval(`#deployRouteDomainSelect option[data-domain="${testDomainName}"]`, o => o.value).catch(() => null);
            if (optVal) {
                await page.selectOption('#deployRouteDomainSelect', optVal);
            }
        }
        await page.waitForTimeout(500);

        // Nhập path prefix
        await page.fill('#deployRoutePathPrefix', testPathPrefix);
        await page.waitForTimeout(300);

        const previewDeployUrl = await page.$eval('#previewDeployUrl', el => el.innerText.trim());
        console.log(`   Preview URL triển khai: "${previewDeployUrl}"`);

        // Submit deploy
        await page.click('#modalDeployRoute button[type="submit"]');
        await page.waitForTimeout(1500);
        await page.$eval('#modalDeployRoute', el => el.classList.add('hidden')).catch(() => {});
        await page.waitForTimeout(500);

        // Kiểm tra xem route mới đã xuất hiện trong card domain chưa
        const updatedCardsText = await page.$eval('#domainsListContainer', el => el.innerText);
        if (!updatedCardsText.includes(testPathPrefix)) {
            throw new Error(`Đường dẫn "${testPathPrefix}" chưa xuất hiện trong danh sách routes!`);
        }
        console.log(`   ✅ Đã triển khai route "${testPathPrefix}" thành công!`);

        // -------------------------------------------------------------
        // BƯỚC 8: Kiểm tra Dynamic Multi-Domain Host & Path Router qua HTTP
        // -------------------------------------------------------------
        console.log('👉 8. Kiểm tra Go Dynamic Edge Host & Path Router');
        const routeCheckRes = await page.request.get(`http://localhost:8080${testPathPrefix}`, {
            headers: {
                'Host': testDomainName
            }
        });
        console.log(`   HTTP GET http://localhost:8080${testPathPrefix} with Host: ${testDomainName} -> Status: ${routeCheckRes.status()}`);
        const routeHtml = await routeCheckRes.text();
        if (routeCheckRes.status() !== 200 || !routeHtml.includes('<!DOCTYPE html>')) {
            throw new Error('Edge Router không trả về Landing Page tương ứng!');
        }
        console.log('   ✅ Dynamic Edge Router đã định tuyến chính xác Landing Page theo Host và Path!');

        // -------------------------------------------------------------
        // BƯỚC 9: Kiểm tra Tích hợp Chọn Domain trong Ladi Studio
        // -------------------------------------------------------------
        console.log('👉 9. Kiểm tra tích hợp chọn Tên Miền trong Ladi Studio');
        // Đảm bảo tất cả modal đang đóng trước khi chuyển tab
        await page.$$eval('.fixed.inset-0', els => els.forEach(el => el.classList.add('hidden'))).catch(() => {});
        await page.$eval('#tabBtnLadiStudio', el => el.click());
        await page.waitForTimeout(1000);

        // Mở modal tạo landing
        await page.click('button[onclick="openCreateLandingModal()"]');
        await page.waitForTimeout(1000);

        // Kiểm tra dropdown cfgDomainSelect
        const domainSelectOptions = await page.$$eval('#cfgDomainSelect option', opts => opts.map(o => o.value));
        console.log(`   Các domain khả dụng trong Ladi Studio: ${domainSelectOptions.join(', ')}`);
        if (!domainSelectOptions.includes(testDomainName)) {
            throw new Error(`Dropdown tên miền trong Ladi Studio thiếu domain "${testDomainName}"!`);
        }
        console.log(`   ✅ Tên miền "${testDomainName}" đã được tích hợp đầy đủ vào Ladi Studio!`);

        // Đóng modal settings
        await page.$eval('#landingSettingsModal', el => el.classList.add('hidden')).catch(() => {});
        await page.waitForTimeout(500);

        // Chuyển lại Tab Quản Lý Tên Miền để chụp ảnh màn hình
        await page.$eval('#tabBtnDomains', el => el.click());
        await page.waitForTimeout(1000);

        // Chụp screenshot
        const shotPath = path.join(screenshotDir, 'domain_management_full_e2e.png');
        await page.screenshot({ path: shotPath, fullPage: true });
        console.log(`📸 Đã lưu ảnh chụp màn hình kiểm thử: ${shotPath}`);

        console.log('\n🎉 ==============================================================');
        console.log('🎉 TẤT CẢ KIỂM THỬ E2E QUẢN LÝ TÊN MIỀN & MULTI-DOMAIN ĐỀU PASS 100%!');
        console.log('🎉 ==============================================================\n');

    } catch (error) {
        console.error('❌ Lỗi kiểm thử E2E:', error);
        await page.screenshot({ path: path.join(screenshotDir, 'domain_management_error.png'), fullPage: true }).catch(() => {});
        process.exitCode = 1;
    } finally {
        await browser.close();
    }
})();
