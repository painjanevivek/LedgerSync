import { expect, test } from "@playwright/test";
import { mockOperatorConsole } from "./fixtures";

test("core default hides Systems Lab and rejects direct optional BFF calls", async ({ page }) => {
  await mockOperatorConsole(page); await page.goto("/developer");
  await expect(page.getByRole("heading", { name: /Developer/ }).first()).toBeVisible();
  await expect(page.locator("#developer-distributed-systems")).toHaveCount(0);
  for (const path of ["/api/developer/distributed", "/api/developer/distributed/evidence"]) {
    const response = await page.request.get(path); expect(response.status()).toBe(404);
  }
  const response = await page.request.post("/api/investigation/live/rooms", { data: {} }); expect(response.status()).toBe(404);
  await page.goto("/investigations/live#invite=" + "A".repeat(43));
  await expect(page.getByRole("heading", { name: "Live collaboration is disabled" })).toBeVisible();
  await expect(page).not.toHaveURL(/invite=/);
});

test("old room links show stored findings without signals, presence or writes", async ({ page }) => {
  const id = "12345678-1234-4234-8234-123456789012";
  await mockOperatorConsole(page);
  await page.route(`**/api/investigation/live/rooms/${id}`, route => route.fulfill({ contentType: "application/json", body: JSON.stringify({ room_id: id, investigation_id: "12345678-1234-4234-8234-123456789013", request_reference: "12345678-1234-4234-8234-123456789014", status: "active", participant_role: "owner", peer_bound: true, peer_present: false, version: "3", created_at: "2026-09-08T10:00:00Z", expires_at: "2026-09-08T10:30:00Z", authoritative_request_state: "expired", finding: { code: "still_unresolved", authoritative_request_state: "in_progress", recorded_at: "2026-09-08T10:05:00Z" } }) }));
  const active: string[] = [];
  page.on("request", request => { if (request.url().includes("/api/investigation/live/") && (request.method() !== "GET" || /\/(signals|presence)/.test(request.url()))) active.push(request.url()); });
  await page.goto(`/investigations/live/${id}`);
  await expect(page.getByRole("heading", { name: "Retained room history" })).toBeVisible();
  await expect(page.getByText(/still unresolved/)).toBeVisible();
  await expect(page.getByRole("link", { name: "Open transfer recovery" })).toHaveAttribute("href", "/transfers");
  await expect(page.getByRole("button", { name: /join|record finding|start room/i })).toHaveCount(0);
  expect(active).toEqual([]);
});
