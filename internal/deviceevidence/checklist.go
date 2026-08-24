package deviceevidence

import (
	"fmt"
	"strings"
)

func Checklist(manifest Manifest) string {
	var output strings.Builder
	fmt.Fprintf(&output, "# LedgerSync physical-device evidence run %s\n\n", manifest.RunID)
	fmt.Fprintf(&output, "- Commit: `%s`\n", manifest.CommitSHA)
	fmt.Fprintf(&output, "- Target: `%s`\n", manifest.TargetURL)
	fmt.Fprintf(&output, "- Reviewer: %s\n", manifest.Reviewer)
	fmt.Fprintf(&output, "- Created: `%s`\n", manifest.CreatedAt.Format("2006-01-02T15:04:05Z"))
	output.WriteString("- Evidence status: `PENDING` until the complete manifest validator passes\n\n")
	output.WriteString("## Fixed financial fixture\n\n")
	fmt.Fprintf(&output, "Post exactly **%s** from Operating Reserve (`%s`) to Vendor Payables (`%s`). After recording the row, post a separate compensating transfer of **%s** in the opposite direction. Never delete or rewrite either transfer.\n\n", manifest.TestData.DisplayAmount, manifest.TestData.ForwardDebitID, manifest.TestData.ForwardCreditID, manifest.TestData.DisplayAmount)
	output.WriteString("## Per-device execution\n\n")
	for _, device := range manifest.Devices {
		fmt.Fprintf(&output, "### %s\n\n", strings.ToUpper(device.DeviceClass))
		output.WriteString("Record model, exact OS/browser version, physical viewport/orientation, locale, assistive technology, reviewer, and UTC completion time in `manifest.json`.\n\n")
		output.WriteString("- [ ] Normal network: complete navigation, account investigation, exact transfer, immediate balance, history, detail, and compensating transfer.\n")
		output.WriteString("- [ ] Slow network: repeat the transfer review and prove progress/status copy never implies completion early.\n")
		output.WriteString("- [ ] Offline before submit: prove the write is disabled and no transfer appears.\n")
		output.WriteString("- [ ] Lost response after submit: prove the request left the device, interrupt only the response, reconnect, and use **Retry same transfer**; record one transfer ID and one debit.\n")
		output.WriteString("- [ ] Open/close compact navigation, verify focus return, investigate and copy full identifiers, rotate/resize with a transfer draft, exercise the virtual keyboard, 200% text/zoom and 400% reflow, maximum signed-64-bit evidence, and screen-reader reading/action order.\n")
		fmt.Fprintf(&output, "- [ ] Upload `%s_%s_journey-recording.*`, `%s_%s_retry-recording.*`, and `%s_%s_accessibility-notes.*`; record immutable HTTPS URLs, SHA-256 digests, capture times, and retention dates.\n\n", manifest.RunID, device.DeviceClass, manifest.RunID, device.DeviceClass, manifest.RunID, device.DeviceClass)
	}
	output.WriteString("## Completion\n\n")
	output.WriteString("Set only observed results to `PASS`, record every defect, attach retest evidence for closed defects, set each device `completed_at`, then set the overall status to `PASS`. Run the complete validator. A verbal confirmation, emulator run, missing digest, open critical/high defect, or pending field fails completion.\n")
	return output.String()
}
