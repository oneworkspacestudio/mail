# Incoming SMTP recovery and first-run network checks

## Request and confirmed cause

User reported Gmail-to-Workspace mail missing and asked that first-time installers
avoid the same failure. Existing public SMTP DNS and native TCP25 listener were
correct, Windows firewall already allowed the exact native engine, but the router
had no TCP25 forwarding rule. HTTPS webmail reachability did not establish SMTP
reachability. Three external TCP probes timed out before repair.

Added a scoped, read-back-confirmed leased TCP25 UPnP rule to this PC only, without
changing another rule, DNS, Windows firewall policy, or separate live installations.
After repair two external locations connected (one timed out), and an SMTP probe
accepted the owner's recipient then QUIT before DATA: no new email was sent.
Initially Gmail receipt remained unconfirmed and a fresh test question was pending.
Update: Gmail retried the original user message; native SMTP ingest and actual
MyAccounts-authenticated Mail UI confirmed receipt from Gmail to the owner address
at 11:17:29 BST (10:17:29 UTC). No fresh test send was needed or performed. Do not
copy private subject/body into history. Never infer delivery from TCP alone.

## Implementation

workspace-mail/network.go discovers IGD on the selected private IPv4 LAN, confines
HTTP description/control endpoints to the responding private same-subnet peer,
disallows redirects, bounds XML, and checks public router IP against SMTP DNS.
TCP25 conflicts/disabled/unknown rules fail closed; matching manual rules stay
untouched. Only an unused rule or this app's exact existing rule can be created/
renewed. Lease1800s, renewal/retry5min while managed hosting runs; no permanent rule.
Public/shared NAT, DNS mismatch, unsupported router and mapping errors have guidance.

Managed first run and later starts also probe public TCP25 through Check-Host,
sharing only the public SMTP hostname (documented in README). Bounded requests,
validated result IDs/public addresses, no email/content/credential transmission.
States checking/reachable/attention/unverified distinguish blocked and unavailable
checks. Routing errors cannot become success just because some public SMTP answers.
setup.go exposes status, hosting UI shows Incoming mail, mailbox warning links to
network guidance, owner-authorized Retry automatic setup wakes the check immediately.
No production SMTP auth policies were relaxed. No new dependencies/Docker/service.

Suite mail_progress.go waits for incoming check, uses done only for reachable;
blocked/inconclusive/older Mail status produces actionable warning, others remain
usable. This is TCP reachability, not full delivery/spam/signing verification.

## Verification / local deployment

Mail all Go tests pass including fresh NTFS opt-in native provisioning fixture,
16send-parser cases, gateway endpoint/mapping conflict/renewal/readback cases and
probe pending/blocked/malformed/shared/private/success cases. Suite mock first-run
progress tests cover reachable/blocked/unverified/legacy status and directory
provisioning before login. No full clean-host installer or reboot acceptance run.
Git diff --check and Windows builds passed. Existing separate live installations
were not test targets; actual authorized suite received the repair.

Built Mail0.1.0-inbound-preview.11; suite/setup0.3.0-inbound-preview.14. Actual install
C:/tmp/workspace uses these with unchanged Drive drive-upload-preview.3 and Photos
photos-upload-preview.2. Normal exact-path-verified manager stop; changed EXE/config
backups Private/backups/mail-inbound-20261015-111320. No mailbox schema/data change.
Final manager initial PID56856; always refresh running.json before future stops.
Final deployed build public status reachable, no hosting/identity errors, startup
ready8/8, mailbox sync ready. Actual browser shows original Gmail sender, owner
recipient and receipt timestamp. Proof outside Git:
C:/Projects/artifacts/mail-inbound-20261015/gmail-received.jpg.
No full clean-host/service/reboot test claimed.

Local ignored artifacts workspace-mail/dist/Mail-inbound-preview.exe and SHA256SUMS;
source/build staging C:/Projects/.cache/mail-inbound-fix. Suite packaged source
C:/Projects/.cache/photos-upload-suite. No commit/push/release/Actions authorized.
Preserve all prior Mail/suite uncommitted fixes and concurrent Drive styling.

## Remaining limits / resume

No guarantee for routers without UPnP, TCP25 already owned elsewhere, Windows
explicit block policy, blocked ISP, double/shared NAT: guidance calls for scoped
firewall allow, manual upstream forwarding/public IP or inbound mail gateway.
Outgoing relay alone does not receive mail. Public probe dependency failure stays
unverified, never ready. IPv6-only networks are not implemented by this IPv4 route.
Original Gmail incoming delivery is confirmed for this installation; a fresh test
send is unnecessary. Prior unsigned outbound DKIM/Gmail Spam placement remains open;
do not call all-provider deliverability or inbox placement solved.
