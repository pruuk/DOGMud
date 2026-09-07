# Droplet performance screenshots

DigitalOcean Insights captures for the production droplet, kept so a future
deploy can be compared against a picture rather than a remembered impression.

Naming: `YYYY-MM-DD-droplet-<window>[-context].png`. Keep the window in the name
(`14day`, `6hour`) because the y-axis autoscales, so two captures are only
comparable at the same window length.

Build-stage captures (terminal output from `docker compose up --build`, not an
Insights graph) are named `YYYY-MM-DD-deploy-stage-breakdown.png` instead. They
answer a different question: not "what did the box do" but "where did the time
go inside the build".

Numeric datapoints (restart seconds, CPU spike, idle) live with the deploy
record in the session memory's prod perf baseline, not here. These images are
the supporting evidence.

## Index

### 2026-08-31 — deploy `a1af7269a..7b71bc2eb` (53 commits, PRs #102-#107)

- `2026-08-31-deploy-stage-breakdown.png` — the 20 build stages with per-stage
  timings, captured from the droplet terminal.

Hand-recorded datapoints: **167.0s total**, CPU spike 42%, idle 8%.

What it shows:

- **CPU spike 42% and idle 8% are unchanged from the 2026-08-11 baseline below.**
  Steady-state did not move. Note the two numbers measure different things: the
  spike is the deploy, the idle figure is background load, *not* idle bottoming
  out during the build. Reading them as one series invents a CPU shortfall that
  is not there.
- **72% of the deploy is two stages**: `go build` 97.2s and `go generate` 22.8s.
  Image export is 22.9s, `COPY . .` (15.81MB context) 10.4s, all else ~14s.
- **`COPY . .` sits before both**, so any change at all — docs included —
  invalidates them. That is why deploy time is flat regardless of diff size.
- **The cost is compiling 2,799 Go files across 137 packages on 1 vCPU**, not
  fetching dependencies: `go.mod` has only 20 modules. The `apk add` layer and
  the four setup layers above the COPY were all cache hits at 0.0s.
- **167s against a 135-145s baseline** is a real increase. Expect this to keep
  creeping as the codebase grows, since nothing is cached across builds.

### 2026-08-11 — deploy `6b717c25b..7c64c228c` (148 commits, Waves 1-4 + 2.8/3.6b-1/4.6)

- `2026-08-11-droplet-14day.png` — two weeks to 11 Aug.
- `2026-08-11-droplet-6hour-post-deploy.png` — 05:15 to 11:15 on deploy day.

What they show:

- **The deploy is the 09:00-09:15 spike**: CPU to ~42%, disk write to ~4.7 MB/s,
  a matching bandwidth spike. That is the pull, rebuild and restart, and it
  agrees with the 42% figure recorded by hand.
- **Idle CPU is unchanged at ~8-12% user** before and after, and matches the
  whole 14-day band. This deploy rewrote the entire save path (2.8, 3.6b-1, 4.6)
  and steady-state CPU did not move.
- **The small periodic disk-write humps after 09:30 are autosave cycles.** They
  are the amortised writes from chunk 3.6b-1 doing what they were built to do:
  a series of small regular bumps rather than one large periodic spike. Worth
  keeping as the visual signature to compare against later.
- The 14-day view shows the pre-08-07 period was spikier (CPU to 26-38% on 3, 4,
  5 and 6 Aug, disk to 5-6 MB/s). Those were the 08-03 and 08-04 deploys and the
  migration work, consistent with the conclusion that the 08-03 restart cost was
  migration, not a regression.
