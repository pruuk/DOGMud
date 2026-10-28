# webshot

`live-shot.js` logs characters into the real web client of a running local
server, in headless Edge, and saves screenshots. It drives the page over the
DevTools protocol with Node's built-in WebSocket; no packages.

It never starts a server. Boot one from a detached worktree on private ports
(`dogmud-shipping`, boot check), with `Playtest.ProfilesManifest` in the
override config so the server materializes seeded characters at boot and
writes their logins to `creds.json` beside the manifest
(`internal/playtestprofiles`). The owner's own server may be on this machine:
never point the script at it.

    node tools/webshot/live-shot.js <shots.json>

The comment at the top of the script gives the `shots.json` shape. Put
`shots.json`, the Edge profile folder and the logins in the session
scratchpad; they hold passwords and are throwaway.

It lives outside `tools/webclient-tests/` because CI runs every script there.
