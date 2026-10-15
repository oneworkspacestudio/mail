# Readable addresses and automatic MyAccounts mailbox provisioning

User objected to UUID-based sender and requested admin mail at setup plus a
mailbox for every new MyAccounts account. Keep immutable subject binding; do not
switch mailbox ownership based on contact email or overwrite an existing mailbox.
Previous send-repair/end-to-end verification request remains relevant. No current
commit/push/release/Actions authorization and no new external email sent here.

## Implementation

- New workspace-mail/provision.go keeps native account.name=OIDC sub/domain,
  assigning a public alias admin@domain to the configured Workspace owner and
  normalized username@domain to ordinary users. Collisions use .2/.3; reserved
  names cannot be allocated to ordinary users. Existing admin collisions fail
  visibly. Native account IDs, roles, aliases and stored messages are preserved.
- New accounts use the ordinary native User role and no password credential.
  Native x:Account/get/set readback confirms address ownership before recording
  protected mailboxes.dpapi assignments, scoped to issuer/client/domain.
  Assignments survive username changes and are not recycled on user removal.
  IdP contact/recovery email is never changed. Native aliases/credentials are
  numeric-key objects, not arrays; list indexes can be compacted by native storage.
- Local-only POST/api/mailboxes/provision requires the supervisor's rotating
  random Bearer token, loopback peer/local Host and no forwarded request headers.
  Normal Origin checks still apply. Private recovery auth only configures native
  accounts; all mailbox message operations still use each user's token.
- Callback prepares any missing mailbox using verified sub and preferred_username
  (same-sub userinfo fallback), keeping standalone OIDC usable on first login.
  Session returns own mailAddress, UI chooses its native sending identity, shows
  read-only From and readable address in sidebar. It refuses UUID fallback sending
  while a managed address is unprepared. Existing sent headers are preserved.
- One Workspace owns background directory synchronization every15s; its native
  MyAccounts admin API is already used for installer client setup. Poll is scoped
  to organization myaccounts, paginates100 users, skips disabled/deleted users,
  reuses the authenticated session and catches users created while Mail was down.
  New users do not need a Mail login first. See suite matching history.

## Verification

Existing16 send-parser cases,6 local-part cases,5 authorization cases and opt-in
native integration pass. Fresh NTFS fixture uses the pinned official engine:
new accounts, existing owner adoption, preserved account ID/legacy alias,
collisions/reserved names, repeat sync and username change stability; no Admin
mailbox role granted. Native SMTP accepts the new user's recipient before any
Mail login, then QUIT before DATA (no message sent). Fixture uses its own loopback
unprivileged listener with test-only inbound auth requirement disabled to model
port25. Production submission/auth settings were never changed.

Early checks exposed expected bootstrap admin-name collision and incorrect
array assumptions for native List properties; fixed before deployment. Alias
preservation assertion was corrected to compare contents rather than an index
native storage compacts. SMTP probe initially used authenticated submission
semantics; corrected only in the isolated fixture. Final go test ./... passes,
JS syntax and Windows compilation pass. Suite fixture confirms a newly added
IdP user is synchronized without a Mail session; no test user was created in
the actual MyAccounts installation. No tests against separate live installations.

## Deployment / resume

Mail0.1.0-mailbox-preview.10 and suite/installer0.3.0-mailbox-preview.13 installed
at C:/tmp/workspace after normal verified manager stop. Stopped Mail data snapshot
(excluding re-extractable runtime/log) plus changed EXEs/workspace.json under
Private/backups/mailbox-addresses-20261015-103639. No restore acceptance performed.
Files, root DNS/other project mail route, existing messages and identities retained.
New manager initial PID60272; use running.json for fresh process identity.

Actual private mailbox-sync.json reports ready/1 connected user; public SSO
observed Fahim account and admin@inbox.m8e.co.uk sidebar/Compose From. Existing
test Sent message remains visible. Startup ready8/8; native status has no identity
or provisioning error. Empty inspection composer discarded; no new mail sent.
Proof outside Git: C:/Projects/artifacts/mailbox-addresses-20261015/admin-from.jpg.

Mail build source C:/Projects/.cache/mail-mailbox-fix; suite packaging source
C:/Projects/.cache/photos-upload-suite. Ignored artifacts Mail-mailbox-preview.exe
and WorkspaceSetup-mailbox-preview.exe/Workspace-mailbox-preview.exe/checksums.
Prior Drive drive-upload-preview.3 and Photos photos-upload-preview.2 retained.
Preserve all prior uncommitted Mail/suite fixes and concurrent Drive styling.

Still outstanding from prior task: unsigned DKIM/inbox-placement evaluation,
external inbound acceptance and other-provider delivery. Do not infer those from
address provisioning, native RCPT fixture or the earlier Gmail Spam delivery.
