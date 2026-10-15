# Mail setup and recovery

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
   **Repair mailbox access**. Other users should contact that owner.
3. If the standard User role has been changed or removed, restore/review that
   role first. The repair deliberately refuses ambiguous or elevated roles.

No DNS change is needed to repair mailbox sign-in. Configuration repair errors
remain visible in Mail settings and Workspace startup progress.

## Existing mail DNS detected

An MX record determines where Internet email for a domain is delivered. Existing
MX/SPF/DMARC or conflicting hostname records can belong to another mail service.
Mail preflights the whole DNS plan before creating records. The updated screen
lists current and proposed values and offers guidance for keeping or migrating
the existing service. **Recheck domain setup** cannot migrate existing mail.

### Use an available mail subdomain

The owner can choose **Find an available mail subdomain**. Workspace checks
`mail`, `inbox`, `post` and `mail2` under the connected Cloudflare zone, including
the SMTP and DMARC names. An existing webmail CNAME is treated as occupied.
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
every five minutes; use the incoming connection retry control after a repair.

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
