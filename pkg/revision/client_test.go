package revision

import (
	"context"
	"strings"
	"testing"
	"time"

	chrysov1 "github.com/helmetica-framework/chrysopoeia/api/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const ownerUID = types.UID("uid-instance-1")

// newIR builds an InstanceRevision fixture. owner=="" means no ownerReference;
// values=="" means no spec.values; approved==nil means never approved.
func newIR(name string, created time.Time, owner types.UID, values string, approved *time.Time) *chrysov1.InstanceRevision {
	ir := &chrysov1.InstanceRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         "prod",
			CreationTimestamp: metav1.Time{Time: created},
		},
		Spec: chrysov1.InstanceRevisionSpec{
			OCIUrl:  "oci://demo",
			Version: "1.0.0",
		},
	}
	if values != "" {
		ir.Spec.Values.Raw = []byte(values)
	}
	if owner != "" {
		isController := true
		ir.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: "podinfo.helmetica-bundles.io/v1",
			Kind:       "Podinfo",
			Name:       "my-app",
			UID:        owner,
			Controller: &isController,
		}}
	}
	if approved != nil {
		ir.Spec.ApprovedAt = &metav1.Time{Time: *approved}
	}
	return ir
}

func chrysoScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := chrysov1.AddToScheme(s); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	return s
}

func revClient(t *testing.T, objs ...client.Object) *Client {
	t.Helper()
	kube := fake.NewClientBuilder().WithScheme(chrysoScheme(t)).WithObjects(objs...).Build()
	return &Client{kube: kube}
}

func TestLoadRevisionsFiltersByControllerOwnerAndSorts(t *testing.T) {
	t1 := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 7, 2, 9, 0, 0, 0, time.UTC)
	c := revClient(t,
		newIR("rev-newer", t2, ownerUID, "", nil),
		newIR("rev-older", t1, ownerUID, "", nil),
		newIR("rev-other-owner", t1, types.UID("uid-other"), "", nil),
		newIR("rev-no-owner", t1, "", "", nil),
	)

	revs, err := c.LoadRevisions(context.Background(), "prod", ownerUID)
	if err != nil {
		t.Fatalf("LoadRevisions: %v", err)
	}
	if len(revs) != 2 {
		t.Fatalf("got %d revisions, want 2 (owner-filtered)", len(revs))
	}
	if revs[0].Name != "rev-older" || revs[1].Name != "rev-newer" {
		t.Errorf("order = [%s, %s], want [rev-older, rev-newer]", revs[0].Name, revs[1].Name)
	}
}

func TestLoadRevisionsEqualTimestampsSortByName(t *testing.T) {
	same := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	c := revClient(t,
		newIR("rev-b", same, ownerUID, "", nil),
		newIR("rev-a", same, ownerUID, "", nil),
	)

	revs, err := c.LoadRevisions(context.Background(), "prod", ownerUID)
	if err != nil {
		t.Fatalf("LoadRevisions: %v", err)
	}
	if len(revs) != 2 || revs[0].Name != "rev-a" || revs[1].Name != "rev-b" {
		t.Errorf("equal-timestamp order not by name: %v", []string{revs[0].Name, revs[1].Name})
	}
}

func TestLoadRevisionsScopedToNamespace(t *testing.T) {
	ir := newIR("rev-a", time.Unix(1, 0), ownerUID, "", nil)
	ir.Namespace = "other"
	c := revClient(t, ir)

	revs, err := c.LoadRevisions(context.Background(), "prod", ownerUID)
	if err != nil {
		t.Fatalf("LoadRevisions: %v", err)
	}
	if len(revs) != 0 {
		t.Errorf("got %d revisions from wrong namespace, want 0", len(revs))
	}
}

func TestLoadRevisionsExtractsSpecFields(t *testing.T) {
	approved := time.Date(2026, 7, 8, 16, 1, 29, 0, time.UTC)
	c := revClient(t,
		newIR("rev-a", time.Unix(1, 0), ownerUID, `{"replicaCount":3}`, &approved),
	)

	revs, err := c.LoadRevisions(context.Background(), "prod", ownerUID)
	if err != nil {
		t.Fatalf("LoadRevisions: %v", err)
	}
	if len(revs) != 1 {
		t.Fatalf("got %d revisions, want 1", len(revs))
	}
	r := revs[0]
	if r.OCI != "oci://demo" || r.Version != "1.0.0" {
		t.Errorf("OCI/Version = %q/%q", r.OCI, r.Version)
	}
	rc, ok := r.Values["replicaCount"]
	if !ok {
		t.Fatalf("Values missing replicaCount: %#v", r.Values)
	}
	if got := toInt(t, rc); got != 3 {
		t.Errorf("replicaCount = %d, want 3", got)
	}
	if r.ApprovedAt == nil || !r.ApprovedAt.Equal(approved) {
		t.Errorf("ApprovedAt = %v, want %v", r.ApprovedAt, approved)
	}
}

