"""Create the presentation guide from local implementation and evidence records.

Run with the bundled Python runtime. No service or financial data is changed.
"""

from pathlib import Path
from html import escape

from reportlab.lib import colors
from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import ParagraphStyle
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.platypus import (
    Flowable, KeepTogether, PageBreak, Paragraph, SimpleDocTemplate,
    Spacer, Table, TableStyle,
)

ROOT = Path(__file__).resolve().parents[2]
PDF = ROOT / "output/pdf/ledgersync-unit-3-unit-4-demo-and-viva.pdf"
MD = ROOT / "docs/academic/ledgersync-unit-3-unit-4-demo-and-viva.md"
PDF.parent.mkdir(parents=True, exist_ok=True)
FONT_ROOT = Path("C:/Windows/Fonts")
for name, filename in [("Guide", "segoeui.ttf"), ("Guide-Bold", "segoeuib.ttf"),
                       ("Code", "consola.ttf")]:
    pdfmetrics.registerFont(TTFont(name, str(FONT_ROOT / filename)))
pdfmetrics.registerFontFamily("Guide", normal="Guide", bold="Guide-Bold")
INK = colors.HexColor("#19343B")
TEAL = colors.HexColor("#166653")
MUTED = colors.HexColor("#536A73")
LINE = colors.HexColor("#D5E1DE")
PALE = colors.HexColor("#F1F6F4")
WIDTH = A4[0] - 88
STYLES = {
    "title": ParagraphStyle("title", fontName="Guide-Bold", fontSize=26,
                            leading=31, textColor=INK, spaceAfter=13),
    "h2": ParagraphStyle("h2", fontName="Guide-Bold", fontSize=13,
                         leading=17, textColor=TEAL, spaceBefore=12, spaceAfter=6),
    "body": ParagraphStyle("body", fontName="Guide", fontSize=10,
                           leading=14.1, textColor=INK, spaceAfter=7),
    "small": ParagraphStyle("small", fontName="Guide", fontSize=8.4,
                            leading=11.4, textColor=MUTED, spaceAfter=5),
    "cell": ParagraphStyle("cell", fontName="Guide", fontSize=9,
                           leading=12, textColor=INK),
    "head": ParagraphStyle("head", fontName="Guide-Bold", fontSize=9,
                           leading=12, textColor=colors.white),
    "code": ParagraphStyle("code", fontName="Code", fontSize=8.1,
                           leading=11.5, textColor=INK, spaceAfter=2),
    "question": ParagraphStyle("question", fontName="Guide-Bold", fontSize=10,
                               leading=13.2, textColor=TEAL, spaceAfter=3),
}
story = []
markdown = ["# LedgerSync: Unit 3 and Unit 4 demonstration and viva", "",
            "Prepared: 12 October 2026. Based on the local implementation and saved study evidence.", ""]


def p(text, style="body"):
    return Paragraph(escape(text).replace("\n", "<br/>"), STYLES[style])


def para(text, style="body"):
    story.append(p(text, style))
    markdown.extend([text, ""])


def heading(text):
    story.append(p(text, "h2"))
    markdown.extend(["### " + text, ""])


def page(number, title, sub):
    if number > 1:
        story.append(PageBreak())
    story.append(p(f"{number:02d} / PRESENTATION GUIDE", "small"))
    story.append(p(title, "title"))
    para(sub, "small")
    markdown.extend(["## " + title, ""])


def steps(items):
    for i, item in enumerate(items, 1):
        story.append(p(f"{i}. {item}"))
        markdown.extend([f"{i}. {item}", ""])


def table(headers, rows, widths):
    cells = [[p(x, "head") for x in headers]]
    cells.extend([[p(str(x), "cell") for x in row] for row in rows])
    t = Table(cells, colWidths=[WIDTH * x for x in widths], repeatRows=1,
              hAlign="LEFT")
    t.setStyle(TableStyle([
        ("BACKGROUND", (0, 0), (-1, 0), TEAL),
        ("VALIGN", (0, 0), (-1, -1), "TOP"),
        ("LEFTPADDING", (0, 0), (-1, -1), 9),
        ("RIGHTPADDING", (0, 0), (-1, -1), 9),
        ("TOPPADDING", (0, 0), (-1, -1), 8),
        ("BOTTOMPADDING", (0, 0), (-1, -1), 8),
        ("ROWBACKGROUNDS", (0, 1), (-1, -1), [PALE, colors.white]),
        ("LINEBELOW", (0, 1), (-1, -1), 0.4, LINE),
    ]))
    story.extend([t, Spacer(1, 8)])
    markdown.append("| " + " | ".join(headers) + " |")
    markdown.append("| " + " | ".join("---" for _ in headers) + " |")
    markdown.extend("| " + " | ".join(str(x).replace("\n", "<br>") for x in row) + " |" for row in rows)
    markdown.append("")


