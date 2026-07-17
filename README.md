# Cupel

**Cupel**: a shallow, porous vessel used in assaying to separate precious metals and reveal their true content.

Cupel **weighs** two sources against each other — where a source is an OCI-hosted Helm chart or a claim resource in your cluster: render each to its Kubernetes manifests, and show the difference side by side in your terminal.
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

Each operand may also be a claim reference (`<kind>/<name>`, resolved in the cluster; `-n` overrides the namespace) instead of an OCI ref, so you can simulate an update by diffing a claim against a chart:

```bash
go run . weigh instance/my-app oci://ghcr.io/stefanprodan/charts/podinfo:6.7.0
```

A claim operand renders with its own values; two claims work too. Pure `oci://` diffs never contact the cluster.

### Keys

| Key | Action |
| --- | ------ |
| `↑` / `↓`, `j` / `k`, `PgUp` / `PgDn` | scroll both columns in lockstep |
| `q`, `esc`, `ctrl-c` | quit |

## Browsing revisions against a claim (`ledger`)

Instead of weighing two sources, the `ledger` command lets you browse a claim's
`InstanceRevision` history from the cluster:

```bash
go run . ledger instance/my-app
```

- The operand names the claim instance (`<kind>/<name>`) — an instance of one of
  chrysopoeia's dynamically generated CRDs.
- The revisions are the `InstanceRevision` objects owned by that instance.
- `-n` / `--namespace` — defaults to the kubeconfig context namespace.

This opens a three-pane TUI: the revision list on the left (sorted oldest → newest
by creation timestamp), and the claim-vs-selected-revision side-by-side diff on
the right. Selecting a revision renders and diffs it on demand.

### Try it with the examples

> **Note:** this assumes a cluster with
> [chrysopoeia](https://github.com/helmetica-framework/chrysopoeia) already
> installed and running (its CRDs plus the Flux source-controller it depends
> on — see its README for setup).

```bash
# Generate the claim CRD: a CustomResourceDefinitionSource plus the Flux
# OCIRepository it reads the chart from.
kubectl apply -f examples/crdsource.yaml

# Wait until the generated CRD is established:
kubectl get crds | grep helmetica-bundles

# Create a claim instance; chrysopoeia records an InstanceRevision for it.
kubectl apply -f examples/instance.yaml
kubectl get instancerevisions

# Browse it — the revision shows red "not approved"; press `a` to approve,
# which patches spec.approvedAt in the cluster:
go run . ledger instance/my-app
kubectl get instancerevisions -o yaml | grep approvedAt

# Or weigh the claim against a newer chart:
go run . weigh instance/my-app oci://ghcr.io/stefanprodan/charts/podinfo:6.7.0
```

Each distinct spec state (`ociUrl`, `version`, `values`) yields a new owned
`InstanceRevision` — bump `spec.version` in `examples/instance.yaml` and
re-apply to grow the history ledger browses.

### Keys

| Key | Action |
| --- | ------ |
| `↑` / `↓`, `j` / `k` | select a revision |
| `a` | approve the selected unapproved revision |
| `PgUp` / `PgDn` | scroll both diff columns in lockstep |
| `q`, `esc`, `ctrl-c` | quit |

Each revision shows an approval status line under its name: green `approved at:`
with the timestamp for approved revisions, red `not approved` for pending ones
(with an `(a) approve` hint when selected), and gray for a future approval date.
Approving patches `spec.approvedAt` on the `InstanceRevision` in the cluster;
the view only flips once the write is confirmed.

## How it works

Each chart is pulled from its OCI registry and rendered client-side, using a
fixed dummy release (name `cupel`, namespace `default`) so any difference is
chart- or values-driven; the cluster is contacted only to read claims and
revisions and to write approvals, never to render. `weigh` renders each
source — an OCI ref with the chart's **default values**, a claim with its
`values` overlaid; `ledger` overlays the claim's and each revision's `values`
on top. The rendered manifests are then diffed line by line and aligned into
rows for the side-by-side view.

## Known limitations

- OCI-ref operands render with **default values only** (no `--values` / `--set` yet); claim operands and `ledger` take values from the claim and each revision.
- Templates that use non-deterministic functions (e.g. `randAlphaNum` in a Helm test hook) can surface as spurious differences, since each render generates fresh values.

## Libraries

* [diff](https://pkg.go.dev/github.com/helmetica-framework/cupel/pkg/diff) - pluggable diff engines over rendered charts (the `linewise` engine ships by default).
* [render](https://pkg.go.dev/github.com/helmetica-framework/cupel/pkg/render) - render a chart to its manifests with default values.
* [source](https://pkg.go.dev/github.com/helmetica-framework/cupel/pkg/source) - turn a diff operand (OCI ref or cluster claim reference) into a rendered manifest.
* [oci](https://pkg.go.dev/github.com/helmetica-framework/cupel/pkg/oci) - pull a Helm chart from an OCI registry.
* [revision](https://pkg.go.dev/github.com/helmetica-framework/cupel/pkg/revision) - load a claim and its InstanceRevisions from the cluster, and write approvals back.
* [tui](https://pkg.go.dev/github.com/helmetica-framework/cupel/pkg/tui) - the interactive side-by-side viewer.
