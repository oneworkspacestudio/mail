# Mail access and hosting recovery

User reported HTTP403 after MyAccounts sign-in and vague existing-DNS warning.
Current task authorizes local diagnosis/fix, not GitHub publication. Preserve
the prior uncommitted hosting implementation and one-workspace setup changes.
See 2026-10-14-workspace-mail-compacted.md and one-workspace's matching history.

## Confirmed causes

Actual installation C:/tmp/workspace, Mail8996/engine8997, suite8990. Engine log
reported security.unauthorized, permission `authenticate`, account1. Private
recovery-authenticated read-only x:Authentication/get with ids singleton showed
the correct OIDC directory but ALL default role maps empty. The existing User
role (id b) contains authenticate and jmapMailboxGet; account1 has User/Inherit.
The wrapper selected the directory without ensuring the ordinary-user role link.

Cloudflare read-only inspection found MX m8e.co.uk -> m8e.co.uk, priority1,
unowned by this installation. Root A is proxied; public MX response uses
Cloudflare's generated _dc-mx alias. This alone does not establish whether the
old service is in use or working. No DNS records were modified. Do not silently
replace that route. User was asked whether to keep the existing service or plan
a move. Until answered, Internet mail remains paused intentionally.

## Fixes

- identity.go selects exact issuer/audience/sub/domain directory; schema2
  migration restores only an EMPTY defaultUserRoleIds with the unambiguous
  native standard User role. Validates its permissions against pinned upstream
  ordinary-user defaults (including self-scoped sys settings/password APIs),
  no inherited/admin roles. Existing nonempty custom policy is preserved.
- Read back the saved role link. Native mailbox requests still use user tokens;
  private recovery authentication is used only for configuration. No user was
  made administrator. Empty admin/group/tenant links were left unchanged.
- Owner/setup-authorized POST/api/identity/repair, normal CSRF and Origin checks.
  Migration failures keep the settings UI available with identityError instead
  of exiting before recovery can be reached. Suite startup surfaces a warning.
- Structured engine-access errors with support codes and persistent mailbox
  recovery view; sign-in again, retry, owner repair and settings actions.
- DNS preflight collects conflicts before any writes, returns hostingIssue with
  existing/proposed values; UI separates local configuration from Internet mail,
  explains keep/migrate choices, limits blind recheck, documents network/aliases
  and import prerequisites. No claim of working external delivery.
- RECOVERY.md plus README link; status exposes no credentials or message data.

## Local deployment and evidence

Actual installation was stopped via its normal control command after checking
manager PID/path, then updated and restarted. First repair build refused the
standard User role because it legitimately includes some self-scoped sys APIs;
the conservative validator was corrected against pinned permissions.rs. Final
role readback showed defaultUserRoleIds {b:true}, defaultAdminRoleIds {}. Suite
returned8/8 startup, all apps running, with actionable DNS warning.

Backups of old Mail.exe, workspace.json and encrypted Mail config are under
Private/backups/mail-access-fix-20261015, protected by inherited private ACLs.
These are changed-file backups, not a full mailbox/database backup.

Final local targets: Mail0.1.0-recovery-preview.5 and suite/installer
0.3.0-mail-recovery-preview.6. Artifacts are ignored dist/Mail-recovery-preview.exe
and one-workspace/dist/WorkspaceSetup-mail-recovery-preview.exe. See the suite
history for final deployment evidence. No tests added/run or emails sent. Go
compilation, JS syntax, read-only configuration diagnostics and rendered guidance
were used. The actual signed-in inbox has not been observed after restart;
owner must sign in again. Existing unrelated live installations untouched.
No commit/push/release/Actions. Mail service choice and full Internet deployment
remain outstanding; do not infer consent to migrate from this investigation.

## Follow-up: recommend an unused mail subdomain

User requested a separate subdomain and then confirmed the original root mail
may serve another project. Preserve it and recommend an available alternative.
This supersedes the earlier unanswered clarification and DNS-blocked state.

Added owner-authorized POST/api/hosting/subdomain/plan and /apply. Plan checks
mail/inbox/post/mail2 and each candidate's SMTP/DMARC names against exact
Cloudflare records. Existing records of any type are occupied, including a
webmail CNAME. UI explains address-domain vs webmail URL, the local rename,
preserved storage and external delivery requirements before Use @domain.

Apply is restricted to a single unpublished local managed domain with no DNS
ownership receipts and manual DNS/DKIM/certificate management. It validates the
exact MyAccounts directory and default domain before a protected journal, then
renames Domain in place, updates Directory.usernameDomain and SMTP hostname,
saves config, clears Mail sessions and restarts the native child. Domain/account
IDs and message/folder storage remain unchanged. Journal resumes on startup or
via Resume mail-domain change; failures leave recovery UI available and suppress
automatic DNS until the pending change is resolved. Previously published/live
domains are refused for this shortcut; they require planned migration.

Actual availability: mail.m8e.co.uk is occupied by the previous public preview's
Cloudflare tunnel CNAME; inbox.m8e.co.uk was unused. Applied inbox.m8e.co.uk using
the protected setup authorization. Native Domain b and Account b/domainId b
were retained; the same OIDC directory now uses inbox.m8e.co.uk. Root MX remains
m8e.co.uk -> m8e.co.uk, priority1, confirmed by provider readback.

Created only smtp.inbox A, inbox MX/SPF and _dmarc.inbox TXT with ownership
receipts. Native engine published a dedicated DKIM selector below inbox and
ACME DNS01 logs reached order-valid/order-completed; a native Certificate exists.
No old root/webmail DNS was modified. No mail was sent and external SMTP
reachability/send/receive remain unverified. Webmail stays at the existing
workspace.m8e.co.uk/mail/ URL; inbox is the email domain.

A stopped Mail data snapshot (excluding re-extractable native runtime and logs)
plus Mail.exe/workspace.json is under the actual installation's protected
Private/backups/mail-subdomain-20261015. No restore acceptance was performed.
Recovery button also restarts the native engine and waits for private config
readiness, avoiding cached permissions immediately after repair.

Final local build targets supersede earlier versions: Mail0.1.0-subdomain-preview.8
and suite0.3.0-subdomain-preview.9, same ignored recovery artifact filenames.
Still no publication or automated tests. Follow up on reachable SMTP25, PTR and
address aliases before claiming usable Internet mail. User must sign in again.

Final deployment confirmed Mail0.1.0-subdomain-preview.8 and suite health with all
apps running, hostingError/identityError empty, domain inbox.m8e.co.uk. Native
configuration and public portal health were observed; no signed-in mailbox or
external delivery acceptance claim. Current process PID is in running.json.
Localhost setup sign-in buttons now start OAuth at the configured public origin,
avoiding a state-cookie host mismatch. Final screenshot is in one-workspace's
docs/screenshots/mail-subdomain-ready.png; earlier DNS-conflict guidance capture
is retained alongside it. Matching installer/EXEs/checksums were rebuilt locally.
