# Nodes, inbounds and the paths traffic takes

How a server becomes a node, what it serves, and how traffic gets in and
out of it. Version notes name the bosun release a feature needs; Captain
1.2 ships against bosun 0.49.

## Adding a node

Admin → Nodes → new node gives you a one-time pairing code and two ways to
use it:

```sh
curl -fsSL https://panel.example.com/api/agent/install.sh?pair=CODE | sh
```

or, with Docker, a `docker run` line carrying `BOSUN_CAPTAIN` and
`BOSUN_PAIR`. The installer fetches bosun, writes
`/etc/bosun/config.yaml`, creates the service, pairs with the panel and
applies the state it receives. A node that pairs with an empty public
address gets it filled in from the address it paired from.

A node can be given a **host name** (Node → Domain, e.g.
`jp1.example.com`): new inbound recipes use it as the TLS name and entries
advertise it instead of the address, so a certificate for that name reaches
the node with no further setup.

Nodes learn about changes within seconds: bosun keeps a long-poll request
open on the state endpoint, so nothing has to be pushed and no persistent
connection is needed. The node page shows host metrics, the cores' status,
the doctor's verdicts and which inbounds were not applied and why.

**Node jobs** run from the console: upgrade to a release, roll back to the
previous build, scan REALITY targets, run a speed test. Each is recorded
with its result.

## Removing a node

Node → Delete asks for the node name and offers three actions:

- **Keep bosun in standalone mode** imports the currently provisioned inbounds,
  users, per-inbound access and limits, routes, certificates and forwards, then
  restarts bosun with the local driver. UUIDs/passwords and immutable traffic
  identities remain. Captain's plans, balances, history, expiry schedule and
  subscription URLs stay in Captain; standalone subscription URLs are new.
  Existing local administrator credentials remain. A headless node gets a panel
  on `127.0.0.1:2053`: use SSH to set its password with
  `bosun admin set -user admin -password 'NEW_PASSWORD'`, restart bosun, then use an
  SSH tunnel or configure a reverse proxy. Imported per-inbound limits remain
  until that user is edited locally; configure ongoing quotas/expiry locally.
- **Uninstall bosun** stops the agent and its cores, removes its service, binary
  and `/etc/bosun`, and cleans up its own firewall/forwarding/shaping rules.
  `/var/lib/bosun` is removed unless **Keep local data** is selected. This does
  not uninstall OS packages or change SSH, interfaces or unrelated firewall rules.
- **Only remove the Captain record** retains the legacy behavior, including for
  offline nodes: delete the node and its inbounds without contacting bosun.
  The node may continue serving the cached configuration in managed mode.

Remote actions require a full administrator, an online paired node running
**bosun ≥ v0.55.0**, and the standard Linux systemd/OpenRC installation paths.
Docker containers and custom installation paths must be managed on the host.
The worker runs outside bosun's service so it can report after stopping it.
Captain deletes its record only after a successful result. Failures/timeouts
retain the record; reopen the dialog to see or resume the same task. An unclaimed
request expires after five minutes, so a returning offline node cannot execute
an old uninstall. A standalone startup failure restores the managed files and
attempts to restart the original service.

API: `POST /api/admin/nodes/{id}/removal` accepts
`{"mode":"standalone"}` or `{"mode":"uninstall","keep_data":true}` and returns
`{"id":"JOB_ID"}`. `GET` on that path returns the latest job or `null`.
After `done_at` is set, `error` is empty and `result.phase` is `complete`, use
`DELETE /api/admin/nodes/{id}?removal_job=JOB_ID`. The original DELETE without
that query still removes only the record. Workers authenticate their claim and
completion at `POST /api/agent/removal` with the existing node credential.

If result delivery fails, the node retains a root-only plan/result under
`/var/tmp/bosun-removal-JOB_ID/`. After inspecting the worker service/log and
confirming it is no longer running, rerun its staged worker with
`/var/tmp/bosun-removal-JOB_ID/worker internal-node-removal /var/tmp/bosun-removal-JOB_ID/plan.json` to deliver
an existing `result.json` without repeating the operation. Keep this directory
private: it contains the node credential. Do not remove a `running.lock` while
its worker is alive.

## Inbounds

The **Core** selector filters choices by protocol, transport and features such
as Shadowsocks 2022, ShadowTLS, PROXY protocol, fallbacks and Snell mode.
**Automatic (recommended)** follows the node's configured core priority;
REALITY prefers Xray. A manual choice is retained when settings change; an
incompatible choice must be corrected before saving.

