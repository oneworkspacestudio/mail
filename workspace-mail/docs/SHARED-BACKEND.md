# Shared backend preview

[My Calendar](https://github.com/oneworkspacestudio/calendar) supplies the shared
service manager and pinned separate Stalwart executable. Mail, Calendar and
Contacts each have their own executable and can be installed in any order.
Calendar does not require Mail.

Download and extract the Calendar Windows portable package. Run Mail.exe from
that package, or build this source with:

```powershell
./workspace-mail/scripts/build-shared.ps1 -SharedPackage I:/Downloads/CalendarShared -Go go
```

The helper verifies the shared manager and engine against the package checksum
manifest and builds Mail.exe. It never embeds Stalwart. Install the resulting
program alongside the shared package, or use its install-app.ps1 helper with
`-App mail`. Pass the same `--shared-data` to all apps in an installation. The
first app's wizard asks an owner for one dedicated MyAccounts OIDC client and
issuer; register the exact callbacks for all installed apps on that client.
Ordinary users then sign in with MyAccounts. See Calendar's technical guide.

Shared-mode Mail delegates all backend startup, health, recovery and lifetime
to WorkspaceServices.exe. Closing or unregistering Mail never stops Stalwart
or deletes user data. The native user token, rather than a manager credential,
is used for JMAP. Immutable issuer/subject identity owns each account. Native
user permissions remain in force. App installation enables no SMTP/IMAP ports.

The common client is audience-checked by native Stalwart. Mail does not receive
the shared recovery password or manage shared hosting through its older wizard.
Friendly public email aliases, external mail delivery and legacy mailbox
migration are not accepted by this preview. Initial shared mail identities use
immutable subjects at the installation's private domain.

Existing private-engine folders and external Stalwart services are refused,
not adopted or modified. Existing published Mail installations continue using
their old executables. `--legacy-engine` is an explicit compatibility mode for
an administrator who supplies its separate official engine. Do not point new
shared mode at existing legacy storage. Back up before planning a migration.

Native acceptance in the Calendar repository launches this standalone Mail.exe,
completes synthetic MyAccounts-compatible OIDC/JMAP, checks per-user isolation,
and confirms the shared service survives Mail exit. No existing installation,
public MyAccounts server or external email was used. No GitHub Actions.
