package revision

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	chrysov1 "github.com/helmetica-framework/chrysopoeia/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Client loads claims and revisions from a cluster and writes approvals back.
type Client struct {
	kube client.Client
}

// LoadRevisions lists the InstanceRevisions in ns that are controller-owned by
// the instance with the given UID, sorted oldest-first by creation timestamp
// (name breaks ties for a stable order).
func (c *Client) LoadRevisions(ctx context.Context, ns string, owner types.UID) ([]Revision, error) {
	revList := &chrysov1.InstanceRevisionList{}

	err := c.kube.List(ctx, revList, &client.ListOptions{Namespace: ns})
	if err != nil {
		return nil, fmt.Errorf("listing revision in %s: %w", ns, err)
	}

	filtered := []Revision{}

	for _, rev := range revList.Items {
		oref := metav1.GetControllerOf(&rev)

		if oref != nil && oref.UID == owner {
			converted, err := fromInstanceRevision(rev)
			if err != nil {
				return nil, err
			}

			filtered = append(filtered, converted)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Created.Equal(filtered[j].Created) {
			return filtered[i].Name < filtered[j].Name
		}
		return filtered[i].Created.Before(filtered[j].Created)
	})

	return filtered, nil
}

// Approve stamps spec.approvedAt on the named InstanceRevision with a merge
// patch, leaving the rest of the spec untouched.
func (c *Client) Approve(ctx context.Context, ns, name string, t time.Time) error {
	ir := &chrysov1.InstanceRevision{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name}}

	patch := fmt.Appendf(nil, `{"spec":{"approvedAt":%q}}`, t.UTC().Format(time.RFC3339))

	return c.kube.Patch(ctx, ir, client.RawPatch(types.MergePatchType, patch))
}

// fromInstanceRevision reduces a typed InstanceRevision to the fields cupel
// renders and diffs. spec.values is optional JSON in Raw; empty Raw means "no
// overlay". Errors name the revision.
func fromInstanceRevision(ir chrysov1.InstanceRevision) (Revision, error) {
	var values map[string]any

	if len(ir.Spec.Values.Raw) > 0 {
		err := json.Unmarshal(ir.Spec.Values.Raw, &values)
		if err != nil {
			return Revision{}, fmt.Errorf("parsing values of revision %s: %w", ir.GetName(), err)
		}
	}

	rev := Revision{
		Name:    ir.GetName(),
		Created: ir.GetCreationTimestamp().Time,
		OCI:     ir.Spec.OCIUrl,
		Version: ir.Spec.Version,
		Values:  values,
	}

	if ir.Spec.ApprovedAt != nil {
		rev.ApprovedAt = &ir.Spec.ApprovedAt.Time
	}

	return rev, nil
}
