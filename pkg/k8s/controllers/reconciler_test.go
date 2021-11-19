package controllers_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/getoutreach/gobox/pkg/k8s/controllers"
	"github.com/getoutreach/gobox/pkg/k8s/resources"
	"github.com/getoutreach/gobox/pkg/k8s/resources/mocks"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type TestResourceSpec struct {
	FakeHash string
}

func (s *TestResourceSpec) Hash() (string, error) {
	return s.FakeHash, nil
}

type TestHandler struct {
	UseNilResource bool
	FakeResult     controllers.ReconcileResult
	// EndResult can be different from FakeResult if it is not returned by Handler's Reconcile
	EndResult controllers.ReconcileResult
}

func (h *TestHandler) CreateResource() resources.Resource {
	if h.UseNilResource {
		// to simulate Get failures
		return nil
	}
	return &mocks.TestResource{}
}

func (h *TestHandler) Reconcile(
	ctx context.Context,
	log logrus.FieldLogger,
	in resources.Resource,
) controllers.ReconcileResult {
	// must clone to avoid cross-call contamination of result
	return h.FakeResult
}

func (h *TestHandler) NotFound(ctx context.Context, log logrus.FieldLogger, resourceName types.NamespacedName) controllers.ReconcileResult {
	return h.FakeResult
}

func (h *TestHandler) EndReconcile(ctx context.Context, log logrus.FieldLogger, rr controllers.ReconcileResult) {
	// capture end result for testing
	h.EndResult = rr
}

func createFakeClient(t *testing.T) client.Client {
	s := scheme.Scheme
	assert.NilError(t, apiextensions.SchemeBuilder.AddToScheme(s))
	assert.NilError(t, mocks.SchemeBuilder.AddToScheme(s))

	// register new schema and test CRD with the new client
	return fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(mocks.GetTestResourceDefinition()).
		Build()
}

func newRequest(name string) reconcile.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: mocks.TestNamespace,
			Name:      name,
		},
	}
}

func assertStatus(t *testing.T, cl client.Client, name string, expected *resources.ResourceStatus) {
	var cr mocks.TestResource
	err := cl.Get(
		context.Background(),
		types.NamespacedName{
			Namespace: mocks.TestNamespace,
			Name:      name,
		},
		&cr)

	assert.NilError(t, err)

	assert.Equal(t, cr.Status.LastReconcileSuccessHash, expected.LastReconcileSuccessHash, "LastReconcileSuccessHash does not match")
	assert.Equal(t, cr.Status.LastReconcileErrorHash, expected.LastReconcileErrorHash, "LastReconcileErrorHash does not match")
	assert.Equal(t, cr.Status.ReconcileFailCount, expected.ReconcileFailCount, "ReconcileFailCount does not match")
}

func TestReconciler_Sanity(t *testing.T) {
	log := logrus.New()
	handler := &TestHandler{}
	cl := createFakeClient(t)
	reconciler := controllers.NewReconciler(cl, mocks.TestKind, mocks.TestVer, log, handler)

	assert.Equal(t, reconciler.Kind(), mocks.TestKind)
	assert.Equal(t, reconciler.Version(), mocks.TestVer)
	assert.Equal(t, reconciler.Client(), cl)
}

func TestReconciler_MissingResource(t *testing.T) {
	log := logrus.New()
	handler := &TestHandler{}
	cl := createFakeClient(t)
	reconciler := controllers.NewReconciler(cl, mocks.TestKind, mocks.TestVer, log, handler)
	ctx := context.Background()

	// sanity test the reconciler - Get on the CR is expected to fail
	req := newRequest("any")

	res, err := reconciler.Reconcile(ctx, req)
	// we do not return err to controller, instead we log it and requeue
	assert.NilError(t, err)

	// We handle non-existing CRs by invoking the Handler.NotFound callback and do not retry unless Handler reports an error.
	assert.NilError(t, err)
	assert.NilError(t, err)
	assert.Equal(t, res, ctrl.Result{})
	// make sure we went thru NotFound case
	assert.Check(t, handler.EndResult.NotFound)
}

func TestReconciler_GetError(t *testing.T) {
	log := logrus.New()
	handler := &TestHandler{}
	cl := createFakeClient(t)
	ctx := context.Background()

	// it is hard to simulate Get error other than Not Found. Even if we delete the whole CRD, error is still Not Found.
	// Best way to simulate the call failure is by serving the Get method with the nil instead of a resource struct ptr.
	// Since we do not ())yet) differentiate between local to network errors (other than Not Found), for the Reconciler
	// it would be like client.Get has failed.
	handler.UseNilResource = true
	// on Get errors which are not "Not Found" we shall not trigger Reconcile nor NotFound callbacks, this err should not be used.
	handler.FakeResult.ReconcileErr = fmt.Errorf("should not be used")

	assert.NilError(t, cl.Create(ctx, mocks.NewTestResource("obj1")))
	reconciler := controllers.NewReconciler(cl, mocks.TestKind, mocks.TestVer, log, handler)

	req := newRequest("obj1")

	res, err := reconciler.Reconcile(ctx, req)
	// we do not return err to controller, instead we log it and requeue
	assert.NilError(t, err)
	assert.ErrorContains(t, handler.EndResult.ReconcileErr, "expected pointer")
	assert.Equal(t, res.RequeueAfter, controllers.MaxRequeueInterval())
	// make sure NotFound and Skipped stay false
	assert.Check(t, !handler.EndResult.NotFound && !handler.EndResult.Skipped)
}

