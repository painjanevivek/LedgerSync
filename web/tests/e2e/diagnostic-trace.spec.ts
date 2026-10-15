import { expect, test } from "@playwright/test";
import { eventDetail, mockOperatorConsole } from "./fixtures";

test("historical event explains missing trace without losing financial evidence", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.goto(`/events/${eventDetail.event_id}`);
  await page.getByText("Diagnostic trace", { exact: true }).click();
  await expect(page.getByText(/No diagnostic origin was recorded/)).toBeVisible();
  await expect(page.getByRole("link", { name: "Open private trace viewer" })).toHaveCount(0);
});

test("recorded origin remains useful when the viewer is not configured", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.route(`**/api/events/${eventDetail.event_id}`, route => route.fulfill({ json: { ...eventDetail, diagnostic_trace_id: "12345678901234567890123456789012", trace_viewer_available: false } }));
  await page.goto(`/events/${eventDetail.event_id}`);
  await page.getByText("Diagnostic trace", { exact: true }).click();
  await expect(page.getByText(/private trace viewer is not configured/)).toBeVisible();
  await expect(page.getByRole("button", { name: "Copy diagnostic trace ID" })).toBeVisible();
});

test("configured viewer opens through a same-origin record authorization check", async ({ page }) => {
  await mockOperatorConsole(page);
  await page.route(`**/api/events/${eventDetail.event_id}`, route => route.fulfill({ json: { ...eventDetail, diagnostic_trace_id: "12345678901234567890123456789012", trace_viewer_available: true } }));
  await page.goto(`/events/${eventDetail.event_id}`);
  await page.getByText("Diagnostic trace", { exact: true }).click();
  await expect(page.getByRole("link", { name: "Open private trace viewer" })).toHaveAttribute("href", `/api/events/${eventDetail.event_id}/trace`);
});
