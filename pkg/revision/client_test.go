package revision

import (
	"context"
	"strings"
	"testing"
	"time"

	chrysov1 "github.com/helmetica-framework/chrysopoeia/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
