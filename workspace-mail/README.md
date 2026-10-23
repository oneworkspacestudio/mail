# Mail

> **Workspace managed releases use the private Mail backend.** A separate
> Calendar/Mail/Contacts shared-backend build is documented in the
> [shared Windows setup guide](docs/SHARED-BACKEND.md). Do not replace one
> storage layout with the other without an explicit migration.



A native Windows mail app with a Gmail-style interface and a real Stalwart
JMAP backend. Sign in with **MyAccounts**, use it independently, or add it to
[One Workspace](https://github.com/oneworkspacestudio/one-workspace).

![Mail inbox and compose panel](docs/screenshots/mail-compose.jpg)

## Included

- Inbox, reader, compose, reply/forward, server search, stars, read/unread,
  archive, bin, custom labels, Sent and Drafts.
- Autosaved drafts, attachment upload/download (25 MiB upload limit), and
  submission through Stalwart's authenticated sending identity.
- MyAccounts authorization-code sign-in with PKCE, state and nonce checks.
  OAuth credentials and access/refresh tokens stay on the server. Each user's
  own token is used for Stalwart, which enforces mailbox access.
- A setup wizard for domains, a dedicated MyAccounts application, Cloudflare
  DNS planning/creation, and a TLS SMTP relay (465 or STARTTLS 587).
- One EXE carrying the Windows engine. It extracts and supervises its child
  locally. No Docker, Node, Go or Rust installation is needed to run it.

This is a preview, not complete Gmail parity: it does not yet include offline
mail, snooze, Gmail category classification, scheduled sending, conversations
merged into Gmail-style thread cards, rich HTML composition or calendar/contact
apps. Messages are displayed as plain text; remote content does not run.

## Use Mail from several PCs

Keep one main PC to receive and store email. Other Workspace installations can
open **Mail settings → Use an existing Mail service** and connect to it. There
is no fixed PC limit; capacity depends on the main server. It must be running.

Fresh managed setup checks for an existing main service before creating local
mailboxes. It reads an optional `_oneworkspace-mail.<zone>` TXT record containing
`oneworkspace-mail=https://your-main-workspace/mail/`, then checks the standard
`workspace.<zone>/mail/` address. A reachable service is offered for owner review.
For custom addresses or an unavailable main PC, enter its Mail HTTPS address in
the same screen. The owner can explicitly save an offline address; the app shows
that it is unavailable and retries instead of pretending mail is ready.

After connecting, opening Mail automatically checks and opens the main service.
This PC's private engine and incoming-router checks stop. Local messages/settings
are retained, not migrated. **Mail settings** at `/mail/setup` lets the owner
change the address or resume the separate local server. Everyone uses accounts
on the main installation; separate MyAccounts issuers are never merged by email.
Discovery uses public HTTPS metadata without credentials or OAuth tokens. Existing
Workspace sign-in protection is retained; when Mail metadata requires sign-in,
the public Workspace health response confirms only that its Mail process is
running. This availability check does not prove message delivery.

One public SMTP port cannot forward independently to multiple PCs just because
their email subdomains differ. Independent stores need a routing/queue gateway
or separate public receiving endpoints. Existing receiver rules are preserved.

## Build locally

Use Windows x64 with Go 1.27 or newer:

```powershell
cd workspace-mail
./scripts/build.ps1
./dist/Mail.exe
```

The script downloads the pinned official Stalwart Windows archive and verifies
its SHA-256 before embedding it. No GitHub Actions are used. See NOTICE.md for
engine provenance and licenses.

Default private state: `%LOCALAPPDATA%/OneWorkspace/Mail` on NTFS.
The browser opens a private first-run link automatically. For unattended runs:

```powershell
./dist/Mail.exe --data C:/MailPrivate --port 19496 --no-browser
```

The protected setup URL is stored in `setup.dpapi`, not printed into logs.
An already configured One Workspace owner can also open Mail's setup screen.
To reopen a standalone instance's wizard, run
`Mail.exe --data C:/MailPrivate --setup` on its Windows host.
Configuration and recovery credentials are machine-DPAPI protected and guarded
by NTFS ACLs. Back up the full private data directory with Stalwart stopped;
mail lives in its ordinary Stalwart blob/metadata store on that disk. Moving
encrypted credentials to another Windows machine requires reconfiguration.

## MyAccounts

In standalone mode register a **dedicated** OIDC client with this exact callback:
`<Mail origin><base path>/auth/callback`. Enable authorization code, openid,
profile and email, and MyAccounts' existing sign-in session. Enter its issuer,
client ID and secret in Mail's wizard. One Workspace registers and supplies
these automatically.

Mail keeps the immutable OIDC `sub` as its internal account key, preserving
mailbox ownership and stored messages. Public addresses are readable:
**admin@your-mail-domain** for the Workspace owner and
**username@your-mail-domain** for other users. The compose window shows From.

One Workspace checks its MyAccounts organization every 15 seconds and creates
mailboxes before users first open Mail. Usernames are normalized to a safe email
local part; collisions receive `.2`, `.3`, etc. Reserved administrator/system
names cannot be claimed by ordinary users. Existing addresses and aliases are
preserved, and username changes keep the assigned email address. Existing
occupied admin addresses require explicit migration and are never taken over.
Protected per-install assignments prevent address reuse after user removal.
MyAccounts contact/recovery email fields are not changed.

Standalone OIDC Mail provisions the signed-in user on first login; background
provisioning requires the Workspace supervisor. Provisioning uses a rotating
local-only credential, separate from user-token mailbox operations. Settings
show native provisioning errors; Workspace records directory synchronization
status in private `mailbox-sync.json`. Disabled/deleted users are not newly
provisioned; mailbox deletion and retention remain explicit administration.
No account is linked or granted administrator rights by matching contact email.

With no OIDC connection, ordinary existing Stalwart mailbox credentials can be
used for standalone sign-in. Recovery administrator credentials are isolated
from that login form and retained in protected `engine.dpapi` for local engine
administration. The wizard initializes an internal directory when OIDC is blank;
create ordinary mailboxes through Stalwart administration before signing in.

## Automatic hosting from One Workspace

The published One Workspace installer has a domain-first
connection mode. Enter the domain, connect Cloudflare, and install. It generates
the MyAccounts client/callback, tunnel and web addresses without asking for
issuer URLs, client secrets, account IDs or zone IDs.

When included in that installer, Mail receives its domain and credentials from
the protected suite runtime and configures itself on first launch. It discovers
the public IPv4 using HTTPS at api4.ipify.org, publishes a DNS-only SMTP A record,
MX, SPF and initial DMARC, and enables Stalwart's automatic DKIM and DNS-01 ACME
certificate renewal. Signing selectors use an installation-specific prefix.
Existing mail records stop automation rather than being replaced. Managed record
IDs are saved for safe retries. The DNS credential is kept in DPAPI and passed to
the engine as an environment secret for renewal.

Mail shows progress and any hosting blocker instead of a second technical wizard.
Advanced settings remain available. **This configures hosting; it does not prove
Internet delivery.** TCP 25 must reach this computer, Windows/router/ISP policy
must permit it, and reverse DNS is controlled by the server provider. Firewall
policy, reverse DNS, gateways and delivery-provider accounts are not provisioned
automatically. Compatible routers receive a leased SMTP forwarding rule, renewed
while Mail runs; existing rules are preserved. Public SMTP is enabled only for
this managed hosting mode. Certificates and DKIM are background Stalwart tasks, so completion is not
claimed merely because their settings were saved. Use a server with a reachable
mail connection, or provision a gateway/relay separately when the network blocks
SMTP. The HTTP tunnel carries the web interface only.

This does not change existing configured installations or create mailboxes from
contact email addresses. Domain migration remains an administrator operation.

## Advanced standalone Cloudflare and Internet mail

`mail.m8e.co.uk` is the requested webmail hostname. A hostname alone does not
activate Internet delivery. The wizard supports another user's domain as well.

1. Choose the public HTTPS web origin and the actual email domain separately.
2. Connect a Cloudflare tunnel to Mail's loopback web port (or use your HTTPS
   reverse proxy). Start standalone Mail with `--origin https://mail.example.com`
   so its allowed Host and OAuth callback match the public URL. One Workspace
   uses the suite's configured HTTPS origin and `/mail` path instead.
