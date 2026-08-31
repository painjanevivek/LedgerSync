import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { createElement, type ComponentProps } from "react";
import { renderToStaticMarkup } from "react-dom/server";

import { Button } from "../../src/ui/controls/Button";
import { DataTableRegion } from "../../src/ui/display/DataTableRegion";
import { Identifier } from "../../src/ui/display/Identifier";
import { Money } from "../../src/ui/display/Money";
import { StatePanel } from "../../src/ui/display/StatePanel";
import { Timestamp } from "../../src/ui/display/Timestamp";

test("static display primitives remain outside client component boundaries", () => {
  for (const modulePath of [
    "../../src/ui/display/PageHeader.tsx",
    "../../src/ui/display/StatusBadge.tsx",
    "../../src/ui/display/StatePanel.tsx",
    "../../src/ui/display/DataTableRegion.tsx",
    "../../src/ui/display/RecordLink.tsx",
    "../../src/ui/display/Evidence.tsx",
    "../../src/ui/display/Money.tsx",
    "../../src/ui/display/Identifier.tsx",
    "../../src/ui/display/Timestamp.tsx",
  ]) {
    const source = readFileSync(new URL(modulePath, import.meta.url), "utf8");
    assert.doesNotMatch(source, /^\s*["']use client["']/m, modulePath);
  }
});

test("browser-dependent controls declare explicit leaf client boundaries", () => {
  for (const modulePath of [
    "../../src/ui/controls/CopyControl.client.tsx",
    "../../src/ui/controls/FocusedRetry.client.tsx",
    "../../src/ui/controls/Pagination.client.tsx",
    "../../src/ui/forms/FormField.client.tsx",
    "../../src/ui/overlays/ConfirmationDialog.client.tsx",
  ]) {
    const source = readFileSync(new URL(modulePath, import.meta.url), "utf8");
    assert.match(source, /^\s*["']use client["']/m, modulePath);
  }
});

test("money, identifiers and timestamps preserve exact machine-readable evidence", () => {
  const money = renderToStaticMarkup(createElement(Money, { currency: "inr", minorUnits: "125000" }));
  assert.match(money, /data-currency="INR"/);
  assert.match(money, /data-minor-units="125000"/);
  assert.match(money, />INR 1250\.00</);

  const identifier = renderToStaticMarkup(createElement(Identifier, { value: "33333333-3333-4333-8333-333333333333" }));
  assert.match(identifier, /title="33333333-3333-4333-8333-333333333333"/);
  assert.match(identifier, />33333333-3333-4333-8333-333333333333</);

  const timestamp = renderToStaticMarkup(createElement(Timestamp, { value: "2026-08-19T12:00:00Z" }));
  assert.match(timestamp, /dateTime="2026-08-19T12:00:00\.000Z"/);
  assert.match(timestamp, /UTC/);
});

test("state panels announce only errors or explicitly dynamic changes", () => {
  const staticPanel = renderToStaticMarkup(createElement(StatePanel, { title: "No records", message: "The bounded page is empty." }));
  assert.doesNotMatch(staticPanel, /role="status"/);
  assert.doesNotMatch(staticPanel, /aria-live=/);

  const politePanel = renderToStaticMarkup(createElement(StatePanel, { title: "Outcome retained", message: "Retry uses the same key.", kind: "unknown", announce: "polite" }));
  assert.match(politePanel, /role="status"/);
  assert.match(politePanel, /aria-live="polite"/);

  const errorPanel = renderToStaticMarkup(createElement(StatePanel, { title: "Evidence unavailable", message: "No result is inferred.", kind: "error" }));
  assert.match(errorPanel, /role="alert"/);
  assert.match(errorPanel, /aria-live="assertive"/);
});

test("table regions and buttons retain native accessible semantics", () => {
  const tableProps: ComponentProps<typeof DataTableRegion> = {
    label: "Authorized transfers",
    caption: createElement("p", null, "Immutable transfer evidence"),
    resultSummary: createElement("p", null, "2 records on this page"),
    sortDescription: createElement("p", null, "Newest completed first"),
    children: createElement("table", null, createElement("tbody")),
  };
  const table = renderToStaticMarkup(createElement(DataTableRegion, tableProps));
  assert.match(table, /role="region"/);
  assert.match(table, /aria-label="Authorized transfers"/);
  assert.match(table, /Immutable transfer evidence/);
  assert.match(table, /Newest completed first/);

  const button = renderToStaticMarkup(createElement(Button, { variant: "primary", guarded: true, busy: true, busyLabel: "Posting…" }, "Post"));
  assert.match(button, /^<button/);
  assert.match(button, /class="button primary guarded-control"/);
  assert.match(button, /disabled=""/);
  assert.match(button, /aria-busy="true"/);
  assert.match(button, />Posting…</);
});
