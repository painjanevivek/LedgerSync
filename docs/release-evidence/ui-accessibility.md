# Phase 9 — responsive and accessibility hardening

Automated Chromium evidence passes for:

- 390×844, 768×1024, 1024×768, 1366×768, 1440×900, and 1920×1080 core journeys;
- targeted 320 px minimum-width reflow with increased letter, word, and line spacing;
- no page-level horizontal overflow;
- compact drawer focus entry, Escape close, and trigger focus restoration;
- skip link, semantic landmarks/headings/tables, labelled form fields, and icon control names;
- live copy announcement, validation/error status, and transfer outcome status;
- reduced motion and forced-colors operability;
- automated axe analysis on the primary console and evidence route.

Physical iOS, Android, and tablet checks cannot be truthfully inferred from emulation. They remain explicitly pending in `ui-device-matrix.md` and block an external production pilot, but do not block the local implementation build.
