# Cupel

**Cupel**: a shallow, porous vessel used in assaying to separate precious metals and reveal their true content.

The cupel compares Helm release revisions, surfacing what changed between two points in a release's history — the diff that tells you whether an upgrade is pure or has picked up dross.

## Quickstart

```bash
go run . compare myrelease 3 5
```

Every flag can also be provided as an environment variable with the `CUPEL_` prefix, e.g. `CUPEL_NAMESPACE`.

## Libraries

_None yet — this is a fresh scaffold._