func TestReconciler_SuccessCase(t *testing.T) {
	log := logrus.New()
	handler := &TestHandler{}
	cl := createFakeClient(t)
	reconciler := controllers.NewReconciler(cl, mocks.TestKind, mocks.TestVer, log, handler)
	ctx := context.Background()

	// create CR 'obj1' and reconcile it again
	assert.NilError(t, cl.Create(ctx, mocks.NewTestResource("obj1")))

	req := newRequest("obj1")
	res, err := reconciler.Reconcile(ctx, req)
	assert.NilError(t, err)
	assert.NilError(t, handler.EndResult.ReconcileErr)
	// we should not requeue on success
	assert.DeepEqual(t, res, ctrl.Result{})

	assertStatus(t, cl, "obj1", &resources.ResourceStatus{
		LastReconcileSuccessHash: mocks.InitialHash,
		LastReconcileErrorHash:   "",
	})
}

func TestReconciler_ReconcileError(t *testing.T) {
	log := logrus.New()
	handler := &TestHandler{}
	cl := createFakeClient(t)
	reconciler := controllers.NewReconciler(cl, mocks.TestKind, mocks.TestVer, log, handler)
	ctx := context.Background()

	// create CR 'obj1' and reconcile it again
	assert.NilError(t, cl.Create(ctx, mocks.NewTestResource("obj1")))

	// force handler to fail reconcile
	handler.FakeResult.ReconcileErr = errors.New("oops")

	req := newRequest("obj1")

	lastTry := 10
	for try := 1; try <= lastTry; try++ {
		res, err := reconciler.Reconcile(ctx, req)
		// this error is not propagated, thus no controller err here
		assert.NilError(t, err)
		assert.ErrorContains(t, handler.EndResult.ReconcileErr, "oops")

		assertStatus(t, cl, "obj1", &resources.ResourceStatus{
			LastReconcileSuccessHash: "",
			LastReconcileErrorHash:   mocks.InitialHash,
			ReconcileFailCount:       try,
		})

		// we should auto-requeue on this error, starting from min interval
		switch {
		case try == 1:
			// on first try we should get the min
			assert.DeepEqual(t, res, ctrl.Result{RequeueAfter: controllers.MinRequeueInterval()})
		case try == lastTry:
			// on last try we should get the max (assuming maxTry is large enough)
			assert.DeepEqual(t, res, ctrl.Result{RequeueAfter: controllers.MaxRequeueInterval()})
		default:
			// in between, we can get any between min to max
			assert.Check(t,
				res.RequeueAfter > controllers.MinRequeueInterval() &&
					res.RequeueAfter <= controllers.MaxRequeueInterval(),
				fmt.Sprintf("received unexpected res %v", res))
		}
	}
}

func TestReconciler_ReconcileErrorPropagated(t *testing.T) {
	log := logrus.New()
	handler := &TestHandler{}
	cl := createFakeClient(t)
	reconciler := controllers.NewReconciler(cl, mocks.TestKind, mocks.TestVer, log, handler)
	ctx := context.Background()

	// create CR 'obj1' and reconcile it again
	assert.NilError(t, cl.Create(ctx, mocks.NewTestResource("obj1")))

	// force handler to fail reconcile
	handler.FakeResult.ReconcileErr = errors.New("oops")
	handler.FakeResult.PropagateErr = true

	req := newRequest("obj1")

	res, err := reconciler.Reconcile(ctx, req)

	assert.ErrorContains(t, err, "oops")
	assert.ErrorContains(t, handler.EndResult.ReconcileErr, "oops")

	// if error is propagated, we do not requeue - controller infra to do so
	assert.DeepEqual(t, res, ctrl.Result{})
}

func TestReconciler_ReconcileSkipped(t *testing.T) {
	log := logrus.New()
	handler := &TestHandler{}
	cl := createFakeClient(t)
	reconciler := controllers.NewReconciler(cl, mocks.TestKind, mocks.TestVer, log, handler)
	ctx := context.Background()

	// create CR 'obj1' and reconcile it again
	assert.NilError(t, cl.Create(ctx, mocks.NewTestResource("obj1")))

	// force handler to fail reconcile
	handler.FakeResult.Skipped = true

	req := newRequest("obj1")

	res, err := reconciler.Reconcile(ctx, req)

	assert.NilError(t, err)
	assert.NilError(t, handler.EndResult.ReconcileErr)
	assert.DeepEqual(t, res, ctrl.Result{})

	assertStatus(t, cl, "obj1", &resources.ResourceStatus{
		// success/failure hashes must stay empty since this CR is skipped
	})
}
