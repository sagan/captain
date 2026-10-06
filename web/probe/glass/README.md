# Glassmorphism public frontend

This frontend vendors the public Vue UI from
[sanrokamlan-prog/komari-theme-Glassmorphism](https://github.com/sanrokamlan-prog/komari-theme-Glassmorphism),
version 3.3.7, commit `06999d5a77ebe521a02fd3e383e67a064e345e7e` (MIT; see LICENSE).
It preserves the upstream templates, Tailwind styles, background, icons, globe,
node cards/list, details and ECharts components. The footer identifies Captain
as the server and retains the theme attribution. The Komari admin application
is not included.

The three-carrier panels and their colour thresholds additionally derive from
[vlongx/komari-theme-Glassmorphism-three-network](https://github.com/vlongx/komari-theme-Glassmorphism-three-network),
commit `2f172e7f9e8a87e0e3a1d0c5c2d2ef3555278b9c` (same MIT licence).
The base frontend remains the newer upstream version above.

Captain adaptations:

- `src/captain.ts` is a read-only adapter to `/api/probe` and its per-node history
  endpoints. `utils/init.ts` initializes the original stores and polls Captain.
  No Komari server, agent, RPC endpoint, or additional port is required.
- `ping-summary.ts` translates counted Captain samples into the original twenty
  latency/loss bars. Null gaps, weighted attempts and successful latency are
  preserved. The metric adapter does not invent raw attempts or quantiles.
- `ping-selection.ts` selects the configured data source. With carrier monitoring
  enabled and CT/CU/CM configured, both card and list show the original three
  rows for latency and loss. Missing samples leave the corresponding row empty.
  Otherwise the first enabled, node-assigned task by ID supplies both bars.
  `ping_tasks` in the public node snapshot contains only IDs and labels, never
  destinations. Selection never jumps to another task because samples are absent.
  Custom non-carrier labels are not presented as Chinese operators.
- CPU/resource validity, monthly monitoring traffic, section visibility and
  unavailable financial data follow Captain semantics. An unknown price does
  not mean free; cost/value cards show a dash. All traffic totals and quota bars
  use persisted period counters (`traffic.used_up/used_down`), independently of
  live network rates. The overview adds each selected node’s current period;
  nodes may have different reset days. Restarts preserve recorded usage. Missing
  counters stay unknown rather than falling back to OS counters. GPU details remain private.
- Hash routes retain arbitrary status page mounts and dedicated host support.
  The snapshot's additive `appearance_scheme` resolves inherited site colours
  without needing `/api/site` on a dedicated status hostname.
- Icons and images are local. `scripts/glass-icons.mjs` rebuilds the icon subset;
  `ICON-LICENSES.json` and `licenses/` record the third-party icon licences.
  The production bundle includes the theme and icon licence texts under `licenses/`.
  External visitor audit/fingerprinting, IP geolocation, exchange-rate fetches
  and Komari administrative operations are disabled.
- Core public labels have six-language dictionaries. Language follows Captain's
  saved `i18nextLng` preference, then the browser language.

Run `pnpm build` from `web/probe`; it tests the adapter and builds both frontends.
`pnpm dev:glass` starts this frontend with the existing local API proxy. The
embedded handler selects this build when `appearance.preset` is `glass`.

When updating, compare against a separately built upstream checkout with the
same data, viewport, theme and browser. Do not replace the upstream layout with
an approximation. Account for Captain branding and unsupported data separately
from layout differences. Do not use screenshots with a frozen clock to assert
that the animated globe renders: check its real pixels with an advancing clock.
Do not use masked comparison images as product previews: all six overview cards,
including the unknown remaining-value card, must be visible in a full preview.
The globe has no scale-in dependency and falls back to a bundled static texture
when WebGL is unavailable or its context is lost.
