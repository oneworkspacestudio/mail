# One Workspace Mail

## Request and repositories

User requested an independent Stalwart fork, a Gmail-style client with MyAccounts
and One Workspace integration, then explicitly authorized creating the web
hostname mail.m8e.co.uk in Cloudflare and a wizard for other users' settings.
Source fork: https://github.com/fahimc/stalwart (main), local I:/Projects/stalwart-mail.
Suite: I:/Projects/one-workspace. No GitHub Actions; both repos have Actions disabled.
Current request authorizes publishing these sources, not replacing existing EXE
releases. Existing mail/identity services and existing MX records are preserved.

## Implementation

New Go module workspace-mail implements Mail.exe, embedded static Gmail-style
client, and an embedded **unmodified official Stalwart 0.16.26 Windows engine**.
It does not claim a local Rust core build. Build script checks official ZIP
SHA256 5f8370b259330e9eba65ed21bd59af92cfecd9597aea17594d803ec68b4bc291.
Frontend: inbox, reader (plain text), compose/reply/forward, server search,
stars/read state/archive/bin/labels, drafts/autosave, attachments, submission.
This is a preview; no claim of full Gmail parity or externally delivered mail.

OIDC uses PKCE/state/nonce, exact issuer/client, HTTP-only sessions, CSRF and
server-held access/refresh tokens. Each user accesses Stalwart with their own
access token. Directory uses immutable sub@domain, no email-based privilege
linking, no IdP-group admin mapping. Both the domain's Directory and Authentication
singleton directoryId must select MyAccounts. Default singleton objects should
be fetched with ids:["singleton"]; querying all can return empty for defaults.
Stalwart session endpoint is /jmap/session; /.well-known/jmap redirects (307).
Listeners' protocol enums use lower-case http/smtp; variant @type names use Pascal.

Wizard: domain/origin/client credentials, Cloudflare DNS review/apply (request-only
token, ownership/collision checks, never overwrite), optional TLS465/STARTTLS587
relay configuration. No verification email is sent. DNS-only SMTP records are
separate from proxied web CNAME; public SMTP/TLS, DKIM, PTR and domain-provider
requirements still need the owner. Never silently replace existing mail hosting.

Native private data requires NTFS ACLs; machine-DPAPI stores config/recovery/setup
links. Embedded engine is hash-held, children contained in a Windows Job; unexpected
engine exit ends launcher for suite supervision. Bootstrap writes registry then
requires a recovery restart before adding our two loopback listeners, then normal
restart. Pending setup can resume only matching settings. Bootstrap/recovery's
upstream temporary HTTP bind uses [::] on the chosen high port, protected by a
random recovery credential; steady-state listeners bind loopback only. Logs filter
upstream auth events that can contain bearer tokens. Older development logs remain
private; do not read/print raw auth error details or commit runtime data.

## Current preview and evidence

Build cache/runtime: C:/Projects/.cache/workspace-mail; source/artifacts remain on I.
Fresh suite at http://localhost:19590; Mail /mail, Accounts19591, Drive19592,
Photos19593, Mail19596, Stalwart19597, local SMTP19598. Private suite state under
suite-private, files/programs under suite-files/suite-programs in that cache.
Synthetic owner@example.test only, password in suite-owner.dpapi. Existing live
MyAccounts/FileBrowser and prior Git/Music preview were not used or changed.
Fresh Windows setup, dedicated client registration, MyAccounts SSO, real empty
mailbox and compose rendering observed. Screenshot docs/screenshots/mail-compose.jpg.
Go compilation/local setup build and JS syntax compilation completed. No test
suites were added/run; external send/receive, draft/attachment mutation flows,
multi-user isolation and clean-host/service/reboot acceptance remain unclaimed.

Public web endpoint https://mail.m8e.co.uk is live via dedicated remotely managed
Cloudflare tunnel b0d87cfb-d656-422a-9a11-12f683534b69, loopback19496. Exactly one
new proxied CNAME was created after zone ownership and absent exact record checks.
No MX/SPF/root records or pre-existing tunnel changed. Public instance is a fresh
unconfigured wizard, NOT the synthetic signed-in suite. Its private state is
public-private in the cache; tunnel token DPAPI-protected there. Connector logs
and launch data stay private. Processes are current-session previews, no boot
persistence/service claim. Reopen private wizard with --data <public-private> --setup.
User must supply the dedicated real MyAccounts client and delivery settings.

## Integration / follow-up

One Workspace has optional -Mail bundle input, mail-manifest.ps1, /mail tile/route,
dedicated OIDC client/owner subject env and base+6/+7/+8 port reservation. Mail
must use the updated manager, which injects base and stable owner subject.
Suite notices include engine AGPL/provenance. Existing releases are unchanged.
Local WorkspaceSetup-mail-preview.exe uses baseline Accounts/Drive/Photos plus
Mail (Git/Music are optional build inputs); do not describe it as a complete
new release of every previously released component.
Future work: public owner wizard completion, SMTP/TLS/gateway/DKIM/PTR deployment,
friendly aliases, explicit functional acceptance if requested, mailbox lifecycle
and full Gmail features. Never send external messages without authorization.
