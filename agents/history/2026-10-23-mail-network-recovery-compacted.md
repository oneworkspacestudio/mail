# Mail incoming connection recovery and stable settings

Follow-up to Mail18: reported automatic setup remains blocked and Advanced
settings closes immediately. Repository-history loaded; based on4f41379e.
Branch codex/mail-network-recovery, same clean isolated I worktree. Own org only,
no Actions/upstream contacts. Current prior authorization includes publication
and installation of this ongoing Mail setup repair. No automated tests requested.

UI status polling previously rebuilt the entire page every1.5seconds while
incoming mail was unverified. Preserve open named panels; render only when status
changes. A generation guard cancels late requests after leaving setup, so an
advanced form or subdomain review is not overwritten. Connection-only retry
avoids DNS/account reconfiguration. UI separates mailbox/address readiness from
incoming connectivity, displays the blocker and next check, keeps technical text
in details, and does not show a fake completion spinner while waiting.

Native adapter gateway lookup and targeted IGD SSDP searches supplement multicast.
UPnP URLBase supported with same-private-peer endpoint validation. Missing upstream
rule can be created only when an owned inner route and verified private WAN target
establish the destination. Compatible unowned outer route is reused read-only and
excluded from delete/refresh. Other-receiver rules are never overwritten.
Structured router permission/conflict help preserves raw diagnostic under details.

Actual affected browser is workspace-2; it now runs Workspace75 and Mail18 and
published post-domain setup succeeded. Latest live status says incoming SMTP25
already points to another server, not merely a DNS conflict. This PC's local
Workspace74 is stopped; do not assume it is the target or start it as a fixture.
Asked user whether original receiver should serve both installations or whether
this server replaces it. Decision/access still pending. One public SMTP25 route
cannot independently deliver to two PCs just by creating another mail subdomain.
Do not silently overwrite another working mail service or claim inbound delivery.

Windows compilation, JS syntax and diff checks passed. No automated/native
fresh-host or external delivery acceptance. Artifacts on I under
Projects/tooling/mail-network-20261023; publication/deployment to be recorded.

Publication discovered a packaging gap in standalone19: the generic managed
updater requires workspace-update.json (schema1/integration2), not only the
fresh-installer workspace-mail.json marker. Remote check correctly refused19;
no install was attempted. Prepare20 with a tracked managed-release generator and
all three owned packages (Mail, license, notice), keeping paths/contracts intact.
Keep19 and all published tags/assets unchanged; promote20 once verified.


## Multi-PC Mail21 follow-up
Owner confirmed the existing receiver remains main and approved connecting the
secondary installation to the existing main Workspace Mail address. Generic
setup must work without agent conversation. Added owner-controlled shared-service
mode, early fresh-install discovery via optional zone TXT pointer / standard main
hostname, explicit review and custom-address entry. HTTPS metadata probes use no
credentials, no redirects, public pinned DNS and bounded time/body; readiness is
cached/coalesced. Unavailable servers remain visibly unavailable, with automatic
retry. Main accounts are used; no federation or email-based account merging.

Local engine and incoming checks stop after selecting a main service; protected
local state/messages remain. In-flight network work is cancelled and joined;
settings login returns to /setup and does not require a local mailbox. Switching
back resumes local startup and falls back to the previous shared connection if
startup fails. Existing receiver rules remain untouched. This is shared access,
not replicated stores or a multi-domain SMTP gateway.

The confirmed main URL currently returns502. Local main installation is stopped
while another Slicer update holds its configuration lock. Do not stop or replace
that unrelated installer. One attempted normal main start refused the held lock;
no running main suite was changed. Target secondary owner update to21 pending.
All large source/build output is on I. No automated tests requested/run.


Mail21 publication prompted an operational compatibility correction before
installing it: existing Workspace managers correctly return401 for anonymous Mail
metadata. Mail22 retains that gate and recognizes only its exact401 response plus
public /healthz with Mail running, describing process availability without claiming
mailbox setup/delivery. Standalone public Mail metadata remains supported. Fresh
shared clients report readiness to the manager without a local mailbox; settings
and local OIDC stay protected. A browser handoff marker stops automatic satellite
forwarding chains. No manager authentication protection was relaxed.


Main Mail22 now runs and reveals a genuine outer-router permission/discovery
blocker. Its local router is configured; the upstream device does not accept
automatic setup. The generic discovery error omitted the structured router URL,
so Mail23 routes that outcome through existing upstream_router help and the
owner-only Open router settings action. No new rule is created or unowned rule
replaced by this UI correction. Physical router permissions/ISP restrictions
remain real prerequisites; shared clients do not each need those permissions.