def code(text):
    # Wrap at spaces for PDF only. Preserve exact copyable commands in Markdown.
    import textwrap
    lines = []
    for line in text.splitlines():
        lines.extend(textwrap.wrap(line, width=91, subsequent_indent="  ",
                                   break_long_words=False, break_on_hyphens=False) or [""])
    t = Table([[Paragraph(escape(line).replace(" ", "&#160;"), STYLES["code"])]
               for line in lines], colWidths=[WIDTH], hAlign="LEFT")
    t.setStyle(TableStyle([
        ("BACKGROUND", (0, 0), (-1, -1), PALE),
        ("BOX", (0, 0), (-1, -1), 0.5, LINE),
        ("LEFTPADDING", (0, 0), (-1, -1), 10),
        ("RIGHTPADDING", (0, 0), (-1, -1), 8),
        ("TOPPADDING", (0, 0), (-1, -1), 4),
        ("BOTTOMPADDING", (0, 0), (-1, -1), 4),
    ]))
    story.extend([t, Spacer(1, 9)])
    markdown.extend(["```powershell", text, "```", ""])


class Architecture(Flowable):
    def __init__(self):
        super().__init__()
        self.width, self.height = WIDTH, 202

    def draw(self):
        c = self.canv

        def box(x, y, w, label, detail):
            c.setFillColor(PALE)
            c.setStrokeColor(LINE)
            c.roundRect(x, y, w, 42, 4, stroke=1, fill=1)
            c.setFillColor(INK)
            c.setFont("Guide-Bold", 9.5)
            c.drawCentredString(x + w / 2, y + 25, label)
            c.setFont("Guide", 8)
            c.drawCentredString(x + w / 2, y + 11, detail)

        def arrow(x1, y1, x2, y2):
            from math import atan2, cos, sin
            c.setStrokeColor(TEAL)
            c.setLineWidth(1)
            c.line(x1, y1, x2, y2)
            angle = atan2(y2 - y1, x2 - x1)
            for offset in [-0.5, 0.5]:
                c.line(x2, y2, x2 - 5*cos(angle + offset), y2 - 5*sin(angle + offset))

        box(0, 154, 103, "Web console", "Next.js + BFF")
        box(129, 154, 107, "Private gateway", "Ready API selection")
        box(262, 154, 96, "Go APIs x2", "Authorized commands")
        box(385, 154, 122, "PostgreSQL", "Ledger + outbox")
        arrow(103, 175, 129, 175)
        arrow(236, 175, 262, 175)
        arrow(358, 175, 385, 175)
        box(262, 75, 96, "Workers x2", "Fenced claims")
        box(129, 75, 107, "Redis", "Stream + projection")
        box(0, 75, 103, "Consumers", "Receive + cache")
        arrow(425, 154, 425, 96)
        arrow(425, 96, 358, 96)
        arrow(262, 96, 236, 96)
        arrow(129, 96, 103, 96)
        box(262, 0, 96, "Schedulers x2", "Optional profile")
        box(385, 0, 122, "Lease + reports", "PostgreSQL coordination")
        arrow(358, 21, 385, 21)
        c.setFillColor(MUTED)
        c.setFont("Guide", 8)
        c.drawString(0, 27, "Solid arrows: request, message,")
        c.drawString(0, 14, "or database operation.")


