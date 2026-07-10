# Cupel

**Cupel**: a shallow, porous vessel used in assaying to separate precious metals and reveal their true content.

Cupel **weighs** two sources against each other — where a source is an OCI-hosted Helm chart or a local claim file: render each to its Kubernetes manifests, and show the difference side by side in your terminal.
It's the assayer's balance for charts: put two versions on the scale and see which way they tip.

## Quickstart

```bash
go run . weigh \
  oci://ghcr.io/stefanprodan/charts/podinfo:6.5.0 \
  oci://ghcr.io/stefanprodan/charts/podinfo:6.6.0
```

This opens an interactive side-by-side view of the rendered manifests:

- **left** is the first ref (before), **right** is the second (after)
- red `-` lines were removed, green `+` lines were added, dim lines are unchanged
- the header shows `refA → refB` and a `+N -M` change summary

Each operand may also be a claim file instead of an OCI ref, so you can simulate an update by diffing a claim against a chart:

```bash
go run . weigh ./myclaim.yaml oci://ghcr.io/stefanprodan/charts/podinfo:6.7.0
```

A claim operand renders with its own values; two claims work too.

### Keys

| Key | Action |
| --- | ------ |
| `↑` / `↓`, `j` / `k`, `PgUp` / `PgDn` | scroll both columns in lockstep |
| `q`, `esc`, `ctrl-c` | quit |

## Browsing revisions against a claim (`ledger`)

Instead of weighing two sources, the `ledger` command lets you browse a set of
local `InstanceRevision` manifests against a **claim** base:

```bash
go run . ledger -r ./myrevisions -c myclaim.yaml
```

- `-r` / `--revisions` — a directory of `InstanceRevision` YAML files.
- `-c` / `--claim` — the base every revision is diffed against.

This opens a three-pane TUI: the revision list on the left (sorted oldest → newest
by creation timestamp), and the claim-vs-selected-revision side-by-side diff on
the right. Selecting a revision renders and diffs it on demand.

The claim file is the base; its shape is:

```yaml
oci: oci://ghcr.io/stefanprodan/charts/podinfo   # chart to pull
version: 6.14.0                                  # base version
values:                                          # base values overlay
  replicaCount: 1
```

Each revision file is an `InstanceRevision` manifest and carries its own
`spec.ociUrl`, `spec.version`, and `spec.values`. The claim base is rendered
from its `oci` at its `version`, each revision from its own `ociUrl` at its
`spec.version`, both with their `values` merged over the chart defaults, then
diffed like weigh.

### Keys

| Key | Action |
| --- | ------ |
| `↑` / `↓`, `j` / `k` | select a revision |
| `PgUp` / `PgDn` | scroll both diff columns in lockstep |
| `q`, `esc`, `ctrl-c` | quit |

## How it works

Each chart is pulled from its OCI registry and rendered client-side (no cluster
contact), using a fixed dummy release (name `cupel`, namespace `default`) so any
difference is chart- or values-driven. `weigh` renders each source — an OCI ref
with the chart's **default values**, a claim file with its `values` overlaid;
`ledger` overlays the claim's and each revision's `values` on top. The rendered
manifests are then diffed line by line and aligned into rows for the
side-by-side view.

## Known limitations

- OCI-ref operands render with **default values only** (no `--values` / `--set` yet); claim operands and `ledger` take values from the claim and each revision.
- Templates that use non-deterministic functions (e.g. `randAlphaNum` in a Helm test hook) can surface as spurious differences, since each render generates fresh values.

## Libraries

* [diff](https://pkg.go.dev/github.com/helmetica-framework/cupel/pkg/diff) - pluggable diff engines over rendered charts (the `linewise` engine ships by default).
* [render](https://pkg.go.dev/github.com/helmetica-framework/cupel/pkg/render) - render a chart to its manifests with default values.
* [source](https://pkg.go.dev/github.com/helmetica-framework/cupel/pkg/source) - turn a diff operand (OCI ref or claim file) into a rendered manifest.
* [oci](https://pkg.go.dev/github.com/helmetica-framework/cupel/pkg/oci) - pull a Helm chart from an OCI registry.
* [revision](https://pkg.go.dev/github.com/helmetica-framework/cupel/pkg/revision) - load a claim base and a directory of InstanceRevision files.
* [tui](https://pkg.go.dev/github.com/helmetica-framework/cupel/pkg/tui) - the interactive side-by-side viewer.