## Resumed October24 deployment and diagnostics
Public Workspace76/Mail23 channel promoted on Workspace main (fe17dd7), rebased
onto concurrent Music publication without dropping it. Unauthenticated raw index
contains both tags; normal75 controller now offers76 and23. Main selected only
Workspace/Mail and is running its normal verified-backup update. A separate
secondary operation includes Mail23/Music/Slicer/Social; do not interrupt/replay it.
Main Mail opened signed in and loaded its existing Inbox. Advanced details stayed
expanded across a real incoming-status change. Secondary accounts-2 sign-in is
reported failing; its browser still holds an old OAuth request and installation
is restarting. Need fresh normal authorization after restart before diagnosing
password vs callback; never reset credentials speculatively.
Router owner has signed in. Read-only Archer inspection found no TCP25 static or
UPnP rule; eero DHCP lease is .120 on upstream subnet, inner mapping routes25 to
the main PC. Prepared TCP25 -> eero .120:25 in router UI; saving requires pending
user confirmation for Internet exposure. No router setting changed. Main shared
Mail pairing on secondary remains pending. C named76 link scratch removed after
exact-path, reparse and build-process checks; free space was17.06GiB. Remote Run
partial recovery command was cancelled without execution; on-screen-input restored
to its original off state. No protected capabilities or authentication credentials
were displayed, persisted in source or sent to another service.

## Mail24 wording correction after real delivery
User approved the prepared router TCP25 rule; saved and verified enabled with
matching eero DHCP reservation. A fresh accounts-2 portal authorization and Mail
setup sign-in completed automatically, without password changes. Saved approved
main URL on secondary; shared page correctly reports main outage, then availability.
Main76/Mail23 received the authorised Gmail test in its Inbox and its reply arrived
in Gmail. Only labelled test content/addresses were used; no other mailbox content
is copied into history. Router inspection still fails across NAT despite working
manual forwarding, so23 incorrectly says it needs permission. Mail24 now separates
unavailable inspection from public SMTP response and actual delivery, tells users
to keep working settings when mail arrives, and uses that guidance in the Inbox
warning. No route, credential, auth or delivery logic changes. JS syntax passed;
Windows build, publication/promotion and final deployment remain to complete.


## Final deployment and shared-service verification
Mail24 a42e9df0 is public and promoted. Workspace78 includes it in the offline
bundle and online distribution. Both real PCs completed normal backed-up updates
to manager78/Mail24 and reported healthy. Shared main URL persisted through manager
updates; normal secondary Mail entry opened the main Inbox automatically. Existing
local messages/settings were retained, not migrated; independent identities were
not merged. Main receiver's approved outer TCP25 route and matching eero address
reservation are saved; other receiver rules were preserved.
A fresh Gmail -> Mail -> Gmail round trip passed after both78 deployments. Exact
labelled test content was checked in both interfaces, with no bodies/artifacts
copied here. Main installed24 bytes match the public verified artifact. Advanced
stayed expanded across live network status changes. Router inspection still cannot
traverse this upstream hop;24 accurately distinguishes that from working delivery.
No passwords changed. C task build scratch/controller logs removed; current data
and newest verified rollback kept. Source/public channel/installer integration is
complete. See Workspace's October24 concurrent-sign-in handoff for the separate
portal-cookie collision found during final checking and fixed in manager78.
No clean-host, physical reboot, arbitrary-provider/router or production claim.

## October24: automatic per-server connection proof
User correctly reported that24 still exposed repair/router guidance on the main
service reached from the secondary. Wording alone did not resolve the state.
Mail25 correlates the existing external TCP probe's successful node, resolved
target and current attempt window with this engine's public SMTP connection-start
metadata. The bounded in-memory observer rejects unrelated, stale, LAN, other-port
and other-listener events. Only the narrow connection-start event is promoted
to info through native registry/reload; raw/debug payload logging stays disabled.
Verified proof takes precedence over unavailable router inspection, clears stale
error/details/actions and keeps healthy state during periodic monitoring. Failures
are re-evaluated every five minutes, never inferred from generic public TCP alone.
Existing routes, identities, shared mode and messages are retained.
Synthetic correlation/router regression and isolated native bootstrap/reload
checks passed; native fixture used named protected NTFS scratch with ephemeral
loopback ports. No live MyAccounts/FileBrowser fixture was used. Publication and
real two-PC deployment still pending at this source checkpoint.

