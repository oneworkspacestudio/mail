# Mail fresh setup and nontechnical recovery

User reported ordinary User role missing immediately after installing Workspace.
Reproduced exact error with pinned official Stalwart0.16.26 in a fresh NTFS fixture:
bootstrap/recovery has no Role records. configureMail asked configureIdentity for
the ordinary User role before the first normal start initialized engine defaults.

Fix: after installing this installation's explicit listeners, start normally to
initialize native roles/authentication, then connect MyAccounts and verify/save.
No duplicated permission lists or grants of administrator permissions to mail users.
Retry exposed a second bug: listener name is read-only on updates. Omit name from
patches while retaining it for creation, preserving listener IDs during resume.

configureIdentity logs diagnostics privately and wraps public errors with plain
Reconnect Mail guidance. Status hides raw identity errors. UI actions renamed
Reconnect Mail; messages/custom access remain intact. Suite companion startup
maps technical failures to plain next steps and saves details in protected logs.
Synced shared blue Mail SVG/Windows resources; corrected a pre-existing obsolete
brand-test expectation (it expected hub artwork after app branding had changed).

Go all tests and vet pass. Opt-in real engine tests cover fresh setup, interruption
before defaults and after defaults, retry without replaced roles, signed owner
and ordinary-user token mailbox access, per-user account isolation, admin-role API
denial, wrong audience refusal, and stable mailbox assignments after restart.
Existing provisioning/SMTP RCPT-only fixture passes; no external mail sent. JS
syntax, identity/status wording and four synthetic compose regressions pass.
Custom default role policy is preserved by a dedicated configuration/API test.

Started clean worktree from current studio/main ca2bf240; original I checkout has
unrelated Linux platform edits which are untouched. Own organization fork only;
no upstream Stalwart contact/issue/PR. No Actions. Artifacts under
I:/Projects/tooling/mail-setup-20261021; NTFS fixtures/GOTMPDIR under named
C:/Projects/.cache/mail-setup-20261021. Mail17/Workspace60 build, native full-suite
upgrade/UI acceptance, backed-up local delivery and publication to be recorded.
