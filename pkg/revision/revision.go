// Package revision loads a claim base and its InstanceRevisions from the
// cluster via Client and writes approvals back, reducing each to the fields
// cupel needs to render and diff them.
package revision

import (
	"time"

	"k8s.io/apimachinery/pkg/types"
)

// Claim is the base a set of revisions is diffed against: a claim instance's
// spec.ociUrl, spec.version, and spec.values, reduced to what rendering needs.
type Claim struct {
	OCI     string
	Version string
	Values  map[string]any
	// UID identifies the claim instance in the cluster; its InstanceRevisions
	// carry it as their controller ownerReference.
	UID types.UID
}

// Revision is a parsed InstanceRevision reduced to what the diff needs: its
// name (list label and cache key), creation time (sort key), OCI chart ref,
// chart version, and rendered-values overlay.
type Revision struct {
	Name    string
	Created time.Time
	OCI     string
	Version string
	Values  map[string]any
	// ApprovedAt is when the revision was approved; nil means never approved.
	ApprovedAt *time.Time
}
