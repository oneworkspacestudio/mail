# Incoming SMTP across nested routers and current organization Mail

## Scope and diagnosis
User requested repair of the installed Mail, Gmail delivery verification, fixes in
oneworkspacestudio/mail, and the organization Workspace installer using that Mail.
Authorized target is the dedicated Workspace installation; unrelated live account
and file services were preserved. No GitHub Actions or upstream issue/PR used.

A nested NAT's upstream rule was present but stale. Recreating only the LAN rule
did not restore external TCP25. Refreshing the exact matching upstream rule did.
The mesh gateway reports the outer public IP despite the extra router, so the
previous public-IP-only check missed the upstream renewal requirement. External
probes then connected and an authorized synthetic Gmail message was visibly
received in the intended Mail inbox. A previously queued message retried too.
Do not put private message contents, account IDs, credentials or router captures
in this repository.

## Implementation
Windows bounded hidden system tracert discovers up to three private route hops;
UPnP discovery is restricted to the local subnet or the exact observed private
hop, numeric same-peer HTTP endpoints, supported WAN service and no redirects.
Existing upstream targets must be private, in the router's subnet, enabled on
TCP25 and owned by this installation. Unknown/manual/other service rules stay
untouched. Legacy adoption requires a matching local legacy rule; subsequent
ownership uses a stable per-install hash and a short router-compatible name.
Some routers retain old descriptions on Add; exact legacy rules are recreated
for adoption. Public connection failure can refresh only verified owned rules.
Renewal/probe retries every five minutes; bounded Add retries handle transient
router responses. The UI refreshes live incoming status every 30 seconds without
discarding drafts. Unmanaged upstream targets, ISP blocks and CGNAT need explicit
network guidance; a TCP probe alone does not prove message delivery.

Windows build script emits workspace-mail.json (schema1/integration1/version/asset)
for Workspace's independently verified organization Mail release resolution.
Internal Go module identity retained; pinned official native engine unchanged.

## Verification / publication
Go test ./... passes on NTFS staging with the pinned engine payload; JS syntax
check passes. Tests cover private hop parsing, upstream ownership/legacy and
conflicting targets, changed-rule preservation, renewal/readback and recovery.
Live testing exposed a router description truncation limit; descriptions now fit
within 31 characters. Mail v0.1.0-preview.13 staged locally, with SMTP renewal and
post-deployment delivery being checked before release. Update this section after
publication and the lease-duration observation completes.
Final deployed build confirms public TCP25 reachable and both router hops renewed.
Release workspace-mail-v0.1.0-preview.13 is being published from the matching
source. Binary SHA256 4ab76b887b0d33aa9b2a6b062b8ccb4f00dc96a87f5fc282c2d81ec4b889a44e.
Local go vet also passed. Lease-duration observation remains pending.
