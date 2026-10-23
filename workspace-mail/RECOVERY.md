# Mail setup and recovery

## Mail setup after installing One Workspace

Mail prepares its standard access settings automatically. You do not need to
create or edit user roles. An earlier installer tried to connect the account
before the mail engine had created those settings. Update One Workspace and
Mail, then choose **Retry automatic setup** if setup was interrupted.

If an existing account will not reconnect, the Workspace owner can choose
**Reconnect Mail** in Mail setup. Messages and custom access settings are kept.
Startup shows a plain-language next step; detailed diagnostics stay in the
protected Mail and Workspace mail-setup logs for support.

## Sign-in works, but Mail reports access denied

MyAccounts authentication and permission to use a Stalwart mailbox are separate.
Earlier managed builds selected the identity directory without ensuring that
Stalwart's default ordinary-user role was linked. This left users with no
`authenticate` permission and produced HTTP 403.

Updated builds migrate engine schema 1 to 2 automatically. The migration restores
only a missing default User role link and verifies it was saved. It preserves
custom role assignments and does not grant administrator access. Mailbox traffic
continues to use each user's own MyAccounts access token.

1. Refresh Mail and sign in again after updating.
2. If access is still denied, the Workspace owner can open Mail settings and use
   **Reconnect Mail**. Other users should contact that owner.
3. If the standard User role has been changed or removed, restore/review that
   role first. The repair deliberately refuses ambiguous or elevated roles.

No DNS change is needed to repair mailbox sign-in. Configuration repair errors
remain visible in Mail settings and Workspace startup progress.

## Existing mail DNS detected

An MX record determines where Internet email for a domain is delivered. Existing
MX/SPF/DMARC or conflicting hostname records can belong to another mail service.
Mail preflights the whole DNS plan before creating records. Matching records are
reused unchanged. An unpublished installation automatically finds an unused
subdomain when the original domain belongs to another mail service. **Finish
automatic setup** retries that process; current and proposed DNS values remain
available under **DNS details and other options**.

### Use an available mail subdomain

The owner can also choose **Review a different mail address** in DNS details.
Workspace checks `mail`, `inbox`, `post`, `mailbox` and numbered mail names under
the connected Cloudflare zone, including SMTP and DMARC names. An existing
webmail CNAME or another installation's DNS is treated as occupied.
Review the offered domain and choose **Use @…**. Email addresses will end in
that subdomain; the webmail URL and original domain's MX records stay unchanged.

For a local domain that has not published DNS/signing/certificates yet, Workspace
renames the native domain in place and updates MyAccounts' username domain.
Domain/account IDs, message storage and folders are retained. A protected journal
resumes an interrupted change; **Resume mail-domain change** is available to the
owner. Previously published mail domains require a planned migration instead.

Keeping the records preserves the current routing, whose delivery health has not
been verified. Workspace does not automatically import that provider's messages.
Changing the delivery route affects everyone using the email domain. Plan mailbox
addresses/aliases, message migration, backup and cutover before changing records.
Changing an already configured mailbox domain also requires migration.

Internet hosting additionally requires SMTP reachability, certificates, signing
keys and appropriate reverse DNS. A working webmail page does not prove Internet
delivery. Cloudflare's normal web proxy does not proxy SMTP port 25; see
[Cloudflare's mail DNS guidance](https://developers.cloudflare.com/dns/proxy-status/use-cases/).

Support references: `mail_access_denied`, `mail_identity_rejected`,
`mail_unavailable`, `existing_mail_dns`. Never include credentials, access tokens,
setup links or message contents in a support report.

## Webmail opens but incoming messages do not arrive

Open Mail settings and review **Your Mail setup**. DNS and the web tunnel can be
healthy while public SMTP is unreachable. Mail checks this separately and retries
every five minutes; choose **Retry automatic setup** after a network repair.

On compatible routers, Mail maintains its leased TCP25 forwarding. For networks
with two routers, it also renews verified upstream rules. If a router still lists
a rule whose public connection fails, Mail can recreate its own exact rule and
probe again. A listed forwarding rule alone is not proof of connectivity.

If automatic recovery cannot verify ownership or the downstream target, the
warning identifies the upstream hop. In that router, forward TCP25 to the next
router, then forward it from the last router to the PC address shown by Mail.
Preserve routes belonging to another service. Allow the native mail engine in
Windows Security for the active network. If your ISP blocks TCP25 or uses shared
carrier NAT, arrange a public mail host or inbound gateway; an outbound relay
alone cannot receive messages. Do not disable the firewall to diagnose this.

After the status reports reachable, send a uniquely labelled message from an
external provider and confirm that it appears in the intended mailbox. Reachable
TCP25 does not prove delivery, TLS authentication, or spam placement. Previously
queued messages can take longer because the sending provider schedules retries.

## Chrome reports a dangerous or deceptive site

This requires investigating the website's Safe Browsing classification. SMTP can
continue receiving messages while a browser blocks a web sign-in. Mail uses its
own One Workspace identity and shows the configured MyAccounts server; it does
not represent itself as Google or ask for a Google account password.

Review the affected URLs in [Google Search Console's Security Issues report](https://support.google.com/webmasters/answer/9044101).
Check the deployed code and content, address every reported issue, then request
a review. If the public status report and browser disagree, the public report
alone does not establish that the warning is resolved. Google also provides a
[classification review form](https://safebrowsing.google.com/safebrowsing/report_error/).
Submit only public paths; remove OAuth codes, state values and other sign-in
parameters from example URLs. Google controls review and propagation timing.
Keep browser protections enabled while the issue is investigated.


## Automatic domain selection

Managed setup reuses exact compatible DNS records without changing their values.
Conflicting routes are preserved. For an unpublished single-domain installation,
setup automatically chooses a free subdomain and resumes the same journalled
domain change; mailbox IDs, folders and messages stay in place. Existing published
domains are not automatically renamed. A subdomain with another installation's
DNS is skipped rather than taken over, even when both use the same public IP.
The setup page offers Finish automatic setup, Open Mail and expandable DNS details.
Internet delivery still depends on SMTP reachability, certificates and signing;
successful DNS publication alone is not delivery verification.

## Incoming mail connection and stable settings

Mail checks the current network adapter's router directly as well as using
multicast discovery. It repairs rules belonging to this installation. For a
second router, it can create a missing rule only when the inner router reports
its private outside address and the destination can be verified. Compatible
existing forwarding is reused without deleting someone else's rule.

**Repair connection** checks only the incoming network route; it does not rerun
DNS or account setup. The page shows the current blocker and the next automatic
check. Advanced settings and connection details stay open during refreshes;
opening the settings form stops old status requests from replacing that form.

If the page says **Another PC already receives your email**, a different email
subdomain alone cannot solve it. Both subdomains reach the same public SMTP
port. Keep one shared mail receiver and configure its domain routing, or plan
to move incoming mail to the new server. Mail preserves the current receiver
until that decision is made. It cannot enable a router's restricted settings or
override an ISP block without permission or an accessible incoming gateway.
