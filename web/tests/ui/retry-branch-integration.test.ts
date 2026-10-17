import assert from "node:assert/strict";
import { test } from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { FocusedRetry } from "../../src/ui/controls/FocusedRetry.client";

test("retry controls retain a native disabled button and an accessible reason", () => {
  const disabled = renderToStaticMarkup(createElement(FocusedRetry, { label: "Retry read", onRetry: () => {}, disabled: true, disabledReason: "Restore the connection first." }));
  assert.match(disabled, /disabled=""/);
  assert.match(disabled, /aria-describedby=/);
  assert.match(disabled, /Restore the connection first\./);
  const busy = renderToStaticMarkup(createElement(FocusedRetry, { label: "Retry read", onRetry: () => {}, busy: true }));
  assert.match(busy, /disabled=""/);
  assert.match(busy, /aria-busy="true"/);
  assert.match(busy, /Wait for the current request/);
  const available = renderToStaticMarkup(createElement(FocusedRetry, { label: "Retry read", onRetry: () => {} }));
  assert.doesNotMatch(available, /disabled=""/);
  assert.doesNotMatch(available, /action-availability-reason/);
});
