# Mail send confirmation and real outbound acceptance

User explicitly requested checking the work end to end and sending a test to
their specified Gmail address. This authorizes that test message and local
repair, not publication. Earlier Mail delivery acceptance was explicitly
unclaimed; this entry supersedes that gap for the tested outbound flow only.

## Confirmed fault and fix

The wrapper required exactly one methodResponses entry. Native Stalwart adds
an implicit Email/set response for EmailSubmission/set.onSuccessUpdateEmail
(pinned crates/jmap/src/submission/set.rs). Thus the message could be accepted,
moved to Sent and actually delivered while UI said Unexpected Stalwart response.
Native logs also showed Google250 for an earlier user-initiated message.

jmap.go now matches the requested method and call ID m, permits implicit Sent
updates, preserves primary errors/notCreated/notUpdated/notDestroyed, rejects
missing/malformed/mismatched results, and returns a warning when submission
succeeded but the implicit Sent update failed. The latter must not look like a
send failure inviting duplicate submission. Unknown submission results explain
checking Sent before retrying. app.js requires created.send.id and displays the
nonfatal warning when present. No permissions, DNS or mailbox identity changed.

## Tests and evidence

Added16 parser regression cases in jmap_test.go: primary+implicit responses,
ordering, empty implicit call ID, implicit method/update failure, primary errors,
rejected submission/update, malformed/missing/mismatched/duplicate replies and
HTML. go test ./... passed in the isolated build source; JS syntax and Go Windows
build passed. Initial source-root attempt lacked the ignored native ZIP; a
new missing brace was corrected before staging/tests. Pinned archive verified by
the normal build script, not replaced with a mock engine.

Actual C:/tmp/workspace, public Workspace Mail, Chrome/MyAccounts sign-in:
composed and clicked Send ONCE for synthetic reference WM-20261015-A1.
Compose closed successfully; the test appeared in Sent. Native engine trace at
09:18:10UTC on2026-10-15 records TLS1.3 to Google's MX, delivery.delivered250 and
delivery.completed. A targeted search in the recipient's actual Gmail found
exactly one test, with matching subject/body, timestamp10:18BST. Gmail placed it
in Spam, explicitly saying it resembles past spam. Show original reports SPF
PASS and DMARC PASS; no DKIM verdict/signature was observed. Do not claim inbox
placement, DKIM success, a proven spam cause, inbound delivery or other-provider
acceptance. No other messages sent, deleted or marked not-spam by the agent.

Private proof screenshot lives outside Git at
C:/Projects/artifacts/mail-send-20261015/gmail-received.jpg. No personal mailbox
messages, tokens or raw engine logs are included in repository evidence.

## Deployment and handoff

Mail0.1.0-send-preview.9 built from C:/Projects/.cache/mail-send-fix using the
existing official engine ZIP. Actual install changed after fresh exact manager
path verification and normal stop; backup changed EXEs/workspace.json under
Private/backups/mail-send-fix-20261015-101623 (not a mailbox/data backup).
Suite/installer0.3.0-mail-send-preview.12 retains Drive drive-upload-preview.3,
Photos photos-upload-preview.2 and prior Mail/hosting/identity configuration.
Startup ready8/8, Mail status has no identity/hosting error. No separate existing
live MyAccounts/FileBrowser installations touched. No commit/push/release/Actions.

Ignored local Mail artifact workspace-mail/dist/Mail-send-preview.exe; installer
details in one-workspace's matching history. Preserve earlier uncommitted Mail
recovery/subdomain work and concurrent Drive styling changes. Remaining work is
deliverability evaluation (especially missing DKIM and IP/domain reputation),
friendly sender aliases, and inbound acceptance if separately requested.