## Live25 follow-up: independent incoming receipt evidence
Main79/Mail25 update completed healthy, but the external checker returned
inconclusive results while a fresh authorized Gmail message was accepted through
public SMTP. Connection metadata was enabled correctly; relying on one external
checker still produced false repair guidance. Mail26 additionally uses bounded,
recent native Internet-SMTP acceptance metadata for the exact configured domain,
keeping only domain/time in memory and restoring a protected 256KiB log suffix
after restart. Local submissions, another domain/receiver, old/future events and
explicit current failed probes cannot clear warnings. No message content is used
or transmitted. Real failed connections remain actionable. Both PCs still need
final26/bundled80 deployment and end-to-end verification at this checkpoint.

Mail27 completes the uncertainty UI: inconclusive checks/unavailable router
inspection are informational, not a repair alarm. Unknown external service state
shows Check again and automatic test-message recognition; technical router
controls remain under Connection details. Inbox does not show a broken-connection
banner for uncertainty alone, including after receipt evidence expires. Explicit
failed probes, known other receivers and DNS conflicts remain actionable. This
avoids a false warning returning merely because an external checker stays down.
Secondary79 local updater confirmed healthy completion (22m35s); public tunnel
reconnection is checked separately. Workspace80 remains an unpublished build and
will be rebuilt with final27 before its tag is created.

## Final public release and deployment running
Mail27 source37287b10 and Workspace80 source75251e9 are public with every asset
size/SHA256 verified, no Actions. Workspace public channel promotionf666d91 was
verified anonymously for80/27. Both current79/Mail25 PCs accepted exactly one
normal workspace+mail80/27 transaction. Main started2026-10-24T11:05:55Z; secondary
started shortly afterward. Do not replay either apply on tunnel disconnect.
Main is backing up; secondary downloads/checks. Earlier79 completed on both
(main10m18s, secondary22m35s); secondary tunnel returned afterward. RDP temporary
on-screen input is restored off, Run/temporary shell closed. Named C build/native
scratch is removed. Need final80/27 completion, main installed hashes/preserved
other binaries, fresh Mail UI without repair guidance, stable Advanced across
polling, shared-secondary entry and Gmail round trip before claiming completion.
A labelled Gmail test reached main Mail25 at10:35UTC despite inconclusive checker;
its native acceptance supplies recent evidence for27's protected startup restore.
No test body, credentials, capability or OAuth artifacts are copied here.

## Completed80/27 deployment and verification
Both PCs completed their normal backed-up Workspace80 + Mail27 updates healthy.
Main completed in8m41s; secondary in15m39s. Latest usable rollback retained on each;
one prior automatic copy removed (~4420.1MB main /3395.3MB secondary). Main installed
manager/Mail hashes match public release bytes. Other25 app records and their
referenced binaries match the pre-task snapshot, including the custom Drive.
Main C free space was about18GiB; all named task native/build scratch is removed.
Unknown failed/recovery data, scheduled backups and user files were preserved.

Main27 automatically reached verified state from a successful external node
matched to its own public SMTP connection. Browser settings show Incoming
connection verified, with no Repair/router warning or stale raw error. Advanced
and Connection details stayed expanded across the next real periodic check
(next-check time advanced). Fresh main Inbox opened signed in as the real owner.
A new authorised Gmail message arrived with its expected test content; Mail's
reply used the proper canonical address and was received/opened in Gmail.

After secondary completion/tunnel return, its normal Mail entry automatically
handed to the main Inbox, signed in, and opened the same final verification
message without a warning. Its Mail settings also show the verified main status.
A fresh secondary update check reports installed Workspace80 and Mail27 up to
date. Shared main setting persists; no local engine/router setup or independent
identity/mailbox migration was introduced. RDP input preference is restored off.
No passwords or new router rules were changed in this task. The earlier known
other-app metadata review issue remains outside this Mail change.

Final public artifacts: Mail27 source37287b10, Workspace80 source75251e9; promoted
channelf666d91 verified anonymously. All builds/publications local, Actions
disabled. Final27 synthetic correlation/receipt/uncertainty/router checks and
isolated NTFS native bootstrap/reload pass. No clean-host, physical reboot,
arbitrary-router/provider, mobile or production acceptance is claimed. There is
no remaining required80/27 implementation/publication/deployment work.
