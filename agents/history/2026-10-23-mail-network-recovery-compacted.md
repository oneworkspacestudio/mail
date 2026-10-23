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
