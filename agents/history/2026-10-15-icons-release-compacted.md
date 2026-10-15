# Application icon refresh and local Windows release

User authorized publishing all app fixes and new releases to oneworkspacestudio,
then requested the supplied eleven SVG app icons and README logos first.
This source is prepared for `workspace-mail-v0.1.0-preview.16` in `oneworkspacestudio/mail`.
Builds run locally on Windows; organization Actions are disabled, including the
previously enabled Notes/MyAccounts repositories. No Actions job is used.

The supplied SVG paths/gradients remain unchanged; PNG/ICO variants and Windows
resources are generated from them. Web branding, favicons, suite launchers and
repository README logos are updated. Existing Workspace hub artwork is retained
because no replacement hub icon was included. Build scripts preserve original
license notices and pinned third-party engines. Do not copy live/private data
into any source or release artifact.

Prior app-flow-fixes handoff records implemented fixes and sampled acceptance.
The icon refresh is a build/publication task; it is not a new full browser or
clean-host acceptance run. Mail Safe Browsing review and provider logout browser
block remain unresolved; do not claim delivery or provider sign-out accepted.
Notes/Mail/Calendar integration feature gaps remain explicit in release notes.

Release preparation/evidence outside Git: projects tooling workspace-release-20261015.
The suite handoff records the final publication receipt. Verify GitHub release
state/assets and source tag before claiming publication. Never rewrite published
tags/assets or use GitHub Actions for this release.

## README logo visibility correction

GitHub rendered the supplied SVGs, but inline40/42px title icons were easy to
miss beside older screenshots. README now leads with a112px labelled logo.
Workspace also has a current-logo overview and explicit organization repository
links for every app. Documentation-only follow-up; published binaries/tags and
current preview limitations are unchanged. No Actions or runtime tests needed.