page(1, "LedgerSync\nDemo and viva", "UNIT 3 + UNIT 4  |  Prepared 12 October 2026")
heading("What we built")
para("LedgerSync is an internal financial ledger with a web console and background workers. It records exact amounts, balanced journals, and transfer evidence. The course additions show how services coordinate, process events, and recover from selected failures.")
para("Use this opening: 'Our project is a distributed web application for an internal ledger. PostgreSQL stores the financial record. Docker runs separate API and worker services. We demonstrate logical clocks, database coordination, consistent reports, and controlled failover.'")
heading("Your 10-minute presentation")
table(["Time", "Show", "Explain"], [
    ["0-1 min", "Architecture on page 2", "Separate services; one financial authority."],
    ["1-3 min", "Developer observations", "Two ready APIs; recorded worker activity; sampled scheduler lease."],
    ["3-5 min", "Event order", "Follow one event through origin, publication, and receive clocks."],
    ["5-6 min", "Snapshot boundary", "One consistent PostgreSQL report."],
    ["6-8 min", "Saved concurrency and recovery evidence", "No overspend; safe replay; replacement after selected failures."],
    ["8-10 min", "Measured results and limits", "Fixed demand, one host, no production capacity claim."],
], [0.16, 0.35, 0.49])
heading("Prepare before the class")
para("Build and rehearse the study stack before the presentation. Keep the PDF and evidence files offline. Run the longer checks before class. During the presentation, show the UI and saved reports. Run a live fault check only if time and Docker are available.")
para("The guide describes existing features. Creating this PDF did not run financial commands or fault checks.", "small")

page(2, "Map the additions", "Use the syllabus terms accurately. A related mechanism is not the same as every named algorithm.")
story.extend([Architecture(), Spacer(1, 12)])
markdown.extend(["Architecture: Web/BFF -> private gateway -> two Go APIs -> PostgreSQL ledger/outbox.",
                 "PostgreSQL outbox -> two workers -> Redis -> projection consumers.",
                 "Two optional schedulers -> PostgreSQL lease coordination -> internal reports.", ""])
table(["Syllabus area", "Implemented addition", "Purpose"], [
    ["Unit 3: logical clocks / Lamport", "Durable origin, send, and receive counters", "Explain causal message order."],
    ["Unit 3: mutual exclusion", "Account locks, advisory locks, fenced claims", "Prevent conflicting financial and worker updates."],
    ["Unit 3: global state", "One repeatable-read database snapshot", "Report a consistent ledger boundary."],
    ["Unit 3: election", "Database lease with scheduler takeover", "Select one current scheduling owner."],
    ["Unit 4: distributed web systems", "Web, private gateway, API and worker replicas", "Show communication and independent service scaling."],
    ["Mini-project deployment", "Isolated Docker Compose stack", "Run repeatable demonstrations with synthetic data."],
], [0.28, 0.38, 0.34])
para("Kubernetes, serverless platforms, Hadoop, distributed file systems, vector clocks, physical clock synchronization, blockchain, and the named election/fair-mutex/deadlock algorithms are not implemented here. Docker satisfies the stated deployment requirement. The snapshot is not a Chandy-Lamport global snapshot; the scheduler is not Bully, Ring, or Raft.", "small")

page(3, "Start the study stack", "PowerShell commands run from D:\\Work\\Project\\Dev\\LedgerSync.")
heading("1. Preserve the saved evidence")
para("Some study scripts replace report files. Before a new run, copy the saved evidence to a new dated folder. Keep failed results as well as successful results.")
code("Set-Location 'D:\\Work\\Project\\Dev\\LedgerSync'\n$backupName = 'unit-evidence-' + (Get-Date -Format 'yyyyMMdd-HHmmss')\nNew-Item -ItemType Directory -Path '.\\tmp\\presentation' -Force | Out-Null\nCopy-Item -LiteralPath '.\\docs\\release-evidence\\unit-3-unit-4' `\n  -Destination (Join-Path '.\\tmp\\presentation' $backupName) -Recurse")
heading("2. Start Docker Desktop and build the isolated stack")
para("Keep Docker Desktop running. The last saved cleanup removed the study containers and image tags. Rebuild on the first start. Do not add -SkipBuild to this first command.")
code(".\\scripts\\start-unit-study.ps1 `\n  -APIReplicas 2 -WorkerReplicas 2 -IncludePeer -WithScheduler\n.\\scripts\\test-unit-study-final.ps1")
para("Expected: a financial command passes, the identical replay passes, and reconciliation matches. The final check uses a synthetic amount of 100 minor units. With INR, this is INR 1.00. It changes only study data.")
heading("3. Open the correct workspace")
table(["Address", "Use"], [
    ["http://127.0.0.1:3000/", "Normal workspace; scheduler disabled by default."],
    ["http://127.0.0.1:3102/", "Synthetic study; use this for the presentation."],
    ["http://127.0.0.1:3103/", "Second web instance for cross-instance journeys."],
], [0.47, 0.53])
para("Use the existing local sign-in flow. Do not show runtime.env, credentials, cookies, or assertions. The study uses a separate Compose identity and data volumes. Its database and Redis remain private.", "small")

