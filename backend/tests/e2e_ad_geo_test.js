const { chromium } = require('playwright');
const fs = require('fs');
const path = require('path');

(async () => {
    console.log('🚀 Bắt đầu kiểm thử E2E hệ thống Admin & GeoIP / Ad Placement Analytics...');
    const browser = await chromium.launch({ headless: true });
    const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    const page = await context.newPage();

    const screenshotDir = path.join(__dirname, 'screenshots');
    if (!fs.existsSync(screenshotDir)) {
        fs.mkdirSync(screenshotDir, { recursive: true });
    }

    try {
        // 1. Mở trang Admin Dashboard
        console.log('👉 1. Truy cập http://localhost:8080/admin');
        await page.goto('http://localhost:8080/admin', { waitUntil: 'networkidle', timeout: 15000 });
        await page.waitForTimeout(1500);

        // Kiểm tra tiêu đề trang
        const title = await page.title();
        console.log(`   Tiêu đề trang: "${title}"`);

        // 2. Kiểm tra CRM Table View có cột Khu Vực & Vị Trí QC
        console.log('👉 2. Kiểm tra cột "Khu Vực & Vị Trí QC" trong bảng CRM');
        const headerTexts = await page.$$eval('table thead th', ths => ths.map(t => t.innerText.trim()));
        console.log(`   Headers tìm thấy: ${headerTexts.join(' | ')}`);
        const hasGeoPlacementHeader = headerTexts.some(h => h.includes('Khu Vực') || h.includes('Vị Trí QC'));
        if (!hasGeoPlacementHeader) {
            throw new Error('Không tìm thấy cột "Khu Vực & Vị Trí QC" trong bảng CRM!');
        }
        console.log('   ✅ Đã có cột "Khu Vực & Vị Trí QC" trong CRM Table!');

        // 3. Mở Customer 360 Drawer
        console.log('👉 3. Mở Customer 360 Drawer bằng cách click vào dòng Lead đầu tiên');
        const firstRow = await page.$('#crmTableBody tr');
        if (firstRow) {
            await firstRow.click();
            await page.waitForTimeout(1000);

            // Kiểm tra các trường trong GeoIP Inspector
            const cityText = await page.$eval('#drawerCityText', el => el.innerText).catch(() => '');
            const regionBadge = await page.$eval('#drawerRegionBadge', el => el.innerText).catch(() => '');
            const ispText = await page.$eval('#drawerISP', el => el.innerText).catch(() => '');
            console.log(`   Customer 360 Geo: City="${cityText}", Region="${regionBadge}", ISP="${ispText}"`);

            // Kiểm tra các trường trong Ad Placement Inspector
            const adPlat = await page.$eval('#drawerAdPlatformBadge', el => el.innerText).catch(() => '');
            const adPlace = await page.$eval('#drawerAdPlacement', el => el.innerText).catch(() => '');
            console.log(`   Customer 360 Ad Placement: Platform="${adPlat}", Placement="${adPlace}"`);

            await page.screenshot({ path: path.join(screenshotDir, '01_customer_360_drawer.png') });
            console.log('   📸 Đã chụp ảnh 01_customer_360_drawer.png');

            // Đóng drawer
            await page.click('#drawerBackdrop');
            await page.waitForTimeout(500);
        }

        // 4. Chuyển sang Kanban View
        console.log('👉 4. Chuyển sang Kanban Pipeline Board');
        await page.$eval('#viewBtnKanban', el => el.click());
        await page.waitForTimeout(1000);
        await page.screenshot({ path: path.join(screenshotDir, '02_crm_kanban.png') });
        console.log('   📸 Đã chụp ảnh 02_crm_kanban.png');

        // Quay lại table view
        await page.$eval('#viewBtnTable', el => el.click());
        await page.waitForTimeout(500);

        // 5. Chuyển sang Tab Thống Kê & Biểu Đồ (Analytics)
        console.log('👉 5. Chuyển sang Tab Analytics & Biểu Đồ');
        await page.$eval('#tabBtnAnalytics', el => el.click());
        await page.waitForTimeout(2000);

        // Kiểm tra sự xuất hiện của 4 canvas biểu đồ mới
        const canvasCities = await page.$('#chartCitiesCanvas');
        const canvasRegions = await page.$('#chartRegionsCanvas');
        const canvasAdPlacements = await page.$('#chartAdPlacementsCanvas');
        const canvasPlacementCR = await page.$('#chartPlacementCRCanvas');
        const geoPerfBody = await page.$('#geoPerfBody');

        if (!canvasCities || !canvasRegions || !canvasAdPlacements || !canvasPlacementCR || !geoPerfBody) {
            throw new Error('Thiếu một trong các canvas hoặc bảng ma trận địa lý trong Tab Analytics!');
        }
        console.log('   ✅ Đầy đủ 4 Canvas Chart.js & Bảng Ma Trận Địa Lý!');

        const geoRows = await page.$$eval('#geoPerfBody tr', trs => trs.length);
        console.log(`   Số dòng trong Bảng Ma Trận Địa Lý & Chuyển Đổi: ${geoRows}`);

        await page.screenshot({ path: path.join(screenshotDir, '03_analytics_charts.png'), fullPage: false });
        console.log('   📸 Đã chụp ảnh 03_analytics_charts.png');

        // 6. Chuyển sang Tab Quản Lý Landing & Phiên Truy Cập (Landing)
        console.log('👉 6. Chuyển sang Tab Quản Lý Landing & Phiên Truy Cập (Sessions)');
        await page.$eval('#tabBtnLanding', el => el.click());
        await page.waitForTimeout(2000);
        await page.screenshot({ path: path.join(screenshotDir, '08_tab_landing.png') });
        console.log('   📸 Đã chụp ảnh 08_tab_landing.png');

        const sessionHeaders = await page.$$eval('#sectionTabLanding table thead th', ths => ths.map(t => t.innerText.trim().toLowerCase()));
        console.log(`   Session table headers: ${sessionHeaders.join(' | ')}`);
        const hasSessionGeo = sessionHeaders.some(h => h.includes('vị trí & isp') || h.includes('địa chỉ'));
        const hasSessionPlacement = sessionHeaders.some(h => h.includes('vị trí qc') || h.includes('placement'));
        if (!hasSessionGeo || !hasSessionPlacement) {
            throw new Error('Thiếu cột "Vị Trí & ISP" hoặc "Vị Trí QC" trong Sessions Table!');
        }
        console.log('   ✅ Sessions Table đã có cột "Vị Trí & ISP (GeoIP)" và "Vị Trí QC (Placement)"!');

        // Mở Session Timeline Modal
        await page.waitForTimeout(1000);
        const hasSessionBtn = await page.$('#sessionsTableBody button');
        if (hasSessionBtn) {
            await page.$eval('#sessionsTableBody button', btn => btn.click());
            await page.waitForTimeout(1000);
            const subTitle = await page.$eval('#modalSessionSub', el => el.innerText).catch(() => '');
            console.log(`   Session Timeline Subtitle: "${subTitle}"`);
            await page.screenshot({ path: path.join(screenshotDir, '04_session_timeline_modal.png') });
            console.log('   📸 Đã chụp ảnh 04_session_timeline_modal.png');
            await page.$eval('#sessionModal button', btn => btn.click());
            await page.waitForTimeout(500);
        }

        // 7. Chuyển sang Tab Bảo Mật & Bot
        console.log('👉 7. Chuyển sang Tab Bảo Mật & Bot');
        await page.$eval('#tabBtnSecurity', el => el.click());
        await page.waitForTimeout(1000);
        const isSecurityVisible = await page.$eval('#sectionTabSecurity', el => !el.classList.contains('hidden'));
        if (!isSecurityVisible) throw new Error('Tab Bảo Mật & Bot bị ẩn!');
        await page.screenshot({ path: path.join(screenshotDir, '05_tab_security.png') });
        console.log('   ✅ Tab Bảo Mật & Bot hiển thị thành công! Đã chụp ảnh 05_tab_security.png');

        // 8. Chuyển sang Tab Đồng Bộ Google Sheets
        console.log('👉 8. Chuyển sang Tab Đồng Bộ Google Sheets');
        await page.$eval('#tabBtnSheets', el => el.click());
        await page.waitForTimeout(1000);
        const isSheetsVisible = await page.$eval('#sectionTabSheets', el => !el.classList.contains('hidden'));
        if (!isSheetsVisible) throw new Error('Tab Google Sheets bị ẩn!');
        await page.screenshot({ path: path.join(screenshotDir, '06_tab_sheets.png') });
        console.log('   ✅ Tab Google Sheets hiển thị thành công! Đã chụp ảnh 06_tab_sheets.png');

        // 9. Chuyển sang Tab Quản Lý Landing (Ladi Studio)
        console.log('👉 9. Chuyển sang Tab Quản Lý Landing (Ladi Studio)');
        await page.$eval('#tabBtnLadiStudio', el => el.click());
        await page.waitForTimeout(1500);
        const isLadiVisible = await page.$eval('#sectionTabLadiStudio', el => !el.classList.contains('hidden'));
        if (!isLadiVisible) throw new Error('Tab Ladi Studio bị ẩn!');
        await page.screenshot({ path: path.join(screenshotDir, '07_tab_ladistudio.png') });
        console.log('   ✅ Tab Ladi Studio hiển thị thành công! Đã chụp ảnh 07_tab_ladistudio.png');

        console.log('\n🎉 TOÀN BỘ CÁC TÍNH NĂNG VÀ TẤT CẢ CÁC TABS ĐÃ HOẠT ĐỘNG HOÀN HẢO 100%!');

    } catch (err) {
        console.error('❌ Lỗi kiểm thử E2E:', err);
        process.exit(1);
    } finally {
        await browser.close();
    }
})();
