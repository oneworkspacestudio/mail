# Automatic Mail domain setup

User requests compatible DNS reuse, automatic fallback subdomains, useful setup
controls, organization publication and installation. Based on public Mail17
(d6c9ea47), isolated worktree codex/mail-domain-setup. Original checkouts untouched.
Own organization fork only; upstream AGENTS forbids contacting Stalwart maintainers.
No upstream contact, GitHub Actions or automated tests in this task.

configureHosting preflights all record sets and reuses exact compatible existing
records without edits, recording Reused receipts. Conflicts are never overwritten.
Managed first-run/retry now automatically chooses an unused mail/inbox/post/mailbox
or numbered subdomain for an unpublished, single, manual native domain. Domain,
directory and account IDs remain intact; journal resumes interruptions. Existing
published domains and multiple/native-custom domains still require migration.
Other installations' occupied candidate subdomains are skipped even at same IP.
The apply method is shared by automation and explicit owner recovery, retaining
owner/Origin checks on HTTP. Renamed mailbox address assignments retain account
IDs and local names under the same protected journal, with provision lock.

Setup screen provides Finish automatic setup, Open Mail and collapsed DNS details,
with an inline action error and current mailbox domain. Secondary manual review
remains available. Native signing/certificates/network remain delivery requirements.
Windows compilation and JS syntax passed; no automated/native setup or external
email acceptance claimed. Publication/build/deployment outcomes pending.

Reported screenshot is workspace-2.m8e.co.uk, a different installation from this
PC's workspace.m8e.co.uk. Current PC has published inbox domain receipts, so retain
its domain; do not repurpose its DNS for the other server. Artifacts on I: under
Projects/tooling/mail-domain-20261023. No owner data used as synthetic fixtures.
