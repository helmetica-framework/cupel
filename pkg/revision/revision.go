// Package revision loads a claim base and a directory of InstanceRevision
// manifests, reducing each to the fields cupel needs to render and diff them.
package revision

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	chrysov1 "github.com/helmetica-framework/chrysopoeia/api/v1"
	"sigs.k8s.io/yaml"
)

// Claim is the base a set of revisions is diffed against. The OCI ref lives
// here until it is added to the InstanceRevision type upstream; every revision
// is rendered from this chart at its own spec.version.
type Claim struct {
	OCI     string         `json:"oci"`
	Version string         `json:"version"`
	Values  map[string]any `json:"values"`
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

// LoadClaim reads and parses the claim YAML at path.
func LoadClaim(path string) (Claim, error) {
	// TODO: replace with k8s client and client get later
	raw, err := os.ReadFile(path)
	if err != nil {
		return Claim{}, fmt.Errorf("reading claim file %s: %w", path, err)
	}

	c := Claim{}
	if err := yaml.Unmarshal(raw, &c); err != nil {
		return Claim{}, fmt.Errorf("parsing claim %s: %w", path, err)
	}

	return c, nil
}

// LoadRevisions reads every *.yaml/*.yml file in dir as an InstanceRevision and
// returns them sorted oldest-first by creation timestamp (name breaks ties for
// a stable order). It fails fast, naming the offending file, if any revision
// cannot be parsed, a malformed revision is a user error worth surfacing.
func LoadRevisions(dir string) ([]Revision, error) {
	// TODO: replace with k8s client later
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("listing revision dir %s: %w", dir, err)
	}

	revisions := []Revision{}
	for _, e := range entries {
		if e.IsDir() || !isYAML(e.Name()) {
			continue
		}

		file := filepath.Join(dir, e.Name())
		rev, err := loadRevision(file)
		if err != nil {
			return nil, err
		}
		revisions = append(revisions, rev)
	}

	sort.Slice(revisions, func(i, j int) bool {
		if revisions[i].Created.Equal(revisions[j].Created) {
			return revisions[i].Name < revisions[j].Name
		}
		return revisions[i].Created.Before(revisions[j].Created)
	})

	return revisions, nil
}

// loadRevision parses a single InstanceRevision file into a Revision.
func loadRevision(file string) (Revision, error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return Revision{}, fmt.Errorf("reading revision %s: %w", file, err)
	}

	ir := chrysov1.InstanceRevision{}
	if err := yaml.Unmarshal(raw, &ir); err != nil {
		return Revision{}, fmt.Errorf("parsing revision %s: %w", file, err)
	}

	// spec.values is optional; sigs yaml stores it as JSON in Raw. Empty Raw
	// means "no overlay", not a parse error.
	var vals map[string]any
	if len(ir.Spec.Values.Raw) > 0 {
		if err := json.Unmarshal(ir.Spec.Values.Raw, &vals); err != nil {
			return Revision{}, fmt.Errorf("invalid values in revision %s: %w", file, err)
		}
	}

	rev := Revision{
		Name:    ir.Name,
		Created: ir.CreationTimestamp.Time,
		OCI:     ir.Spec.OCIUrl,
		Version: ir.Spec.Version,
		Values:  vals,
	}

	// spec.approvedAt is optional; a nil pointer means never approved.
	if ir.Spec.ApprovedAt != nil {
		t := ir.Spec.ApprovedAt.Time
		rev.ApprovedAt = &t
	}

	return rev, nil
}

// isYAML reports whether name has a YAML file extension.
func isYAML(name string) bool {
	return strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")
}
