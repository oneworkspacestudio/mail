# Mail compose and portable-mode fixes

Clean fast-forward to studio/main preceded these uncommitted local changes.
Four synthetic DOM/JMAP checks now pass: forwarded attachments retained,
reopened draft attachment rows rendered/removable, HTML-only original text
included, and valid CC/BCC-only submissions accepted. Replies do not autoattach
source files. Go tests and JS syntax checks pass. tests/package-lock.json pins
the development-only DOM dependency; no Node runtime needed for installation.

Private builds embed the pinned Windows engine by default; sharedengine tag
uses an external engine. Explicit catalog/private configuration keeps existing
private data. --shared-data opts into shared mode without silent legacy adoption.
Initial shared-default startup refusal exposed the compatibility problem; fixed
before acceptance. Existing mail data was not migrated. Private startup and
incoming TCP25 health pass; health is not delivery/inbox-placement proof.

Login includes browser-warning recovery guidance. User screenshot confirmed
Safe Browsing phishing classification at the Accounts authorization path.
Approved official Google review submission succeeded; no removal decision.
No warning bypass or email sent this task, and live Mail browser acceptance
remains blocked. Do not substitute synthetic checks for real delivery proof.
Local builds applied to authorized suite; no commit/push/release.