page(4, "Show the UI locations", "The new details stay inside existing pages. Open the disclosure when you need technical evidence.")
heading("A. Developer: replicas and scheduler")
steps([
    "In the study at port 3102, open Platform in the sidebar. Select Developer (/developer). If the sidebar is collapsed, expand it first.",
    "Expand Distributed systems. Read Ready API replicas and its collection time. With both APIs ready, expect 2.",
    "Read Historical publication evidence. It counts worker identities in the latest 1,000 published attempts. It is not a count of workers running now.",
    "Read Active scheduler. Show its owner, generation, lease expiry, and sample time. Select Refresh observations for a new sample.",
    "Select Download historical test evidence. Explain that this is a saved report asset, not a live stream of results.",
])
para("Say: 'These observations have separate collection times. An active lease is active at the sample time. A retained sample becomes historical.'")
heading("B. Events & webhooks: logical event order")
steps([
    "Open Investigate, then Events & webhooks (/events). Select a recent study event to open its details.",
    "Expand Event order. Show the origin node, logical timestamp, and parent reference.",
    "Read Publication attempts and Receive clocks. Follow the same event ID. Delivery attempts remain a separate section.",
])
heading("C. Internal reconciliation: snapshot boundary")
steps([
    "Open Internal reconciliation (/reconciliation). Select a recent completed study report.",
    "Read the result and mismatch count. Expand Snapshot boundary.",
    "Show Repeatable read, schema version, ledger watermark, database snapshot time, and recorded delivery counts.",
])
para("Older events and reports can lack the added metadata. Select a new study record. The UI must show unavailable fields honestly.", "small")

page(5, "Explain clocks and snapshots", "Use one event and one completed report. Avoid claims that the evidence cannot support.")
heading("Demo 1: follow a causal message")
para("If you need a fresh trace, run this before class. It creates study events, waits for receipts, checks order, and saves causal-trace.json.")
code(".\\scripts\\test-unit-study-clocks.ps1")
steps([
    "Open the saved trace. Find trace.origin and choose an event ID. Open that event in the study UI at /events/<event-ID>.",
    "Read the API origin counter. The API increases its counter before it records the outbox event.",
    "Read the worker receive counter. It must exceed the origin counter. Read the publication send counter; it must exceed that receive counter.",
    "Read the consumer receipt. Its receive counter must exceed the publication send counter.",
])
code("Local event:      C = C + 1\nMessage receive:  C = max(C_local, C_sender) + 1\nExample only:     origin 12 -> worker receive 13 -> send 14 -> receive 15")
para("Say: 'These are logical counters, not wall-clock times. A message receive follows its send. A larger counter between unrelated events does not prove causation. Aggregate versions still control cache update order.'")
heading("Demo 2: read one consistent report")
steps([
    "Open a completed reconciliation result. State its tenant scope and ledger watermark.",
    "Expand Snapshot boundary. Read its database sample time and the pending, published, and stopped delivery counts.",
    "Explain that the financial checks and recorded outbox counts use one repeatable-read PostgreSQL transaction.",
])
para("Say: 'This report checks internal ledger consistency at one database boundary. Redis and process observations have separate times. A matched report does not prove bank settlement.'")
para("For a longer prepared check, use test-unit-study-snapshot.ps1. It includes cache loss, stale cache, and database unavailability. Use its saved snapshot-report.json during the short presentation.", "small")

