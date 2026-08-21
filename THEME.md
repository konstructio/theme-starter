# THEME
version: v2
theme: starter
capabilities: [apps, zones, quota, runtime-logs, app-events]
vocabulary:
  zone: { singular: zone, plural: zones, verb: create }
  app: { singular: app, plural: apps, verb: ship }
  deploy: { verb: ship }

## The contract (v2 — postMessage transport)

Your theme never receives a credential. Keep `theme.js` and call its
methods (`theme.zones(org)`, `theme.apps(org)`, …); everything below is
what it does for you.

- Konstruct launches your theme in a sandboxed iframe at
  `https://<your-theme-host>/?org=<namespace>`. Read the org from the query
  string. There is no token, no fragment, nothing in sessionStorage.
- Each operation is sent to the platform window as
  `{ type: "theme-rpc", id, op, args }` via
  `window.parent.postMessage(…, "*")`. The request carries no secrets.
- Konstruct validates the sender, allows only theme operations, pins every
  org-scoped operation to the org you were launched for (an org you pass is
  ignored), performs the API call with the signed-in user's session on its own
  origin, and replies `{ type: "theme-rpc-result", id, ok, data | error, status }`
  with `targetOrigin` pinned to your theme's origin.
- `theme.isLaunched()` is false when your theme is opened directly (no
  parent window). Render a welcome or sample-data mode — the popout link from
  Konstruct is deliberately unauthenticated.

Allowed ops: `discover`, `zones`, `createZone`, `apps`, `appRepos`,
`shipApp`, `updateApp`, `deleteApp`, `redeploy`, `buildLogs`, `metrics`,
`quota`, `character`, `saveCharacter`, `forgeSite`, `forgeSiteJob`,
`forgeSiteJobs`, `inferSite`, `forgeSiteUpdate`, `domainStatus`.

## Streams (push, not request/response)

Two operations subscribe instead of resolving once. Each returns an
`unsubscribe()` function; the optional `onClose(reason)` fires when the
platform ends the stream (error, limit, teardown) — resubscribe to reconnect.
At most 3 streams can be open at once.

- `theme.logs(org, name, onLine, onClose?)` — live runtime log lines from
  the app's pods, `{pod, time, line}` per event. Lines without a `pod` are
  stream notices (the platform diagnosing itself) — style them differently.
- `theme.appEvents(org, onChange, onClose?)` — one event per app
  create/update/delete in the org. Refetch `apps()` on each instead of
  polling.

Under the hood these use `{ type: "theme-stream-open", id, op, args }`,
`theme-stream-event`, and `theme-stream-close` postMessages; the
platform holds the SSE connection on its own origin — your theme still never
sees a credential.

## Feature detection

`discover().capabilities` lists what this platform implements. Only offer a
feature when its flag is present: `runtime-logs`, `quota`, `app-events`,
`volumes`, `custom-domains`, `site-forge`, `site-update`, `domain-status`
(plus the original `apps`, `zones`, `themes`, `character`).

## Newer app fields

- `shipApp`/`updateApp` accept `volume: {size, mount_path}` — a single persistent
  ReadWriteOnce disk mounted at an absolute path (Konduit defaults to `/data`); attaching one locks the app to 1 replica.
- `shipApp`/`updateApp` accept `custom_domain` — after saving, the app's
  `status.domain_token` is the TXT ownership proof: the user adds
  `TXT _konduit-challenge.<domain>` → `konduit-verify=<token>`, and
  `status.domain_verified` flips true automatically. Empty string clears the
  domain.
- `quota(org)` returns `{plan, capped, cpu|memory|storage: {used, limit}}` —
  the org-wide allowance meter (empty `limit` = uncapped). Quota is admission-
  enforced: surface friendly copy when a ship/update is rejected for it.
- `metrics` series are now `cpu`, `memory`, `pods`, `restarts`, `cpu_limit`,
  `memory_limit`, `network_rx`, `network_tx` (`bytes/s` for network; the
  `*_limit` series are ceilings to draw usage against).
- Ship into an environment: pass `zone_ref` and omit `environment` — the zone
  IS the environment and the platform mirrors it. Registering without an
  environment is rejected.
- `zones()` ordering is a contract guarantee: creation order, oldest first.
  Render the list as given — do not re-sort — so every theme agrees on
  environment order.

## Site forge (capability: `site-forge`)

When `discover().capabilities` includes `site-forge`, the platform can
generate a complete static website from interview answers and push it to
git — the theme then ships the repo like any other app.

- `theme.forgeSite(org, {name, kind, vibe, headline, extras})` starts a job.
  `kind` is `portfolio | blog | landing | docs`; `name` a lowercase RFC-1123
  label. Returns the job `{id, phase, …}`.
- `theme.forgeSiteJob(org, id)` polls it. Phases: `Designing → Writing →
  Pushing → Ready | Failed`. `phase_started_at` + `message` (heartbeat with
  elapsed) power honest progress UI; `brief` is the design brief once
  drafted. On `Ready`, `repo_url`/`repo_name` are exactly what `shipApp`
  needs.
- `theme.forgeSiteJobs(org)` lists jobs newest-first (rehydration).

The job does NOT register the app: call `shipApp({repo_url, repo_name,
zone_ref, region, size, port: 8080, public_url_enabled: true})` yourself
once Ready, then follow the app's build via `apps`/`deployments`/`buildLogs`.
Without the capability flag, hide the feature entirely.

v2 additions:

- `forgeSite` requests are free-text-first: `{name, headline, prompt}` where
  `prompt` is whatever the user typed (any site type). The v1 `kind` field is
  accepted but no longer an enum.
- `theme.inferSite(org, {prompt})` → `{name, headline, summary}` —
  synchronous guesses for a confirm screen. It can 503/fail; always keep a
  local slug fallback so the flow never dead-ends.
- `theme.forgeSiteUpdate(org, {name, instruction})` (capability
  `site-update`) → an update job (`type: "update"`, phases `Reading →
  Writing → Pushing → Ready | Failed`) that commits the change to the site
  repo's `main` WITHOUT force — user commits are never erased, and the
  platform rebuilds automatically (watch `deployments` for `reason:
  "COMMIT"`). One running update per site; a second gets 409. Omitted files
  keep their contents — updates cannot delete files.
- `theme.domainStatus(org, name)` (capability `domain-status`) → the live
  public-resolver view of the app's custom-domain wiring:
  `{domain, txt_record, txt_expected, txt_ok, verified, cname_target,
  resolved_target, cname_ok, serving, checked_at}`. Render a copy-paste
  record checklist from it, and NEVER present the domain as a clickable link
  before `serving` is true — resolver caches lie, especially the user's.
- Forged sites are identifiable by convention: `repo_url` contains
  `/site-<org>-`. The convention (plus a `konduit-site` GitLab topic) is the
  only marker — deleting a theme leaves the sites as plain apps.

v1 themes (token-in-fragment + direct API fetch) stop working once the
platform removes theme-origin CORS: replace your copy of `theme.js` with
this repo's version — the method signatures are unchanged.