func TestLoadRevisionsAllowsMissingValuesRaw(t *testing.T) {
	c := revClient(t, newIR("rev-a", time.Unix(1, 0), ownerUID, "", nil))

	revs, err := c.LoadRevisions(context.Background(), "prod", ownerUID)
	if err != nil {
		t.Fatalf("LoadRevisions: %v", err)
	}
	if revs[0].Values != nil {
		t.Errorf("Values = %#v, want nil for a revision with no spec.values", revs[0].Values)
	}
}

// Malformed values can't be exercised through the fake client (it rejects the
// broken RawExtension while marshalling the List response, as the real API
// server would at admission), so the conversion is tested directly.
func TestFromInstanceRevisionFailsOnMalformedValuesNamingRevision(t *testing.T) {
	ir := newIR("rev-broken", time.Unix(1, 0), ownerUID, `{not json`, nil)

	_, err := fromInstanceRevision(*ir)
	if err == nil {
		t.Fatal("expected error for malformed values")
	}
	if !strings.Contains(err.Error(), "rev-broken") {
		t.Errorf("error should name the offending revision, got: %v", err)
	}
}

func TestApprovePatchesApprovedAt(t *testing.T) {
	c := revClient(t, newIR("rev-a", time.Unix(1, 0), ownerUID, "", nil))
	stamp := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)

	if err := c.Approve(context.Background(), "prod", "rev-a", stamp); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	var got chrysov1.InstanceRevision
	if err := c.kube.Get(context.Background(), client.ObjectKey{Namespace: "prod", Name: "rev-a"}, &got); err != nil {
		t.Fatalf("Get after Approve: %v", err)
	}
	if got.Spec.ApprovedAt == nil {
		t.Fatal("spec.approvedAt not set after Approve")
	}
	if !got.Spec.ApprovedAt.Time.Equal(stamp) {
		t.Errorf("approvedAt = %v, want %v", got.Spec.ApprovedAt.Time, stamp)
	}
}

func TestApproveMissingRevisionErrors(t *testing.T) {
	c := revClient(t)
	if err := c.Approve(context.Background(), "prod", "no-such-rev", time.Now()); err == nil {
		t.Fatal("expected error approving a missing revision")
	}
}

// claimClient builds a Client over a fake cluster containing one instance of a
// dynamically generated claim CRD (group podinfo.helmetica-bundles.io, kind
// Podinfo), with a RESTMapper that resolves the lowercase "podinfo" resource.
func claimClient(t *testing.T) *Client {
	t.Helper()
	gv := schema.GroupVersion{Group: "podinfo.helmetica-bundles.io", Version: "v1"}
	gvk := gv.WithKind("Podinfo")

	s := runtime.NewScheme()
	s.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	s.AddKnownTypeWithName(gv.WithKind("PodinfoList"), &unstructured.UnstructuredList{})

	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{gv})
	mapper.Add(gvk, meta.RESTScopeNamespace)

	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "podinfo.helmetica-bundles.io/v1",
		"kind":       "Podinfo",
		"metadata": map[string]any{
			"name":      "my-app",
			"namespace": "prod",
			"uid":       string(ownerUID),
		},
		"spec": map[string]any{
			"ociUrl":  "oci://ghcr.io/x/podinfo",
			"version": "6.5.0",
			"values":  map[string]any{"replicaCount": int64(2)},
		},
	}}

	kube := fake.NewClientBuilder().WithScheme(s).WithRESTMapper(mapper).WithObjects(u).Build()
	return &Client{kube: kube}
}