page(6, "Show safe concurrency", "Use the saved reports for a short presentation. The scripts below run only in the isolated study.")
heading("Demo 3: competing transfers")
code(".\\scripts\\test-unit-study-concurrency.ps1")
steps([
    "Open concurrent-transfers.json. Show the source funded with 10 units and 20 simultaneous debit attempts of 1 unit.",
    "Show the result: 10 transfers posted and 10 were rejected. The account did not overspend.",
    "Show the opposite-direction transfer check and the same-key request check across API instances. There is one financial result for the repeated command.",
    "Show matched reconciliation and balanced journals. Explain that the controls use PostgreSQL locks and constraints shared by both APIs.",
])
para("Say: 'Mutual exclusion protects shared account state. Idempotency protects one command from duplicate movement. They solve different problems.'")
heading("Demo 4: parallel workers")
code(".\\scripts\\test-unit-study-workers.ps1")
steps([
    "Open parallel-workers.json. Show two distinct worker identities processing the study workload.",
    "Show that pending work drains and no dead events remain in that recorded check.",
    "Explain SKIP LOCKED claims: a worker skips rows already locked by another worker. Each committed claim has an owner, expiry, and increasing generation.",
    "Explain fencing: the database rejects an acknowledgment from an expired or replaced generation. A lease alone cannot reject an old process after it resumes.",
])
heading("Keep posting separate from delivery")
para("A transfer can be posted while its downstream event is pending. The financial transaction commits the ledger and outbox together. Workers publish after that commit. Delivery is at least once. A repeated event must keep its event ID, and consumers must handle duplicates.")
para("A worker identity in a historical report proves recorded activity. Use readiness or a fresh process check to discuss current service state.", "small")

page(7, "Show controlled failover", "Do these drills during rehearsal. They can stop or pause study services.")
heading("Demo 5: lose one API")
code(".\\scripts\\test-unit-study-api-failure.ps1")
steps([
    "Before the drill, refresh Developer observations and show two ready APIs.",
    "Run the script in the study PowerShell session. It sends a bounded workload, stops one API, and restores it in cleanup.",
    "Open api-failure.json. Show that all 40 commands resolved, journals stayed balanced, and reconciliation matched.",
    "Explain the interrupted attempt: the caller repeats the same command with the same key. The gateway does not silently retry financial commands.",
])
para("Recorded detection was 1,661.84 ms. This includes Docker command and probe overhead. It is not a production availability guarantee.")
heading("Demo 6: replace the scheduler owner")
code(".\\scripts\\test-unit-study-scheduler.ps1")
steps([
    "In Developer, show the sampled scheduler owner and generation. The study starts two scheduler candidates.",
    "Run the prepared check. It stops and pauses the current owner in separate cases. A replacement obtains the expired lease.",
    "Open scheduler-takeover.json. Show the replacement, rejected stale acknowledgment, and unique scheduled intervals.",
    "Refresh the UI. Show the new owner. Show that manual reconciliation still works.",
])
para("Say: 'PostgreSQL coordinates leader selection with database time and a fencing generation. The scheduler can create reports for its allowed synthetic tenant. It cannot submit transfers.'")
para("Study takeover: 4,901 ms after stop; 5,025 ms after pause. Study lease: 4 seconds. Normal defaults differ. A report can repeat after an unknown completion, but the scheduler does not repeat a financial movement.", "small")

page(8, "Present measured results", "Read the saved measurements. Do not turn a small experiment into a capacity claim.")
heading("The recorded comparison")
para("Each configuration used 10 paced clients, a 5-second warm-up, and a nominal 30-second workload. Each run posted 300 new transfers and included 30 replays. Data was reset between runs. Each API and worker had 1 CPU, 256 MiB memory, and 4 database connections. Total resources grew with replicas.")
table(["API / worker", "Independent p95", "Contended p95", "Posted/s range"], [
    ["1 / 1", "187.00 ms", "287.55 ms", "10.24-10.29"],
    ["2 / 1", "192.30 ms", "207.20 ms", "10.28-10.28"],
    ["1 / 2", "240.00 ms", "168.10 ms", "10.19-10.29"],
    ["2 / 2", "194.55 ms", "214.10 ms", "10.25-10.28"],
], [0.20, 0.27, 0.27, 0.26])
para("All eight runs recorded no unresolved commands, duplicate journals, or unbalanced journals. Delivery drained and internal reconciliation matched. Latency improved in some runs and increased in others. Throughput stayed near the fixed offered demand.")
heading("What you can conclude")
para("The system supports separate API and worker replicas. Shared controls kept the financial results correct in these experiments. The tested failures recovered. The fixed-demand test did not prove that more replicas increase maximum throughput.")
heading("What you must qualify")
para("These were short tests on one Docker host. PostgreSQL was a common dependency. The account and tenant policy locks limit concurrent commands. The tests do not establish multi-host availability, maximum capacity, sustained production load, or statistical significance.")
para("Worker crash recovery was 620 ms before publication and 615 ms after publication. Both include a deliberate 600 ms wait. The second case delivered the same event twice. Do not present these as intrinsic production recovery times.")
heading("Evidence to open")
para("Use performance-report.md, recovery-report.json, api-failure.json, scheduler-takeover.json, and worker-crash-verified.txt under docs/release-evidence/unit-3-unit-4/.", "small")
para("The full performance runner can take time and temporarily stops the original study database and cache. Use its saved report in class. It leaves the normal workspace outside its boundary.", "small")

