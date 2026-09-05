import { expect, test } from "@playwright/test";

for (const path of ["/", "/welcome"]) {
  test(`${path} introduces the product without reading workspace data`, async ({ page }) => {
    const protectedReads: string[] = [];
    page.on("request", request => {
      const url = new URL(request.url());
      if (url.pathname.startsWith("/api/") && url.pathname !== "/api/session") protectedReads.push(url.pathname);
    });
    await page.route("**/api/session", route => route.fulfill({ status: 401, body: "{}" }));
    await page.goto(path);
    await expect(page.getByRole("heading", { name: /Your money workflows/ })).toBeVisible();
    await expect(page.getByText("Illustrative example · No money moves.")).toBeVisible();
    await expect(page.getByRole("link", { name: "Open workspace", exact: true }).first()).toHaveAttribute("href", "/sign-in");
    expect(protectedReads).toEqual([]);
    for (const width of [320, 360, 390, 768, 1024, 1280, 1440]) {
      await page.setViewportSize({ width, height: 900 });
      expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
    }
    if (path === "/welcome") {
      await page.screenshot({ path: "../docs/design/qa/guided-landing-desktop.png", fullPage: true, animations: "disabled" });
      await page.setViewportSize({ width: 390, height: 844 });
      await page.screenshot({ path: "../docs/design/qa/guided-landing-mobile.png", fullPage: true, animations: "disabled" });
    }
  });
}

test("welcome has working sections and is independent of session outages", async ({ page }) => {
  const apiCalls: string[] = [];
  page.on("request", request => { if (new URL(request.url()).pathname.startsWith("/api/")) apiCalls.push(request.url()); });
  await page.goto("/welcome");
  await page.getByRole("link", { name: "See how it works" }).click();
  await expect(page).toHaveURL(/#how-it-works$/);
  await page.getByText("What happens if a transfer is still being confirmed?", { exact: true }).click();
  await expect(page.getByText(/Do not create another transfer/)).toBeVisible();
  expect(apiCalls).toEqual([]);
});

test("root session outage is explicit and cannot mount financial reads", async ({ page }) => {
  const protectedReads: string[] = [];
  page.on("request", request => { const path = new URL(request.url()).pathname; if (path.startsWith("/api/") && path !== "/api/session") protectedReads.push(path); });
  await page.route("**/api/session", route => route.fulfill({ status: 503, json: { error: { code: "temporary_unavailable" } } }));
  const response = await page.goto("/");
  expect(response?.headers()["cache-control"]).toContain("no-store");
  await expect(page.getByRole("alert").filter({ hasText: "We couldn’t check your session" })).toBeVisible();
  await page.getByRole("button", { name: "Try again", exact: true }).click();
  await expect(page.getByRole("heading", { name: /Your money workflows/ })).toBeVisible();
  expect(protectedReads).toEqual([]);
});
