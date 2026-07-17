package revision

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	chrysov1 "github.com/helmetica-framework/chrysopoeia/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// Client loads claims and revisions from a cluster and writes approvals back.
type Client struct {
	kube client.Client
}

// NewClient connects to the cluster the environment points at (KUBECONFIG,
// ~/.kube/config, or in-cluster) with the chrysopoeia types registered.
func NewClient() (*Client, error) {
	scheme := runtime.NewScheme()
	utilruntime.Must(chrysov1.AddToScheme(scheme))

	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("getting config for kube client: %w", err)
	}

	c, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("initializing kube client: %w", err)
	}

	return &Client{
		kube: c,
	}, nil
}

// LoadClaim fetches the claim instance named by operand ("<kind>/<name>",
// kubectl-style, e.g. "podinfo/my-app") in ns. The kind segment resolves
// through the cluster's RESTMapper to one of chrysopoeia's dynamically
// generated CRDs, so the instance is read as unstructured. The returned Claim
// carries spec.ociUrl/version/values plus the instance UID that its
// InstanceRevisions name as controller owner.
func (c *Client) LoadClaim(ctx context.Context, operand, ns string) (Claim, error) {
	split := strings.Split(operand, "/")

	if len(split) != 2 {
		return Claim{}, fmt.Errorf("wrong shape for claim parameter, want: kind/name have: %s", operand)
	}

	kind := split[0]
	name := split[1]

	gvk, err := c.kube.RESTMapper().KindFor(schema.GroupVersionResource{Resource: kind})
	if err != nil {
		return Claim{}, fmt.Errorf("getting gvk for claim of kind %s: %w", kind, err)
	}

	u := unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)

	err = c.kube.Get(ctx, client.ObjectKey{Name: name, Namespace: ns}, &u)
	if err != nil {
		return Claim{}, fmt.Errorf("getting claim %s: %w", operand, err)
	}

	claim := Claim{UID: u.GetUID()}

	var found bool

	claim.OCI, found, err = unstructured.NestedString(u.Object, "spec", "ociUrl")
	if err != nil || !found {
		return Claim{}, fmt.Errorf("claim %s has no usable spec.ociUrl (found=%t): %v", operand, found, err)
	}

	claim.Version, found, err = unstructured.NestedString(u.Object, "spec", "version")
	if err != nil || !found {
		return Claim{}, fmt.Errorf("claim %s has no usable spec.version (found=%t): %v", operand, found, err)
	}

	// values is optional: absent means "no overlay" (nil map, no error); only a
	// present-but-wrong-typed value errors.
	claim.Values, _, err = unstructured.NestedMap(u.Object, "spec", "values")
	if err != nil {
		return Claim{}, fmt.Errorf("getting values from claim %s: %w", operand, err)
	}

	return claim, nil
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