func TestLoadClaimResolvesKindAndMapsSpec(t *testing.T) {
	c := claimClient(t)

	claim, err := c.LoadClaim(context.Background(), "podinfo/my-app", "prod")
	if err != nil {
		t.Fatalf("LoadClaim: %v", err)
	}
	if claim.UID != ownerUID {
		t.Errorf("UID = %q, want %q", claim.UID, ownerUID)
	}
	if claim.OCI != "oci://ghcr.io/x/podinfo" {
		t.Errorf("OCI = %q", claim.OCI)
	}
	if claim.Version != "6.5.0" {
		t.Errorf("Version = %q", claim.Version)
	}
	rc, ok := claim.Values["replicaCount"]
	if !ok {
		t.Fatalf("Values missing replicaCount: %#v", claim.Values)
	}
	if got := toInt(t, rc); got != 2 {
		t.Errorf("replicaCount = %d, want 2", got)
	}
}

// claimClientWithSpec builds a Client over a fake cluster whose single
// Podinfo claim instance has the given spec, for exercising malformed or
// incomplete specs.
func claimClientWithSpec(t *testing.T, spec map[string]any) *Client {
	t.Helper()
	gv := schema.GroupVersion{Group: "podinfo.helmetica-bundles.io", Version: "v1"}
	gvk := gv.WithKind("Podinfo")

	s := runtime.NewScheme()
	s.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	s.AddKnownTypeWithName(gv.WithKind("PodinfoList"), &unstructured.UnstructuredList{})

	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{gv})
	mapper.Add(gvk, meta.RESTScopeNamespace)

	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "podinfo.helmetica-bundles.io/v1",
		"kind":       "Podinfo",
		"metadata": map[string]any{
			"name":      "my-app",
			"namespace": "prod",
			"uid":       string(ownerUID),
		},
		"spec": spec,
	}}

	kube := fake.NewClientBuilder().WithScheme(s).WithRESTMapper(mapper).WithObjects(u).Build()
	return &Client{kube: kube}
}

func TestLoadClaimMissingOCIUrlErrors(t *testing.T) {
	c := claimClientWithSpec(t, map[string]any{"version": "6.5.0"})
	if _, err := c.LoadClaim(context.Background(), "podinfo/my-app", "prod"); err == nil {
		t.Fatal("expected error for claim without spec.ociUrl")
	}
}

func TestLoadClaimMissingVersionErrors(t *testing.T) {
	c := claimClientWithSpec(t, map[string]any{"ociUrl": "oci://ghcr.io/x/podinfo"})
	if _, err := c.LoadClaim(context.Background(), "podinfo/my-app", "prod"); err == nil {
		t.Fatal("expected error for claim without spec.version")
	}
}

func TestLoadClaimWrongTypedValuesErrors(t *testing.T) {
	c := claimClientWithSpec(t, map[string]any{
		"ociUrl":  "oci://ghcr.io/x/podinfo",
		"version": "6.5.0",
		"values":  "not-a-map",
	})
	if _, err := c.LoadClaim(context.Background(), "podinfo/my-app", "prod"); err == nil {
		t.Fatal("expected error for claim whose spec.values is not a map")
	}
}

func TestLoadClaimBadOperandShape(t *testing.T) {
	c := claimClient(t)
	for _, operand := range []string{"no-slash-here", "/no-kind", "no-name/"} {
		if _, err := c.LoadClaim(context.Background(), operand, "prod"); err == nil {
			t.Errorf("%q: expected error for operand without <kind>/<name> shape", operand)
		}
	}
}

func TestLoadClaimUnknownKindErrors(t *testing.T) {
	c := claimClient(t)
	_, err := c.LoadClaim(context.Background(), "nosuchkind/my-app", "prod")
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
	if !strings.Contains(err.Error(), "nosuchkind") {
		t.Errorf("error should name the kind, got: %v", err)
	}
}

func TestLoadClaimMissingInstanceErrors(t *testing.T) {
	c := claimClient(t)
	_, err := c.LoadClaim(context.Background(), "podinfo/no-such-app", "prod")
	if err == nil {
		t.Fatal("expected error for missing instance")
	}
	if !strings.Contains(err.Error(), "no-such-app") {
		t.Errorf("error should name the instance, got: %v", err)
	}
}

func TestNewClientFailsWithoutKubeconfig(t *testing.T) {
	t.Setenv("KUBECONFIG", "/no/such/kubeconfig")
	// Ensure the in-cluster fallback also fails.
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	if _, err := NewClient(); err == nil {
		t.Fatal("expected NewClient to fail without any reachable config")
	}
}
