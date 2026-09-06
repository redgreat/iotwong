#!/usr/bin/env node
/**
 * Headless-browser UI E2E for the iotwong workbench (T05 gate):
 *   login -> device list (real DB via API) -> select local device ->
 *   track query -> map canvas rendered -> mobile no overflow.
 * Requires: dev server with API proxy (vite dev), SEED_PASSWORD,
 * and a Chromium binary (PUPPETEER_EXEC_PATH or /usr/bin/chromium).
 */
import puppeteer from 'puppeteer-core';

const APP_URL = process.env.APP_URL || 'http://127.0.0.1:5273';
const LOGIN = process.env.SEED_LOGIN || 'admin';
const PASSWORD = process.env.SEED_PASSWORD || '';
const EXEC = process.env.PUPPETEER_EXEC_PATH || '/usr/bin/chromium';

if (!PASSWORD) { console.error('E2E: SEED_PASSWORD is required'); process.exit(2); }

async function main() {
	const browser = await puppeteer.launch({
		executablePath: EXEC, headless: true,
		args: ['--no-sandbox', '--disable-dev-shm-usage', '--disable-gpu']
	});
	const page = await browser.newPage();
	await page.setViewport({ width: 1440, height: 900 });
	const fails = [];
	const ok = (n) => console.log('UI-E2E PASS:', n);
	const bad = (n, d = '') => { fails.push(n); console.error('UI-E2E FAIL:', n, d); };

	try {
		await page.goto(APP_URL, { waitUntil: 'networkidle2', timeout: 20000 });
		await page.waitForFunction(() => document.body.innerText.includes('登录 iotwong'), { timeout: 15000 });
		ok('login page rendered');

		await page.type('input[placeholder="登录名"]', LOGIN);
		await page.type('input[placeholder="密码"]', 'definitely-wrong');
		await page.click('button[type="submit"]');
		await page.waitForFunction(() => document.body.innerText.includes('用户名或密码错误'), { timeout: 15000 });
		ok('wrong password rejected uniformly');

		const pw = await page.$('input[placeholder="密码"]');
		await pw.click();
		await page.keyboard.down('Control');
		await page.keyboard.press('KeyA');
		await page.keyboard.up('Control');
		await page.keyboard.press('Backspace');
		await pw.type(PASSWORD);
		await page.click('button[type="submit"]');
		await page.waitForFunction(() => document.body.innerText.includes('本地演示设备'), { timeout: 15000 });
		ok('login + device list from real DB (dev-1)');

		const text = await page.evaluate(() => document.body.innerText);
		if (text.includes('历史回放')) ok('racebox historical tag'); else bad('racebox tag');
		if (/定位 [0-9]/.test(text) || text.includes('无定位')) ok('position states'); else bad('position states');

		// 角色驱动菜单（admin）：设备地图/轨迹/围栏管理/报警/设备改名
		const menuLabels = await page.evaluate(() =>
			[...document.querySelectorAll('nav[aria-label="功能菜单"] button')].map((b) => b.innerText).join('|')
		);
		for (const need of ['设备地图', '设备管理', '围栏管理', '报警', '设备改名']) {
			if (menuLabels.includes(need)) ok(`admin menu contains ${need}`); else bad('admin menu missing', need);
		}
		// 切到报警视图：空态或列表都能正常渲染
		await page.evaluate(() => {
			const el = [...document.querySelectorAll('nav[aria-label="功能菜单"] button')].find((b) => b.innerText.includes('报警'));
			if (el) el.click();
		});
		await page.waitForFunction(() => {
			const s = document.querySelector('[aria-label="报警列表"]');
			if (!s) return false;
			const t = s.textContent || '';
			return (
				!t.includes('加载报警中') &&
				(t.includes('还没有报警记录') || t.includes('未确认') || t.includes('已确认'))
			);
		}, { timeout: 15000 });
		ok('alarms view settled (empty or list rendered)');
		// 切回设备地图
		await page.evaluate(() => {
			const el = [...document.querySelectorAll('nav[aria-label="功能菜单"] button')].find((b) => b.innerText.includes('设备地图'));
			if (el) el.click();
		});
		await page.waitForFunction(() => !!document.querySelector('section[aria-label="设备列表"]'), { timeout: 15000 });
		ok('back to device map view');

		await page.evaluate(() => {
			const el = [...document.querySelectorAll('button')].find((b) => b.innerText.includes('本地演示设备'));
			if (el) el.click();
		});
		await page.waitForFunction(() => document.body.innerText.includes('查询轨迹'), { timeout: 15000 });
		await page.evaluate(() => {
			const el = [...document.querySelectorAll('button')].find((b) => b.innerText.includes('查询轨迹'));
			if (el) el.click();
		});
		await page.waitForFunction(
			() => /原始点 \d+/.test(document.body.innerText) || document.body.innerText.includes('没有轨迹数据') || document.body.innerText.includes('该时间窗内没有轨迹数据'),
			{ timeout: 20000 }
		);
		const t = await page.evaluate(() => document.body.innerText);
		if (/原始点 \d+/.test(t)) ok('track returned points from DB');
		else ok('track executed (empty-state shown)');
		const hasPlay = await page.evaluate(() => [...document.querySelectorAll('button')].some((b) => b.innerText.includes('回放')));
		if (hasPlay) {
			ok('playback controls rendered');
			await page.evaluate(() => {
				const el = [...document.querySelectorAll('button')].find((b) => b.innerText.includes('回放'));
				if (el) el.click();
			});
			await new Promise((r) => setTimeout(r, 350));
			const t2 = await page.evaluate(() => document.body.innerText);
			if (/\d+\/\d+/.test(t2)) ok('playback progress counter advancing');
			else bad('playback counter');
		} else {
			ok('playback hidden (no points / empty window)');
		}

		const hasCanvas = await page.evaluate(() => !!document.querySelector('canvas'));
		if (hasCanvas) ok('maplibre canvas rendered'); else bad('map canvas');

		await page.setViewport({ width: 390, height: 844 });
		await new Promise((r) => setTimeout(r, 700));
		const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
		if (overflow <= 1) ok('mobile 390x844 no horizontal overflow');
		else bad('mobile overflow', `delta=${overflow}`);
	} catch (e) {
		bad('harness error', String(e));
		try { await page.screenshot({ path: '/tmp/iotwong-ui-e2e.png' }); } catch {}
	} finally {
		await browser.close();
	}
	if (fails.length) { console.error(`UI-E2E: ${fails.length} failure(s)`); process.exit(1); }
	console.log('UI-E2E: all checks passed');
}
main().catch((e) => { console.error(e); process.exit(1); });
