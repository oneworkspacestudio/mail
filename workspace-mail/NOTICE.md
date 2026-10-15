# Component provenance

One Workspace Mail is an independent Gmail-style mail client in the
`oneworkspacestudio/mail` fork. It is not a Google product and is not affiliated with
Google or Stalwart Labs. The interface is an initial implementation, not a
claim of complete Gmail feature compatibility.

The launcher and web client are AGPL-3.0-only. See LICENSE. Earlier Mail-only release executables embedded the **unmodified official
Stalwart 0.16.26 Windows engine**. New shared-mode builds do not embed it: the
separate shared Stalwart executable is supplied by the Calendar shared package
and managed by WorkspaceServices.exe. No local Rust-core rebuild is claimed.
Use scripts/build-shared.ps1 for this mode. The earlier script retains the
pinned official archive verification for explicit legacy packaging. Engine source is available at
https://github.com/stalwartlabs/stalwart/tree/v0.16.26 and in this fork's history.
Stalwart copyright belongs to Stalwart Labs LLC; its source is licensed
AGPL-3.0-only OR LicenseRef-SEL. The community AGPL option applies here.

The One Workspace brand mark is from the Apache-2.0 licensed One Workspace project.

Windows job management, private directory protection and DPAPI helpers in
internal/platform are copied from https://github.com/oneworkspacestudio/one-workspace,
under Apache-2.0; that directory retains its LICENSE. Other Go dependencies
retain their upstream licenses (go-oidc Apache-2.0, oauth2 and x/sys BSD-3-Clause,
go-jose Apache-2.0). No external web fonts, remote image trackers or JavaScript
CDNs are required by this frontend. Stalwart may fetch its own upstream admin
interface, spam rules and IP data during engine initialization.
