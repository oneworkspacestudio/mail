# Shared Stalwart Calendar integration

User requested the new oneworkspacestudio/calendar app and one managed shared
Stalwart for independently installable Calendar, Mail and Contacts. Work used
an isolated codex/shared-stalwart worktree to preserve unrelated concurrent
changes in the primary checkout. Do not reset or stage those primary changes.
No live installation, external mail/DNS or existing identity store was changed.
No GitHub Actions; local Windows builds only.

Calendar owns the reusable Go shared SDK, standalone service manager, pinned
separate official engine, owner setup, native install-order/OIDC/DAV/JMAP tests,
recovery and unregister-without-stop lifecycle. Public preview SDK tag is
v0.1.0-preview.1. The native acceptance checks all six app orders and all six
removal orders, backend crash recovery, offline backup/reapply/restart, user
isolation and standalone Mail OIDC. Synthetic identities and fresh NTFS stores
only. Package/source/checksum links and limitations are in Calendar's README.

Mail source delegates shared lifetime, refuses silent legacy adoption, drops
embedded engine and manager credentials from shared mode. Workspace opts into
shared mode only when both verified shared packages are in the catalog; it
starts services before dependents and registers one shared OIDC audience with
all three callbacks. Existing legacy catalog mode stays compatible.

Unit suites and Windows compilation pass. The complete new Workspace installer
bundle, actual public MyAccounts, clean host/reboot and external mail have not
been accepted. Calendar has Google-inspired layout, not full Google features.
Finish publication with normal main pushes after fetching; do not force push.

## Published

Calendar SDK/source tag v0.1.0-preview.1 and unsigned Windows portable release:
https://github.com/oneworkspacestudio/calendar/releases/tag/v0.1.0-preview.1
All14 uploaded asset sizes/digests verified; public portable download returned200.
Its source manifest pins Mail48f80639 and Workspace8d934a1. Both integrations are
on their public main branches after normal rebase/push, preserving newer remote
work and unrelated primary checkout changes. No force push or GitHub Actions.

Final GOWORK=off suites pass with the published Calendar module/checksum; Windows
Mail and Workspace compile. Final Mail.exe native OIDC/JMAP acceptance passes and
exit retains Stalwart. The portable release supplies shared Mail.exe and exact
Mail source; no new Mail-only or all-in-one suite release was published. Existing
published installers/live apps remain unchanged. Later handoff-only commits do
not change the released source implementation. Keep all documented preview and
legacy migration boundaries when preparing a future full suite package.