page(9, "Finish and recover safely", "Keep a fallback ready. A missing observation must stay missing; do not invent a value.")
table(["Problem", "Action"], [
    ["API count unavailable at port 3000", "Open the study at 3102. The normal workspace does not infer study counts."],
    ["Old event has no clocks", "Select the new event ID from causal-trace.json."],
    ["Old report has no snapshot counts", "Select a new completed study report."],
    ["Lease is historical", "Refresh observations. Check that the scheduler profile is running."],
    ["Access denied", "Use an authorized local session. Developer observations require developer:read."],
    ["Docker or build fails in class", "Show this guide and the saved evidence. State that the evidence is recorded, not live."],
], [0.36, 0.64])
heading("Stop the optional study scheduler")
code(". .\\scripts\\unit-study-common.ps1\nInvoke-LedgerSyncCompose -ComposeArguments @(\n  '--profile','scheduler','stop','reconciliation-scheduler'\n)")
heading("Remove only the isolated study when you finish")
para("This removes synthetic study data. Check the exact project name before the remove command. Do not use a global prune or target the normal compose project.")
code(". .\\scripts\\unit-study-common.ps1\n$studyState.project\nInvoke-LedgerSyncCompose -ComposeArguments @(\n  '--profile','*','down','--volumes','--remove-orphans'\n)")
para("Use a fresh PowerShell session for normal workspace commands. Keep the saved reports and backup. Normal start/status commands are scripts/start-local.ps1 and scripts/status-local.ps1.", "small")
heading("How to describe the product")
para("'LedgerSync is an internal ledger and operations console. It records and checks financial movements within its configured scope. It also demonstrates distributed service coordination.' It is not a bank, a payment rail, a custodian, or a blockchain network.")