3. Review the DNS plan before applying. Supply a zone-scoped API token with
   Zone Read and DNS Edit. Mail uses it only for that request, never persists it,
   and never replaces existing records. Known collisions reject the whole plan.
   API failure mid-creation reports partial progress for manual reconciliation.
4. Configure an authenticated TLS relay in the wizard if outbound SMTP is
   blocked. Saving credentials configures Stalwart without sending a message.
   Use your provider's required SPF/DKIM records instead of the direct-MX SPF
   plan when sending through a relay.
5. For receiving, run a reachable SMTP service on TCP 25 with a valid mail TLS
   certificate, a DNS-only SMTP hostname and its MX. Configure these in Stalwart
   administration or use a mail gateway. Cloudflare's HTTP tunnel does not
   transport normal Internet SMTP. The standalone/manual engine exposes only loopback
   HTTP and local SMTP, not public SMTP/IMAP/submission ports.
6. Publish Stalwart's generated DKIM keys and configure reverse DNS with the
   SMTP host provider. Review existing MX/SPF before any migration.

The DNS wizard's optional public IP creates `smtp.<email-domain>`, MX, initial
SPF and DMARC only; it does not configure firewall rules, TLS or reverse DNS.
It does not overwrite a root domain's existing mail service. A web hostname
that is a CNAME may require a separate email domain / DNS configuration.

