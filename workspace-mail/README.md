# One Workspace Mail

A native Windows mail app with a Gmail-style interface and a real Stalwart
JMAP backend. Sign in with **MyAccounts**, use it independently, or add it to
[One Workspace](https://github.com/fahimc/one-workspace).

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

Mail uses the stable OIDC `sub` as the mailbox local part, at the selected mail
domain. Contact email edits cannot switch a user into another mailbox. These
generated addresses may be long; manage friendly aliases in Stalwart. Never
link an existing mailbox or grant administrator roles solely by matching email.
External identity creates the mailbox on first authentication; provision it
before expecting inbound mail. Disabling/deleting MyAccounts users does not
automatically delete their stored mail. Stalwart administration manages aliases,
retention, suspension, account provisioning and protocol credentials.

With no OIDC connection, ordinary existing Stalwart mailbox credentials can be
used for standalone sign-in. Recovery administrator credentials are isolated
from that login form and retained in protected `engine.dpapi` for local engine
administration. The wizard initializes an internal directory when OIDC is blank;
create ordinary mailboxes through Stalwart administration before signing in.

## Cloudflare and Internet mail

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
   transport normal Internet SMTP. The initial engine exposes only loopback
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
