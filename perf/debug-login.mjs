// One-off: see what the login page actually looks like and whether login works.
import { chromium } from "playwright";

const BASE_URL = process.env.BASE_URL ?? "http://localhost:8080";
const USER = process.env.USER ?? "admin";
const PASS = process.env.PASS ?? "changeme123";

const browser = await chromium.launch();
const page = await browser.newPage();

page.on("console", (m) => console.log("  [browser]", m.text()));

await page.goto(BASE_URL, { waitUntil: "networkidle" });

// Dump every input on the page so we know the real selectors.
const inputs = await page.locator("input").all();
console.log(`Found ${inputs.length} input(s):`);
for (const inp of inputs) {
  const type = await inp.getAttribute("type");
  const name = await inp.getAttribute("name");
  const ph = await inp.getAttribute("placeholder");
  console.log(`  type=${type} name=${name} placeholder=${ph}`);
}

// Dump buttons.
const buttons = await page.locator("button").all();
console.log(`Found ${buttons.length} button(s):`);
for (const b of buttons) {
  console.log(`  "${(await b.textContent())?.trim()}"`);
}

// Try the login.
await page.locator('input[type="text"], input[name="username"]').first().fill(USER);
await page.locator('input[type="password"]').first().fill(PASS);
await page.locator('input[type="password"]').first().press("Enter");

// Wait a moment, then report what's on screen.
await page.waitForTimeout(3000);
console.log("\n--- After login attempt ---");
console.log("URL:", page.url());
const bodyText = (await page.locator("body").textContent())?.slice(0, 400);
console.log("Body text (first 400 chars):", bodyText?.replace(/\s+/g, " ").trim());

await page.screenshot({ path: "login-debug.png", fullPage: true });
console.log("\nScreenshot saved to login-debug.png");

await browser.close();