## One Workspace

Use `scripts/mail-manifest.ps1` from the One Workspace repo to generate a
manifest for this exact Mail.exe, then stop the suite and run `add-app`.
Mail gets its own tile and `/mail` route. Use the updated One Workspace manager
from the same source revision as that integration, so it supplies the owner
subject and path environment.

Alternatively, pass `-Mail <path-to-Mail.exe>` to One Workspace's local bundle
build script. The installer then includes Mail and registers its OIDC client.
Mail uses base port +6; its engine and SMTP use +7 and +8. Reserve those ports.

## Scope of validation

Local Windows compilation, fresh isolated setup and live preview are tracked
in `agents/history/`. No Internet messages are sent during setup. This preview
has not completed clean-host, reboot/service, external delivery or multi-user
acceptance testing. Do not infer those claims from a running inbox.

Upstream's [agent/support policy](../AGENTS.md) permits local fork work but asks
that support and proposed contributions go through support.stalw.art. It does
not accept AI-generated upstream issues, pull requests or security reports;
human security reports require personal reproduction and its SECURITY.md process.

## Setup recovery

See [Mail setup and recovery](RECOVERY.md) for mailbox access repair, exact DNS conflicts and planned migration guidance. The updated UI separates mailbox access from Internet email hosting.


### Incoming mail during automatic installation

An HTTPS tunnel makes the web UI reachable; SMTP uses public TCP25 separately.
Managed hosting checks DNS and discovers a compatible UPnP Internet gateway on
this PC's LAN. It creates a leased TCP25 rule only when that port is unused,
reads the rule back, and renews its own rule every five minutes while running.
Existing forwarding to other servers is preserved. A matching manually managed
rule is preserved too. Router leases expire after the app stops.

Setup also checks public TCP25 using Check-Host (only the public SMTP hostname
is shared; no credentials or email content). An unavailable or inconclusive
probe remains unverified. The installer shows Mail warnings and actionable
network guidance while other Workspace apps remain usable. Checks retry every
five minutes. A reachable connection does not prove message delivery or inbox
placement. The installed Mail UI exposes the current result under Your Mail setup.

If automatic forwarding is unavailable, forward router TCP25 to the Windows
PC shown in Mail, allow the native mail server through Windows Security on the
active network, and check your ISP permits incoming SMTP. Shared ISP NAT/double
NAT needs an upstream forwarding rule, public address, or inbound mail gateway.
An outgoing relay alone does not fix incoming email. The installer does not
silently alter Windows firewall policy or overwrite another server's port rule.

On Windows, bounded route discovery also checks private router hops beyond the
LAN gateway. Supported upstream UPnP rules are renewed only when their existing
TCP25 target and installation ownership can be verified. Older Mail rules can be
adopted when this PC's matching legacy rule proves the migration context. Each
installation then uses a distinct rule description. If a public connection fails,
Mail recreates only its verified rules and checks public reachability again.
Manual and other installations' rules are preserved. Mail refreshes the displayed
incoming status every 30 seconds while open, without discarding an open draft.

Some routers block discovery, do not support UPnP, or require manual forwarding.
When the downstream target cannot be established, Mail explains which upstream
hop needs attention. It cannot bypass ISP SMTP blocks or shared carrier NAT.

Published Windows releases include `workspace-mail.json`, declaring the supported
Workspace integration version, executable name and product version. Workspace's
online installer uses this manifest and GitHub's asset digest to resolve Mail
directly from `oneworkspacestudio/mail`; it fails if the newest release is invalid
or incompatible rather than silently using an older binary. Build locally with
`scripts/build.ps1`; no GitHub Actions are required.

### Local build modes and frontend checks

The ordinary Go build embeds the pinned Windows engine and defaults to private
storage. `WORKSPACE_MAIL_BACKEND=shared` or an explicit `--shared-data` selects
the shared service. `scripts/build-shared.ps1` uses the `sharedengine` build tag
and defaults to the external shared engine. An explicit private configuration
keeps existing private storage; opening an old installation never migrates mail
implicitly. See `docs/SHARED-BACKEND.md` before planning a migration.

Run compose regression checks with `cd tests`, `npm install`, then `npm test`.
The pinned DOM dependency is development-only; the installed executable needs
no Node runtime. These checks exercise attachment retention, reopened drafts,
HTML-only forwarding and CC-only submissions using a stubbed JMAP transport.
They do not prove browser sign-in or real email delivery.


Automatic router inspection and message delivery are separate checks. An upstream
router may reject inspection even when its manually configured forwarding works.
Mail explains that distinction and preserves a working connection; it does not
ask people to enable UPnP again merely because inspection is unavailable.