VIVA = [
    ("What is LedgerSync?", "It is an internal financial ledger with a web console and workers. PostgreSQL stores the authoritative financial record. The study adds distributed coordination evidence."),
    ("What does distributed mean in this project?", "Separate web, API, worker, database, and cache processes communicate across Docker networks. They run on one host in this study. This does not prove multi-machine fault tolerance."),
    ("What is a ledger?", "A ledger records financial entries. LedgerSync keeps journal postings and account balances. Committed financial evidence remains available for investigation."),
    ("What is double-entry accounting?", "Each journal has equal total debits and credits. This helps detect an unbalanced movement. It does not prove that the business reason for the movement was correct."),
    ("How are amounts stored?", "Amounts use exact integer minor units. The API preserves exact decimal strings. With INR, 100 minor units equal INR 1.00. Floating-point arithmetic is unsuitable for exact ledger amounts."),
    ("What is idempotency?", "The same command, key, and body return the original financial result instead of creating another movement. Reusing the key with a different payload is rejected."),
    ("Why is PostgreSQL authoritative?", "It commits financial updates, postings, audit evidence, and the outbox atomically. Redis is disposable. A cache value cannot replace an authoritative financial check."),
    ("What is the outbox pattern?", "The financial transaction stores an event in PostgreSQL with the ledger change. A worker publishes that committed event later. This closes the gap between committing a movement and recording its delivery work."),
    ("What is internal reconciliation?", "It checks ledger consistency within the recorded scope and snapshot. A matched report is internal evidence. It is not proof that a bank transfer settled."),
    ("What is the practical use case?", "A team can record internal movements, review funding evidence, and investigate posting or delivery state. A real deployment also needs suitable operational, legal, and business controls."),
    ("What is a Lamport clock?", "It is a logical counter for local events and message order. Increase it for a local event. On receipt, set it to max(local, sender) + 1."),
    ("Does a larger counter prove causality?", "No. If one event causally precedes another, its Lamport value is smaller. The reverse is not sufficient. Use the same event ID and send/receive evidence to follow the message path."),
    ("Are Lamport clocks physical clock synchronization?", "No. They do not synchronize UTC or measure elapsed time. Database time controls lease expiry. A multi-host physical clock synchronization study was not added."),
    ("What happens to a clock on restart?", "The counter is stored in PostgreSQL for its node identity. Restarting the same identity resumes its counter. A replacement with a new identity has a separate counter."),
    ("What remains unchanged during an event retry?", "The event ID, origin node, origin counter, and parent reference remain unchanged. A new publication claim records its own worker receive/send counters."),
    ("How do we implement mutual exclusion?", "PostgreSQL account locks and advisory locks coordinate shared operations. Fixed account lock order reduces deadlock risk. This is database coordination, not a named fair distributed mutex algorithm."),
    ("What did the concurrency test establish?", "With 10 units available, 20 competing debit attempts of 1 unit produced 10 posted and 10 rejected results. It also checked opposing transfers and same-key requests. Journals stayed balanced and reconciliation matched."),
    ("What is a lease?", "A lease is ownership that expires unless renewed. The database uses its own time for expiry. A replacement can claim expired work."),
    ("Why do we also need fencing?", "An old process can resume after its lease expires. An increasing generation lets the database reject that process's stale update. Ownership checks include owner, generation, and expiry."),
    ("Is our snapshot a Chandy-Lamport snapshot?", "No. It is one repeatable-read PostgreSQL transaction. It excludes process state and messages in transit outside that database. Redis and service observations have separate collection times."),
    ("Why use Docker Compose?", "It defines services, networks, volumes, and replica settings for a repeatable study. The mini-project requirement accepts Docker or Kubernetes. Compose fits this single-host demonstration."),
    ("Why did we not add Kubernetes?", "The required Docker deployment already works. This study did not establish a need for cluster orchestration. Kubernetes can be a later study for multi-host scheduling and operational requirements."),
    ("How does API load balancing work?", "A private Go gateway checks readiness and refreshes Docker DNS. It distributes requests among ready API instances. It preserves request and authorization context and does not automatically retry financial commands."),
    ("How can two APIs share safe command behavior?", "Shared PostgreSQL controls coordinate idempotency, authorization-related replay controls, and financial updates. Per-process memory alone would not safely enforce a system-wide financial invariant."),
    ("What does FOR UPDATE SKIP LOCKED do?", "It locks selected rows and skips rows already locked by another worker. This allows workers to claim different queued work. Stored lease and fencing checks protect updates after the claim transaction ends."),
    ("Is delivery exactly once?", "No. Delivery is at least once. A crash after publication but before acknowledgment can cause a second delivery of the same event. Consumers use idempotency and aggregate-version checks."),
    ("How does scheduler election work?", "Candidates coordinate through a PostgreSQL tenant lease. Only the current owner and generation can manage the scheduled job. A replacement takes over after expiry. This is not Bully, Ring, or Raft."),
    ("Can the scheduler move money?", "No. Its restricted identity can create internal reconciliation evidence only for allowed tenants. It cannot submit transfers or read unrestricted base financial tables."),
    ("Can a scheduled report repeat?", "Yes, if the report commits but its job acknowledgment is lost. The job interval remains unique and ownership is fenced. A repeated report does not create a repeated financial movement."),
    ("Is this blockchain or distributed ledger technology?", "No. It is a ledger application with distributed service processes and one PostgreSQL authority. It has no blockchain consensus, blocks, or peer-replicated ledger network."),
    ("What does the performance test prove?", "It compares selected replica configurations at a fixed demand on one host. Correctness checks passed. It does not measure maximum capacity or establish a proportional throughput increase."),
    ("What are p50 and p95?", "p50 is the median measured response time. p95 is the value at or below which 95 percent of measured responses finish. They describe the measured workload, not every future request."),
    ("Why did throughput stay near 10.2 commands/s?", "The clients offered a paced workload of roughly that rate. More replicas cannot show higher capacity when demand stays fixed. Database locks can also limit concurrent commands."),
    ("Why can more replicas increase latency?", "They add connections, contention, and scheduling overhead. Short runs also contain measurement noise. The report retains both improvements and regressions."),
    ("What happens when one API fails?", "The gateway excludes the failed instance after its check. An interrupted caller can safely repeat the same command with the same key. The recorded drill resolved 40 commands and reconciled."),
    ("What happens when PostgreSQL fails?", "Authoritative financial commands and checks cannot complete safely. The system reports unavailability instead of assuming success from Redis. This study does not provide PostgreSQL high availability."),
    ("What happens when Redis fails?", "The financial record remains in PostgreSQL. Projection and transport work can be delayed. The recorded snapshot check exercised cache loss and stale cache; the UI must show source and collection state honestly."),
    ("How did we check existing features?", "The saved final validation records 42 passing Go packages, 214 frontend unit tests, 245 browser checks, and 6 real-stack journeys. These are recorded results for a captured source boundary, not new checks run to create this guide."),
    ("What are the main limits?", "The study uses one host and one authoritative database. Delivery is at least once. The snapshot is database-only, benchmarks are short, and a Windows race check was not executed. These limits must stay in the presentation."),
    ("What would you add next?", "Choose one measured need first. Possible studies include database failover, repeated saturation benchmarks, or multi-host deployment. Do not add Kubernetes, serverless, or blockchain only to increase the feature count."),
]

