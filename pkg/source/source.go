// Package source turns a diff operand — an OCI chart ref or a claim file — into
// a renderable manifest, so any two sources diff through one code path.
package source

import (
	"strings"

	"github.com/helmetica-framework/cupel/pkg/oci"
	"github.com/helmetica-framework/cupel/pkg/render"
	"github.com/helmetica-framework/cupel/pkg/revision"
)

// Source renders to a Kubernetes manifest for one side of a diff.
type Source interface {
	// Render pulls (if needed) and renders this source, returning the manifest
	// and a short label for the column header.
	Render(puller oci.Puller) (manifest, label string, err error)
}

// chartSource is the shared implementation behind every constructor: pull ref,
// render with overlay (nil for defaults), report label. The three constructors
// differ only in which ref, overlay, and label they set.
type chartSource struct {
	ref     string
	overlay map[string]any
	label   string
}

// OCI renders an OCI chart ref with its defaults. Label is the ref.
func OCI(ref string) Source {
	return &chartSource{ref: ref, overlay: nil, label: ref}
}

// Claim renders a claim's chart with the claim's values overlaid. Label is
// "oci:version".
func Claim(c revision.Claim) Source {
	ref := c.OCI + ":" + c.Version
	return &chartSource{
		ref:     ref,
		overlay: c.Values,
		label:   ref,
	}
}

// Revision renders a revision's chart with the revision's values overlaid.
// Label is the revision name, so the revision TUI column header stays
// meaningful.
func Revision(r revision.Revision) Source {
	ref := r.OCI + ":" + r.Version
	return &chartSource{
		overlay: r.Values,
		label:   r.Name,
		ref:     ref,
	}
}

// Parse turns a weigh CLI operand into a Source: an "oci://"-prefixed operand is
// an OCI ref; anything else is treated as a claim file path and loaded (load
// errors, including a missing/invalid file, surface here with the path).
func Parse(operand string) (Source, error) {
	if strings.HasPrefix(operand, "oci://") {
		return OCI(operand), nil
	}

	// LoadClaim already names the path in its errors; don't re-wrap.
	c, err := revision.LoadClaim(operand)
	if err != nil {
		return nil, err
	}

	return Claim(c), nil
}

// Render pulls the ref and renders it with the overlay; a nil overlay renders
// the chart defaults.
func (s *chartSource) Render(puller oci.Puller) (manifest, label string, err error) {
	chrt, err := puller.Pull(s.ref)
	if err != nil {
		return "", "", err
	}

	manifest, err = render.RenderWith(chrt, s.overlay)
	if err != nil {
		return "", "", err
	}

	return manifest, s.label, nil
}