With bosun ≥ 0.55.0, the form previews automatic selection and greys out
compatible cores that the node has not enabled. Enable those in the node's
`config.yaml` first; selecting a core does not install or enable it. Idle
cores are available even while stopped. Older, offline or unreported nodes
show unknown availability and validate when applying the configuration.
The list separates the configured choice from the last reported running core.
A failed apply has no confirmed running assignment. Existing `Core` values
and the default selection order are preserved; no database migration is needed.

An inbound is one listening service on one node: protocol, port, listen
address, transport and its settings. Quick-setup recipes fill in a working
configuration for VLESS+REALITY, Hysteria2, mieru, Shadowsocks 2022,
SS2022 + ShadowTLS and Trojan+WS, generating any keys server-side so an
API-created inbound cannot break a node.

Supported protocols: VLESS (+ REALITY), VMess, Trojan, Shadowsocks
(including 2022 ciphers), Hysteria2, TUIC, AnyTLS, mieru, Snell, SOCKS,
HTTP, NaiveProxy, WireGuard. Which core serves which protocol is bosun's
decision; see its [README](https://github.com/zeptop-dev/bosun).

**Scoped users** — an inbound bound to a user group only provisions that
group's users; an ungrouped inbound gets every user with a usable
subscription. A user whose plan expires or runs out of quota disappears
from the node's desired state, which is what stops their traffic.

**Port conflicts** — saving or enabling an inbound whose transport and port
is already taken by another enabled inbound or by a forward on the same
node, on an overlapping bind address, is refused with 409 instead of
failing on the node. UDP is counted for Hysteria2, TUIC and WireGuard, both
for Shadowsocks and Snell, and mieru per its transport (a `BOTH` inbound
also reserves `port + 1`).

**Protocol knobs worth knowing**

- **REALITY** — the target (`dest`) can be scanned from the node: Admin
  runs a scan job and gets a list of candidates with real TLS 1.3 / h2 /
  X25519 / certificate checks and CDN detection, then picks one. A node can
  also serve its own HTTPS *decoy* site on loopback and use it as the
  target, so a prober sees a genuine certificate for your name.
  With bosun v0.61.0 or later, the scanner also measures incoming TLS wire records, including headers,
  encrypted overhead and stapled OCSP data. Records over the conservative
  8192-byte limit used by the pinned Xray are rejected by automatic selection.
  Older bosun versions without this check show **Not checked**, not a pass.
  This is basic screening, not a full REALITY connection: accumulated records,
  a different ClientHello or a different destination edge can still fail.
  Confirm candidates with the node's actual core and client.
- **ShadowTLS** — a Shadowsocks inbound can be wrapped in ShadowTLS v3
  (the *SS2022 + ShadowTLS* recipe, or the switch on any Shadowsocks
  inbound). The public port performs a real TLS handshake with a site you
  name — `www.apple.com:443` by default — and only an authenticated client
  is handed through to the Shadowsocks inbound, which moves to loopback: a
  prober sees that site's certificate and nothing else. Every user gets
  their own ShadowTLS password derived from their UUID, on top of the
  Shadowsocks key, so revoking a user revokes both. Strict mode (on by
  default) refuses a ClientHello the site itself would not accept.
  sing-box only — Xray has no ShadowTLS, REALITY is its answer to the same
  problem — and the subscription carries it for mihomo, Stash, sing-box,
  Surge, Loon and Shadowrocket. Needs bosun ≥ 0.48.
- **Snell** — served by sing-box (bosun ≥ 0.41; its Snell server speaks
  v5, obfs http). With *Multi-user (sing-box)* every user connects with
  their own key and traffic is accounted per user, but only sing-box
  clients can present a user key, so such lines appear in sing-box
  subscriptions only. Obfs tls still needs Surge's own `snell-server`.
- **mieru** — MTU, multiplexing level and handshake mode per inbound reach
  the `mierus://` links and the mihomo/Stash lines. Transport `BOTH` serves
  TCP on the port and UDP on `port + 1`; links list both and mihomo takes
  TCP.

## Entries

An entry is what a user sees in their client: a display host and port on
top of a landing inbound. That indirection is what lets a relay, a line
ingress or a domain stand in front of the same inbound. Tags, regions,
ordering and auto flags are described in
[SUBSCRIPTIONS.md](SUBSCRIPTIONS.md#entries-what-each-user-sees).

## Port forwards (relay tunnels)

Node page → Port forwards: listen on a port of this node and relay raw TCP,
UDP or both to a landing server. Clients connect to the relay while the
landing inbound keeps doing authentication and per-user accounting. Pick
another managed node's inbound as the target and one click creates an entry
advertising this relay's address; the node reports each rule's
reachability, RTT, connection count and bytes.

A rule's backend is one of:

| Backend | What it is | Trade-off |
|---|---|---|
| built-in relay | bosun's own userspace relay | connection and byte counters, PROXY protocol support |
| `nft` | nftables kernel DNAT (bosun ≥ 0.18) | IPv4/IPv6 target (IPv6 needs bosun ≥ 0.62), same address family as the listen address, optional source preservation when replies route back through the node; no counters |
| `realm` | bosun installs and runs [realm](https://github.com/zhboner/realm) | high throughput, hostname targets, UDP; no counters |

**Several targets.** A rule can have further targets (the split-arrows
button on the rule): a backup line for when the first is down, or more
lines to spread connections over. *Failover* sends new connections to the
first target whose health check passes, and a connection whose dial fails
moves on to the next target before the client notices; *round-robin*
spreads connections over the healthy targets by weight. Each target's
health, RTT and connection count shows next to the rule. Built-in relay:
both modes; realm: round-robin only (it does not retry another target);
nft: one target. A connection always uses one target, so this keeps a
relay up and spreads load across lines — it does not make one download
faster. Needs bosun ≥ 0.49. With bosun ≥ 0.62, the displayed measurement
explicitly says TCP; UDP is untested, not offline on a refused TCP connection.
Mixed rules show the two separately. UDP hop selection is independent of TCP
probe health. Selecting Hysteria2/TUIC for a new forward defaults to UDP;
existing/manual protocol choices remain configurable. Older UDP health reports
are shown as unknown because their probe transport was not recorded.

Ports are checked against the node's own inbounds. Xray-style domain/IP
splitting inside a tunnel is deliberately not offered: use the landing
node's routing rules instead.

**Device counting behind a relay.** A relay hides the client's address from
the landing node, so by default every connection arriving from one of the
panel's own nodes counts as *one* online device (and is marked "via relay"
in the user drawer). For the real per-client picture, tick *Expect PROXY
protocol* on a landing inbound that is only ever reached through this
panel's forwards (xray only): forwards targeting it then send a PROXY
protocol v2 header automatically (built-in relay or realm backend), the
landing node sees the real client, and devices are counted exactly. Direct
connections to such an inbound fail, by design.

## Ingresses and port mappings (NAT / IPLC)

A node behind NAT, an IPLC or a dedicated line may expose only provider-assigned
ports. Node page → Ingresses and port mappings records the provider's existing
configuration; it does not create NAT mappings or change the provider firewall.
Choose NAT or IPLC / dedicated line. Existing `mapped` records keep their old
addresses, ranges and optional direct listeners after migration.

- the **local NIC address** on the VPS — inbounds and forwards bind to it so replies go
  back through the line;
- the line's **far-end address** — what a relay must forward to; not
  reachable from the public internet;
- the provider's **public entry**, if the service includes one (a China
  Mobile entry address, for example);
- the usable **local port range** and an optional public-port offset; or
  multiple explicit mappings, each with local start/end and public start.

For example, local `20001–20099` → public `30001–30099` and local `40000` →
public `443` can coexist in one NAT ingress. A single port has equal local
start/end. Local and public ranges must each be non-overlapping and stay within
1–65535. Explicit mappings replace the legacy range/offset, rather than adding
to an unrestricted default. Mappings apply equally to TCP and UDP; check which
protocols the provider forwards. Reserved ports are **local** ports (e.g. SSH)
and cannot be used by an inbound or a forward.

Inbounds and forwards both select an ingress. Create and update validate their
listeners against its port policy; a nonempty listen address must match its
bind address. Mieru BOTH also validates its second, adjacent UDP port and the
corresponding public mapping. The relay backend does not bypass these checks.

**Require an ingress on this node** prevents any inbound or forward on that
node from choosing direct access. The new NAT form enables it by default;
existing records default to off. Leave it off for a multihomed node that still
needs direct public listeners. All selected ingresses enforce their own port
policy regardless of this switch. This controls managed proxy and relay
listeners, not SSH, the management UI, or other programs on the server.

Enabling this switch or editing a port policy checks existing inbounds and
forwards first, including disabled configurations. An incompatible edit is
rejected without changing the saved configuration. An ingress still referenced
by an inbound or forward cannot be deleted; reassign or remove those listeners
first. Existing Captain subscription entries have explicit display addresses:
review them after changing public mapping addresses or ports.

The existing API routes and range/offset fields remain. New fields are
`kind: nat|iplc|mapped`, `port_mappings` (rows with `local_from`, `local_to`,
`public_from`) and `require_ingress`; ingress editor requests use their existing
PascalCase equivalents. Omitting or sending null for the new mapping or switch
fields on update preserves saved values. Forward `ingress_id` is an optional
string containing the ingress ID. Captain resolves its bind address before
sending state, so managed nodes do not require a new agent to use this feature.
The bosun standalone editor implements the same policies locally.

Inbounds pick an ingress (direct is the default, or the first ingress on nodes with
no public address). Any protocol may ride a line, and a recipe applied
while an ingress is selected takes the first free, non-reserved port of the
range. Whether a given protocol passes is up to the provider's entry — some
carrier entries only pass non-TLS protocols such as mieru. Entries then
advertise the public entry on the mapped port.

A line **without** a public entry is served through a relay node: add a
port forward there whose target is the far-end address (the picker fills it
in) and use the relay's address in the entry. For a NAT target without a far-end
address, the picker uses its public address and mapped public port. A relay
itself behind NAT can select its own ingress; creating a subscription entry
from that forward uses the relay's mapped public address. Direct inbounds on the same
node (Hysteria2, REALITY) keep using the node's own address or domain.

With the probe on, every ingress that has both a local NIC address and a
far-end address gets an automatic RTT task (bosun ≥ 0.15 binds the TCP
connect to the NIC; even a refused port measures the line), shown under the
ingress name on the status and speed-test pages. Lines are usually private,
so carrier latency is not measured through them.

## Node connections: VLESS Reverse

Open **Nodes → exit B → Node connections · VLESS Reverse → Configure transits**.
Select up to 32 transit nodes A in one wizard. Each connection owns two Xray
listeners on A: a user-facing inbound (VLESS + REALITY by default), and a
separate VLESS + REALITY receiver for B's dedicated tunnel identity. It also
creates a subscription entry pointing at A. Users authenticate and their
traffic is counted on A; B initiates the reverse connections and exits directly.
B needs no public tunnel listener. Ordinary inbounds on either node remain
independent, including ordinary A inbounds that exit locally.

For each A, choose the local user and tunnel ports, optional NAT/IPLC ingress,
user group and REALITY SNI. The wizard previews both public endpoints after
port mapping. Both ports must be available and, for NAT, forwarded by the
provider; Captain cannot create the provider's forwarding rules. B must be
able to reach A's public tunnel endpoint, and A must be able to reach the
chosen REALITY target. User protocol settings reuse the inbound editor and
require Xray support; the transport between B and A remains VLESS + REALITY.

Each A has **Auto select** and **Probe current target** buttons. Scans run on
that transit, not on B or Captain, so results reflect A's path to the target.
Selecting a target updates both the tunnel SNI and the REALITY user listener;
the advanced protocol editor and bulk SNI action keep those values consistent.
An ordinary TLS user listener keeps its own certificate name. Editing the
target or closing/removing the form cancels result polling and prevents stale
results from replacing the current draft; a job already queued on A may finish.

Enter a target verified with a real REALITY connection from each A; new
connections have no preselected SNI. Avoid `www.microsoft.com` with the pinned
Xray 26.3.27: some Microsoft responses include OCSP data that pushes the TLS
Certificate record beyond its REALITY implementation's 8192-byte buffer.
An ordinary TLS 1.3 handshake can succeed while REALITY fails. This is a
[reported upstream limitation](https://github.com/XTLS/Xray-core/issues/6356),
not a permanent SNI blacklist. The [upstream buffer fix](https://github.com/XTLS/REALITY/pull/33)
does not change the already pinned core; a core upgrade requires separate
compatibility testing.

Save the full set once. Later, reopen the same wizard to add/remove A nodes,
change a connection or disable it. Existing connections retain their identities
and keys. Removing a connection deletes only its owned listeners and entry;
changing settings can restart the affected node's shared Xray process and
interrupt other connections on that process. Saving is atomic in Captain,
but each node applies its new state independently. Conflicting concurrent
edits are rejected. Managed resources cannot be edited through the ordinary
inbound/entry forms; remove connections before detaching a node to standalone.
Deleting either node also cleans up the connection's owned resources.

Both nodes need bosun v0.60.0 or later, advertising `vless_reverse` under Xray core
capabilities and Xray enabled. Older nodes receive none of the managed reverse
inbounds, so ignoring a new field cannot silently turn A into a direct exit.
The wizard distinguishes upgrade required, offline, awaiting application,
connected, disconnected and unknown. Connected means A reported an active
tunnel identity after both nodes applied their current configuration; it is
not an end-to-end website availability check. Xray reconnects automatically;
use either node's self-check and diagnostics to investigate failures.

Reverse user traffic is pinned to its reverse outbound. It fails closed when
B is unavailable, without falling back to A's default exit. B still enforces
private-destination isolation and audit blocks. Xray overrides of routing,
outbounds and policy are refused while managed connections exist; configure
ordinary traffic through the routing editor instead. Tags beginning `reverse-`
are reserved on these nodes.

**Limits:** this is a TCP reverse transport, not WireGuard. Xray may maintain
multiple workers/connections; it does not promise one connection or zero CPS.
It may help with incoming TCP connection pressure, but cannot establish an
ISP's cause of packet loss or remove B's connections to destination websites.
The current socket-mark-based tc per-user speed limits do not apply to the
multiplexed reverse path. Captain aggregates usage after reports arrive; shared
quotas across several A nodes are not an instantaneous global hard cap. Do not
use this mode when those per-user speed guarantees are required.

## Outbounds, landing and egress

**Outbounds & landing** (node page): add an exit from a share link — bosun
renders it in the serving core's dialect (sing-box takes every protocol,
Xray vless/vmess/trojan/ss/socks/http) — chain exits with `via`, then pick
a default exit for the whole node or add rules such as `inbound:tag`,
`domain:`, `ip:`, `protocol:`, `port:` → outbound / direct / block. Needs
bosun ≥ 0.12.

**Egress follows ingress.** On a node with several public addresses, the
node option *Egress follows ingress* makes every inbound that is bound to a
specific address send its users' traffic out from that same address
(sing-box, xray and hysteria; mieru cannot). Give each inbound its bind
address; any-address inbounds keep the default route, and a default landing
outbound takes precedence. Needs bosun ≥ 0.44.

**Private destinations are refused.** A node's own neighbourhood —
loopback, link-local, cloud metadata, RFC 1918, CGNAT — is rejected for
user traffic in the cores' own routing and again by an nftables egress
guard, and the cores' unauthenticated control APIs are reachable only by
root. Details in bosun's README; the live regression checks it on every
release.

## Domains and certificates

Admin → Domains & certs registers the domains you own — each with
Cloudflare as the DNS provider, using the global token from Settings → ACME
or its own token when the zone lives in another account, or "manual" —
shows what uses each one (node host names, inbound TLS names, subscription
hosts, the panel itself) and manages certificates:

- **Issue** — Let's Encrypt through DNS-01 on the panel, one certificate
  for any set of names (`example.com` plus `*.example.com` together is
  fine), stored in the database and renewed 30 days before expiry; the
  first failed renewal notifies the admin. The Cloudflare token stays on
  the panel: nodes need neither a token nor port 80.
- **Upload** a PEM pair, or let a certificate manager (Certimate, an
  acme.sh deploy hook) POST renewals to the per-panel webhook shown on the
  page. The JSON keys are lenient: `domain`/`domains`, `certificate`,
  `privateKey`/`private_key`.
- **Deploy by coverage** — every node whose standard-TLS inbounds use a
  covered name (exact or wildcard) receives the pair in its state; bosun
  (≥ 0.13) uses it ahead of ACME and lists it as method `custom`. Deleting
  a certificate lets the nodes fall back to node-side ACME.

**Automatic DNS records.** A registered Cloudflare domain with *Auto DNS
records* on (the default) gets A/AAAA records created or updated whenever a
node with a host name under it is saved (node domain → public / IPv6
address), or a line ingress with an *entry domain* is saved (entry domain →
the provider's entry address). Records are never deleted and never proxied,
and the outcome is shown in a toast. The token needs DNS edit permission on
the zone, which the DNS-01 token already has.

## Additions in 1.8.0

Inbound and routing editors support typed named presets with a draft diff and impact preview. Node detail also links to administrator-only supplier/asset costs. See [administration workflows](ADMIN_WORKFLOWS.md).

## Private upstream exceptions (bosun ≥ 0.62)

Full administrators can edit **Node → Routing → Private upstream exceptions**.
Enter an IP/CIDR, TCP/UDP and one port per row. Use this for an authenticated
SOCKS upstream reachable over a private WireGuard/BGP network, for example
`10.10.0.2/32`, TCP, 1080. The endpoint remains an ordinary outbound in Routing;
this policy only permits the core's network socket through the OS guard.

`GET/PUT /api/admin/settings/nodes/{id}/egress` reads/writes
`{"upstreams":[{"cidr":"10.10.0.2/32","protocol":"tcp","port":1080}]}`.
Only administrators (including appropriately scoped administrator tokens) may
access it; old nodes reject nonempty updates. GET reports `supported` and
`minimum_version`. PUT requires the list: omission/null is rejected, `[]`
explicitly clears it. The new column is separate from routing, so older
routing clients cannot erase the policy. Updates use the topology lock.
Check the node self-check after saving: a saved policy alone is not proof that
Linux/nft installed it. Local YAML exceptions also apply.

Exceptions cover all cores on the node, not one inbound/outbound. They do not
add user direct routes; cores without destination filtering, or unresolved
hostnames, may also let users reach an excepted endpoint. Keep the exception
narrow and authenticate the upstream. Per-inbound private access is not part
of this feature. Disabling host `cores.egress_guard` no longer leaves stale
rules after restart, and root-only control APIs remain protected.

### Per-inbound private access (Captain 1.14 / bosun 0.63)

In the inbound editor, use **Private access** (under **Advanced** in the
standalone editor and reverse wizard). The default is
**Off**. **Internal networks** allows RFC 1918 and IPv6 ULA; **Custom addresses**
accepts 1–64 literal IP/CIDR rules, optionally restricted to TCP/UDP and a
single destination port or range. Blank protocol means both; port 0 means all
ports. CGNAT needs a custom rule. Loopback, link-local, core control APIs and
known cloud metadata endpoints remain protected even inside a broader grant.
Prefer a narrow address and port for a single internal service.

The permission applies to every authorized user of that inbound. Use the
existing user/group assignment to keep it internal. Full Captain administrators
alone can enable or edit an inbound granting this access. The standalone
administrator can configure it in the same editor. It grants destinations,
not a new exit: existing split routes, audit blocks and bound direct exits
still apply. OS WireGuard routes work as ordinary system routes. For managed
VLESS Reverse, edit the user inbound through the exit node's connection wizard;
the corresponding reverse client on B enforces its own policy, independently
of other transits. A continues tunnelling to B. Authentication and accounting
remain on A; the existing reverse-path per-user tc limitation is unchanged.

This requires bosun ≥ 0.63.0 advertising `private_access`, Linux nftables,
a non-root core account, enabled egress protection and sing-box or Xray.
Capabilities reflect the node's runtime prerequisites. Native Hysteria, mita
and snell-server, WireGuard inbounds, shared-key Snell and Xray SOCKS/HTTP
inbounds do not support this policy. Hysteria2 may use sing-box instead.
Currently the whole node must use direct/freedom outbounds: proxy chains,
remote proxies, WARP, balancers, GeoIP split rules, global private grants and
raw overrides of routing, DNS, policy, inbounds or outbounds are rejected.
This restriction avoids claiming enforcement when a remote proxy or override
controls the final socket. Node-wide upstream exceptions remain a separate
feature; private grant sockets cannot borrow their broader exceptions.

Changing the effective permission/mark set restarts supervised proxy cores to
close old sockets before installing the new grants; existing connections will
reconnect. Installation failure stops those cores and reports an apply error.
Ordinary limited users' private socket marks share their existing tc class.
A known older/unsupported node cannot save an enabled policy, and delivery
withholds such inbounds/clients after a capability downgrade. Do not downgrade
a node while relying on this feature.

The additive `private_access` object lives in the existing inbound settings:

```json
{"private_access":{"mode":"custom","rules":[{"cidr":"10.10.0.2/32","protocol":"tcp","port_start":8080}]}}
```

Use `{"mode":"internal"}` for the preset and `{"mode":"off"}` to disable.
Older API clients that omit or send null for this field preserve an existing
policy. Templates do not export private permissions; applying a preset retains
the destination inbound's current permission. No database migration is needed.
