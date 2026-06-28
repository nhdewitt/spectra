// Spectra render benchmark — measures pure render time (data-arrival -> rows
// painted) across CPU throttle levels, excluding the polling floor.
//
// Unlike overview-perf.mjs (which timed reload-to-rows and so caught the 10s
// poll), this anchors t0 on the /overview response and t1 on rows being in the
// DOM, so the number is React turning data into DOM -- the actual render cost.
//
// It does NOT reseed: point it at one running stack (a given fleet size) and it
// sweeps throttle levels. Run it once per fleet size:
//
//   docker compose down -v && SEED_N=500  docker compose up -d --build
//   until curl -sf http://localhost:8080 >/dev/null; do sleep 1; done
//   node render-bench.mjs
//   # then repeat for 2000, 5000
//
// Env:
//   BASE_URL       default http://localhost:8080
//   SPECTRA_USER / SPECTRA_PASS   default admin / changeme123
//   THROTTLES      comma list of CPU multipliers (default "1,2,4,6")
//   RUNS           measured iterations per throttle (default 5)
//   VIEW           "table" or "cards" (default table)

import { chromium } from "playwright";

const BASE_URL = process.env.BASE_URL ?? "http://localhost:8080";
const USER = process.env.SPECTRA_USER ?? "admin";
const PASS = process.env.SPECTRA_PASS ?? "changeme123";
const THROTTLES = (process.env.THROTTLES ?? "1,2,4,6")
	.split(",")
	.map((s) => Number(s.trim()))
	.filter((n) => n > 0);
const RUNS = Number(process.env.RUNS ?? 5);
const VIEW = process.env.VIEW ?? "table";

function stats(xs) {
	const s = [...xs].sort((a, b) => a - b);
	const sum = s.reduce((a, b) => a + b, 0);
	const p = (q) => s[Math.min(s.length - 1, Math.floor(q * s.length))];
	return { min: s[0], max: s[s.length - 1], mean: sum / s.length, median: p(0.5), p95: p(0.95) };
}

async function login(page) {
	await page.goto(BASE_URL, { waitUntil: "networkidle" });
	await page.locator('input[type="text"], input[name="username"]').first().fill(USER);
	const pass = page.locator('input[type="password"]').first();
	await pass.fill(PASS);
	await pass.press("Enter");
	await page.getByText("Fleet Overview", { exact: false }).first().waitFor({ timeout: 15000 });
}

// rowSelector for the active view: table rows vs card cells.
function rowSelector(view) {
	return view === "cards"
		? 'div[style*="grid-template-columns"] > div'
		: "tbody tr";
}

// measureRender forces a fresh fetch+render by navigating away from the overview
// and back (remount without a full page reload, so the bundle stays warm), then
// times from the /overview response arriving to rows being present in the DOM.
async function measureRender(page, view) {
	// Navigate away (Agent Mgmt is always in the sidebar).
	await page.getByText("Agent Mgmt", { exact: false }).first().click();
	await page.waitForTimeout(100);

	// Arm the response wait BEFORE navigating back, so we don't miss it.
	const respPromise = page.waitForResponse(
		(r) => r.url().includes("/api/v1/overview") && r.status() === 200,
		{ timeout: 30000 }
	);

	await page.getByText("Fleet Overview", { exact: false }).first().click();
	await respPromise;                       // data arrived
	const t0 = await page.evaluate(() => performance.now());

	// Wait until at least one row is painted, then let the batch settle.
	await page.locator(rowSelector(view)).first().waitFor({ timeout: 30000 });
	const t1 = await page.evaluate(() => performance.now());

	return t1 - t0;
}

async function main() {
	const browser = await chromium.launch();
	const context = await browser.newContext();
	const page = await context.newPage();
	const client = await context.newCDPSession(page);

	await login(page);
	if (VIEW === "cards") {
		await page.getByText("Cards", { exact: false }).first().click().catch(() => {});
	}

	// Report the fleet size from the StatBar "Total Agents" if we can read it.
	let fleet = "?";
	try {
		fleet = await page.evaluate(() => {
			const els = [...document.querySelectorAll("div")];
			const idx = els.findIndex((e) => e.textContent?.trim() === "Total Agents");
			return idx > 0 ? els[idx - 1]?.textContent?.trim() ?? "?" : "?";
		});
	} catch {}

	console.log(`Render benchmark — ${BASE_URL} | fleet=${fleet} | view=${VIEW} | runs=${RUNS}`);
	console.log("(render = /overview response -> rows painted, excludes poll)\n");
	console.log("throttle |  median |    mean |     p95 |     min |     max   (ms)");
	console.log("---------+---------+---------+---------+---------+--------");

	for (const rate of THROTTLES) {
		await client.send("Emulation.setCPUThrottlingRate", { rate });

		// Warm-up (not counted).
		await measureRender(page, VIEW).catch(() => {});

		const times = [];
		for (let i = 0; i < RUNS; i++) {
			times.push(await measureRender(page, VIEW));
		}
		const s = stats(times);
		console.log(
			`  ${String(rate).padStart(2)}x    | ` +
			`${s.median.toFixed(0).padStart(6)} | ` +
			`${s.mean.toFixed(0).padStart(6)} | ` +
			`${s.p95.toFixed(0).padStart(6)} | ` +
			`${s.min.toFixed(0).padStart(6)} | ` +
			`${s.max.toFixed(0).padStart(6)}`
		);
	}

	await client.send("Emulation.setCPUThrottlingRate", { rate: 1 });
	await browser.close();
}

main().catch((err) => {
	console.error(err);
	process.exit(1);
});
