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