for group, title in enumerate(["Viva: project basics", "Viva: Unit 3", "Viva: Unit 4 and recovery", "Viva: results and limits"]):
    page(10 + group, title, "Give the short answer first. Add the evidence only when the examiner asks.")
    for idx, (question, answer) in enumerate(VIVA[group*10:(group+1)*10], group*10 + 1):
        story.append(KeepTogether([
            p(f"{idx:02d}. {question}", "question"), p(answer), Spacer(1, 3),
        ]))
        markdown.extend([f"### {idx}. {question}", "", answer, ""])

page(14, "Find the implementation", "All paths below are relative to D:\\Work\\Project\\Dev\\LedgerSync.")
table(["Feature", "Main implementation path"], [
    ["Study deployment", "deploy/compose/docker-compose.study.yml\nscripts/start-unit-study.ps1"],
    ["Private gateway", "cmd/study-proxy/main.go\ninternal/platform/routing/"],
    ["Worker coordination", "internal/application/lease/\ninternal/platform/db/outbox_repository.go\nmigrations/000041_worker_claim_fencing.up.sql"],
    ["Lamport clocks", "internal/platform/db/logical_clock_repository.go\ninternal/application/outbox/causal_publisher.go\nmigrations/000042_lamport_event_metadata.up.sql"],
    ["Snapshot evidence", "internal/application/reconciliation/snapshot.go\ninternal/platform/db/reconciliation_repository.go"],
    ["Optional scheduler", "cmd/reconciliation-scheduler/main.go\ninternal/application/scheduler/\nmigrations/000043_reconciliation_scheduler.up.sql"],
    ["Developer UI", "web/src/features/developer/DistributedSystems.tsx"],
    ["Event order UI", "web/src/features/operations/EventOrderEvidence.tsx"],
    ["Snapshot UI", "web/src/features/reconciliation/ReconciliationViews.tsx"],
], [0.26, 0.74])
heading("Local source and evidence references")
para("Implementation plan: docs/plans/ledgersync-unit-3-unit-4-implementation-plan.md. All 11 required steps are marked verified. Product release status remains separate from academic study completion.", "small")
para("Evidence folder: docs/release-evidence/unit-3-unit-4/. Read handover.md for commands, clock-rules.md for counter rules, scheduler.md for lease rules, performance-report.md for methods, and progress.md plus final-validation.md for verification history. Retained failed runs explain corrections.", "small")
para("Recorded evidence has its own source and image fingerprints. A fresh run can use a different working tree. Label it as a new run and preserve the original evidence. The downloaded web asset stays historical until it is regenerated and the web images are rebuilt.", "small")


def footer(canvas, doc):
    canvas.saveState()
    canvas.setStrokeColor(LINE)
    canvas.line(44, 43, A4[0]-44, 43)
    canvas.setFont("Guide", 8)
    canvas.setFillColor(MUTED)
    canvas.drawString(44, 29, "LedgerSync | Unit 3 + Unit 4 | Academic demonstration")
    canvas.drawRightString(A4[0]-44, 29, f"{doc.page} / 14")
    canvas.restoreState()


doc = SimpleDocTemplate(str(PDF), pagesize=A4, rightMargin=44, leftMargin=44,
                        topMargin=40, bottomMargin=57,
                        title="LedgerSync: Unit 3 and Unit 4 Demonstration and Viva",
                        author="LedgerSync", subject="Presentation procedure and 40 viva answers")
doc.build(story, onFirstPage=footer, onLaterPages=footer)
MD.write_text("\n".join(markdown), encoding="utf-8")
print(f"PDF: {PDF}")
print(f"Markdown: {MD}")
