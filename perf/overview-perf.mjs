// Spectra overview performance harness.
//
// Drives the real dashboard in a headless browser, logs in, navigates to the
// Fleet Overview, and measures how long the agent table takes to render. Runs
// under configurable CPU throttling to simulate weaker clients than the host,
// and repeats for statistical stability.
//
// Prereqs:
//   npm install            (installs playwright)
//   npx playwright install chromium
//   the Spectra stack running (docker compose up) with a seeded fleet
//
// Usage:
//   node overview-perf.mjs
//   BASE_URL=http://localhost:8080 SPECTRA_USER=admin SPECTRA_PASS=changeme123 \
//     CPU_THROTTLE=6 RUNS=5 node overview-perf.mjs
//
// Env:
//   BASE_URL      default http://localhost:8080
//   SPECTRA_USER / SPECTRA_PASS   dashboard credentials (default admin / changeme123)
//   CPU_THROTTLE  CDP CPU slowdown multiplier, 1 = none (default 4)
//   RUNS          measured iterations (default 5)
//   VIEW          "table" or "cards" (default table)
//   HEADED        "1" to watch the browser (default headless)

import { chromium } from "playwright";

const BASE_URL = process.env.BASE_URL ?? "http://localhost:8080";
const USER = process.env.SPECTRA_USER ?? "admin";
const PASS = process.env.SPECTRA_PASS ?? "changeme123";
const CPU_THROTTLE = Number(process.env.CPU_THROTTLE ?? 4);
const RUNS = Number(process.env.RUNS ?? 5);
const VIEW = process.env.VIEW ?? "table";
const HEADED = process.env.HEADED === "1";

function stats(xs) {
  const s = [...xs].sort((a, b) => a - b);
  const sum = s.reduce((a, b) => a + b, 0);
  const p = (q) => s[Math.min(s.length - 1, Math.floor(q * s.length))];
  return {
    n: s.length,
    min: s[0],
    max: s[s.length - 1],
    mean: sum / s.length,
    median: p(0.5),
    p95: p(0.95),
  };
}

async function login(page) {
  await page.goto(BASE_URL, { waitUntil: "networkidle" });
  // The login form has username/password inputs and a submit. Prefer accessible
  // selectors; fall back to input types if labels differ.
  const userField = page.locator(
    'input[name="username"], input[type="text"]'
  ).first();
  const passField = page.locator('input[type="password"]').first();
  await userField.fill(USER);
  await passField.fill(PASS);
  // Submit by pressing Enter in the password field (works regardless of button text).
  await passField.press("Enter");
  // Wait until we're past the login screen: the Fleet Overview heading appears.
  await page.getByText("Fleet Overview", { exact: false }).first().waitFor({
    timeout: 15000,
  });
}

async function measureOverviewRender(page) {
  // Force a fresh navigation to the overview so we measure a cold render, not a
  // cached one. Mark the start, navigate, wait for the agent rows to be present,
  // then read elapsed time from the page's own clock.
  await page.evaluate(() => performance.mark("nav-start"));

  // If not already on overview, click the nav item.
  const navItem = page.getByText("Fleet Overview", { exact: false }).first();
  await navItem.click().catch(() => {});

  // Wait for the table/cards to have actual agent rows. "seed-" hostnames are a
  // reliable signal the fleet has rendered. Adjust the selector if your row
  // markup differs.
  const start = performance.now();
  await page.locator('text=/seed-/').first().waitFor({ timeout: 30000 });
  // Give layout a beat to settle, then mark end.
  await page.evaluate(() => {
    performance.mark("nav-end");
    performance.measure("overview-render", "nav-start", "nav-end");
  });
  const elapsed = performance.now() - start;
  return elapsed;
}

async function main() {
  console.log(
    `Spectra overview perf — ${BASE_URL} | CPU x${CPU_THROTTLE} | ${RUNS} runs | view=${VIEW}`
  );

  const browser = await chromium.launch({ headless: !HEADED });
  const context = await browser.newContext();
  const page = await context.newPage();

  // Apply CPU throttling via the Chrome DevTools Protocol.
  const client = await context.newCDPSession(page);
  if (CPU_THROTTLE > 1) {
    await client.send("Emulation.setCPUThrottlingRate", { rate: CPU_THROTTLE });
  }

  await login(page);

  // Optionally switch view.
  if (VIEW === "cards") {
    await page.getByText("Cards", { exact: false }).first().click().catch(() => {});
  }

  // Warm-up run (not counted) to prime any first-load costs.
  await measureOverviewRender(page).catch(() => {});

  const times = [];
  for (let i = 0; i < RUNS; i++) {
    // Reload to force a cold render each iteration.
    await page.reload({ waitUntil: "domcontentloaded" });
    if (VIEW === "cards") {
      await page.getByText("Cards", { exact: false }).first().click().catch(() => {});
    }
    const t = await measureOverviewRender(page);
    times.push(t);
    console.log(`  run ${i + 1}: ${t.toFixed(1)} ms`);
  }

  const s = stats(times);
  console.log("\nResults (ms):");
  console.log(
    `  n=${s.n}  min=${s.min.toFixed(1)}  median=${s.median.toFixed(1)}  mean=${s.mean.toFixed(1)}  p95=${s.p95.toFixed(1)}  max=${s.max.toFixed(1)}`
  );

  await browser.close();
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
