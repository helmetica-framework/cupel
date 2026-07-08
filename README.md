# Cupel

**Cupel**: a shallow, porous vessel used in assaying to separate precious metals and reveal their true content.

Cupel **weighs** two OCI-hosted Helm charts against each other — pull both, render each to its Kubernetes manifests, and show the difference side by side in your terminal. It's the assayer's balance for charts: put two versions on the scale and see which way they tip.

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

### Keys

| Key | Action |
| --- | ------ |
| `↑` / `↓`, `j` / `k`, `PgUp` / `PgDn` | scroll both columns in lockstep |
| `q`, `esc`, `ctrl-c` | quit |

## How it works

Each chart is pulled from its OCI registry and rendered with its **default values**, client-side (no cluster contact), using a fixed dummy release — name `cupel`, namespace `default` — so any difference is purely chart-driven. The rendered manifests are then diffed line by line and aligned into rows for the side-by-side view.

## Known limitations

- Renders with **default values only** (no `--values` / `--set` yet).
- Templates that use non-deterministic functions (e.g. `randAlphaNum` in a Helm test hook) can surface as spurious differences, since each render generates fresh values.

## Libraries

* [diff](https://pkg.go.dev/github.com/helmetica-framework/cupel/pkg/diff) — pluggable diff engines over rendered charts (the `linewise` engine ships by default).
* [render](https://pkg.go.dev/github.com/helmetica-framework/cupel/pkg/render) — render a chart to its manifests with default values.
* [oci](https://pkg.go.dev/github.com/helmetica-framework/cupel/pkg/oci) — pull a Helm chart from an OCI registry.
* [tui](https://pkg.go.dev/github.com/helmetica-framework/cupel/pkg/tui) — the interactive side-by-side viewer.
