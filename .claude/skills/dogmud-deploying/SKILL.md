---
name: dogmud-deploying
description: Use when preparing or reasoning about a deploy to the DOGMud production droplet, or when diagnosing a deploy that behaved oddly. Covers the 1 CPU / 961 MB limits, the unbounded Docker build cache, verifying the checkout SHA before building, root-owned files blocking git writes, and the Caddy container's TrustedProxies range. The owner performs deploys, not Claude.
---

## The owner performs deploys

The owner runs every production deploy themselves; Claude merges to master but
does not deploy and does not ask to. After `gh pr merge`, report the merge
commit and say plainly that prod is still on the previous commit until the
owner deploys. Merging to master is still shipping in the code sense, but the
deploy step stays the owner's, on their own eye, because the droplet is small
and their Docker expertise is self-described as zero. [[feedback-owner-does-the-deploys]]

Nothing below is permission to run a deploy. It is context for reasoning about
one, reviewing one, or helping the owner diagnose one.

## Droplet limits

The production droplet is 1 CPU / 961 MB RAM. Disk pressure from the build
cache (see below) is the same reason `Logging.LogToFile: false` is required in
`_datafiles/config.yaml`: the droplet does not have the disk headroom for
accumulating log files on top of the build cache. Compilation there is CPU and
memory bound, not cache bound, so build-cache tricks are not the first lever
to reach for. [[reference-droplet-build-cache-and-dockerfile]]

## Verify the checkout before building

A suspiciously fast rebuild is a signal, not a relief: it usually means the
checkout did not actually land before the build ran. Verify with
`git rev-parse HEAD` BEFORE building, and compare FULL SHAs, not abbreviated
ones, against the commit that was meant to deploy.

## Known deploy failures

Three failure modes have hit prod, each with its own fix:

- **Build cache growth.** The Docker build cache on the droplet grows
  unbounded and nothing prunes it automatically; it has reached 12 GB (77% of
  disk) before. `docker builder prune -f` reclaimed it back down to a few
  hundred MB without interrupting the running game, though the next build
  after a prune is cold and slower than normal (the build after that is the
  real number). Whether a BuildKit cache mount is worth adding on top of this
  is an open question in the source file itself: it documents a decision NOT
  to add one, and a later section reverses that read and calls it likely to
  help. Both halves stand in the file; this skill does not adjudicate between
  them. [[reference-droplet-build-cache-and-dockerfile]]
- **Root-owned files blocking git writes.** Roughly 9,620 files under the prod
  repo are owned by root, not `mudadmin`. Any git operation that must write
  one of those paths, including `git reset --hard`, fails partway through and
  leaves the working tree half-updated, including live game content YAML,
  which is exactly how a boot panic or a subtly-wrong world happens.
  `sudo chown -R mudadmin:mudadmin` on the repo is the fix.
  [[reference-droplet-root-owned-files-block-git-writes]]
- **Caddy runs as a container, not on loopback.** Prod fronts the MUD with
  Caddy as a separate container on `mud_network`, so the server sees Caddy's
  container address, not `127.0.0.1`. Left at the loopback-only default,
  `Network.TrustedProxies` never trusts the `X-Forwarded-For` header Caddy
  sends, so every web visitor collapses onto Caddy's one address: the admin
  page locks out on shared throttle buckets, and web-client IP bans do
  nothing. The fix is `TrustedProxies: ["172.18.0.0/16"]` (the `mud_network`
  subnet) in `~/mud-config/config-production.yaml`, then a `restart server`
  to also clear the in-memory lockout map. Confirmed fixed on the droplet
  2026-08-21. [[reference-prod-trustedproxies-caddy-container]]

## MOTD

The MOTD lives on the `Motd:` line in `_datafiles/config.yaml` (position
drifts, not the `motd.template` help file). It is a YAML single-quoted string
wrapping a Go template call: apostrophes need `'` (the doubled backslash
is because YAML eats one layer), paragraph breaks are literal `\n`, and
backticks and URLs are fine raw. Copy the existing prefix and suffix exactly
and swap only the inner prose, or avoid apostrophes entirely.
[[reference_motd_location]]

Keep the body to at most four highlights, each a caps headline plus one tight
sentence, and prune every push: the MOTD is a rolling snapshot of the latest
release, not a cumulative changelog. Older detail belongs in
`PATCH_NOTES.md`. End with two links, always on the final lines, never in the
middle: the novel
(`https://github.com/pruuk/DOGMud/blob/master/what_the_moons_keep.md`) and the
full patch notes
(`https://github.com/pruuk/DOGMud/blob/master/PATCH_NOTES.md`).
[[feedback_motd_format]]

## `config.yaml` skip-worktree

`_datafiles/config.yaml` carries `skip-worktree`, which desyncs the disk copy
from the committed blob in both directions: a naive commit dance can leak
local dev overrides (wrong `HttpPort`, `LogToFile: true`, a stray `Playtest:`
block) onto master, and a stale disk copy can silently make a normal-looking
edit revert config another commit already changed. Never build the commit
from the disk file. Build it from `git show HEAD:_datafiles/config.yaml`,
apply the intended edit to that blob, stage it directly with
`git update-index --cacheinfo` (which clears the skip-worktree bit and must be
re-set afterward), and only then update disk separately to match the running
server's needs. [[feedback-skip-worktree-config-leak]]

## Build time baseline

Docker rebuild time on the deploy pipeline has held near 135 to 145 seconds
regardless of diff size across multiple prod pushes, because `COPY . .`
invalidates the Go build cache on essentially every build. A cleanup pass
that shrinks the compiled surface can move this number; if a refactor lands
and rebuild time does not improve, that is worth a second look, though these
are casual, not precision-profiled, measurements.
[[reference_docker_build_times]]

## The guide

`docs/guides/DEPLOYMENT_GUIDE.md` is accurate and current as of this writing
and is the authoritative walkthrough for the full deploy lifecycle: creating
the droplet, initial server setup and firewall, installing Docker, cloning
and configuring, the production compose file and Caddyfile, DNS, the first
deploy, and day to day maintenance (logs, restart, backups, OS updates). It
also carries its own troubleshooting section (stuck certificates, port
conflicts, a `git pull` that silently does nothing, a stale Docker layer
cache, the full copy-paste deploy checklist, SSH lockout recovery, and
copying files off the server with `scp`). Its section 6.x on
`TrustedProxies` and the reverse-proxy subnet already reflects the same fix
recorded above and is not stale. Use the guide as the procedure; this skill
adds the facts and failure modes the guide does not carry: the droplet's
resource ceiling as an operating constraint, the SHA-verification step before
trusting a rebuild, the two additional failure modes (build cache growth,
root-owned files), the MOTD convention, and the `config.yaml` skip-worktree
hazard.

## Sources

Folded: [[feedback-skip-worktree-config-leak]] ·
[[reference_docker_build_times]] · [[feedback_motd_format]] ·
[[reference_motd_location]]

Cited, not folded (dated incidents and a running log; read the source file
for full detail, not restated as rules here):
[[reference-droplet-build-cache-and-dockerfile]] (self-contradictory on the
build-cache-mount question, deliberately left unresolved above) ·
[[reference-droplet-root-owned-files-block-git-writes]] ·
[[reference-prod-trustedproxies-caddy-container]] ·
[[reference_hotswap_upstream_prs_638_639]] (hot-reboot/copyover on prod,
adjacent to but not part of the build-and-restart deploy path covered above) ·
[[reference_prod_perf_baseline]] (running log of pull/restart timing and idle
CPU, newest first)
