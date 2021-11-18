// Copyright 2021 Outreach Corporation. All Rights Reserved.

// Description: This file defines a common Reconciler implementation for kubernetes controllers that use ResourceStatus.
package controllers

import (
	"context"
	"time"

	"github.com/getoutreach/gobox/pkg/k8s/resources"
	"github.com/sirupsen/logrus"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// requeueIntervals are the intervals with which we will requeue failed CRs
// last value (also available as maxRequeueInterval) is used after we tried all
var requeueIntervals = []time.Duration{
	1 * time.Minute,
	2 * time.Minute,
	5 * time.Minute,
	20 * time.Minute,
	60 * time.Minute,
}

// MinRequeueInterval is the min duration between tries on same CR. This is exposed for testing.
func MinRequeueInterval() time.Duration {
	return requeueIntervals[0]
}

// MaxRequeueInterval is used when either the CRD or CR have issues or if there are permission
// issues to read CR or update its status. It is also used as a max retry interval, after too many retries.
// In this case we want controllers to keep trying, but very slow (waiting for the fix to be pushed).
func MaxRequeueInterval() time.Duration {
	return requeueIntervals[len(requeueIntervals)-1]
}

// Handler provides the actual per-CRD implementation of the reconciler.
type Handler interface {
	// CreateResource is called to create an empty CRD resource object.
	CreateResource() resources.Resource
	// Reconcile is called to perform the actual reconciliation of CRD, when its spec changes
	Reconcile(
		ctx context.Context,
		log logrus.FieldLogger,
		in resources.Resource) ReconcileResult
	// EndReconcile is called when reconciliation finishes. It is always called, even if reconcile fails before calling
	// the Handler's Reconcile method.
	// This method is for logging and metrics, ReconcileResult is intentionally passed by value so there is no point modifying it.
	EndReconcile(ctx context.Context, log logrus.FieldLogger, rr ReconcileResult)
}

// Reconciler is a controller for CRD resources.
type Reconciler struct {
	// client accesses k8s api
	client client.Client
	// kind is the CRD kind served by this reconciler
	kind string
	// version is the version of CRD served by this controller
	version string
	// log preconfigured with reconciler fields.
	log logrus.FieldLogger
	// Handler is the reconciler's implementaion.
	handler Handler
}

// reconcileResult holds the outcome of the reconciler
type ReconcileResult struct {
	// Skipped indicates that reconciler has decided to skip this CR.
	Skipped bool
	// ReconcileErr is set when reconciliation fails, or when status changes fail.
	ReconcileErr error
	// PropagateErr is set when reconciler error needs to be reported as a failure to the controller and retry immedaitely
	// (rather than in intervals).
	PropagateErr bool
	// ControllerRes is the result to be returned back to the controller's infra.
	ControllerRes ctrl.Result
	// failCount is for internal use, holding number of times CR with the same hash has failed so far
	failCount int
}

// NewReconciler creates a new reconciler instance.
func NewReconciler(cl client.Client, kind, version string, log logrus.FieldLogger, handler Handler) *Reconciler {
	return &Reconciler{
		client:  cl,
		kind:    kind,
		version: version,
		log:     log,
		handler: handler,
	}
}

// Kind returns the CRD's kind
func (r *Reconciler) Kind() string {
	return r.kind
}

// Version returns the CRD's version served by this reconciler
func (r *Reconciler) Version() string {
	return r.version
}

// Client returns the client to access k8s API.
func (r *Reconciler) Client() client.Client {
	return r.client
}

// endReconcile is invoked when Reconciler finishes.
// This method is for logging and metrics and should NOT modify the ReconcileResult.
func (r *Reconciler) endReconcile(
	ctx context.Context, //nolint:unparam // Why: ctx might be ignored
	log logrus.FieldLogger,
	rr ReconcileResult,
) {
	// pass rr by value - EndReconcile should not tamper with the result
	r.handler.EndReconcile(ctx, log, rr)

	if rr.ReconcileErr != nil {
		// make sure error messages are never lost
		log.WithError(rr.ReconcileErr).Error("Reconciler failed to apply the CR")
		if rr.PropagateErr {
			log.Error("The error will be propageted to the controller")
		}
		return
	}

	if rr.Skipped {
		log.Info("Reconciler skipped this event")
		return
	}

	log.Info("Reconciler has applied the CR successfully")
}

// Reconcile is invoked when controller receives resource spec.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.log.WithField("resourceName", req.NamespacedName).WithField("kind", r.Kind())
	rr := r.doReconcile(ctx, log, req)

	// defer as func to avoid 'capturing' rr - we want to provide latest values to the endReconcile
	defer func() {
		r.endReconcile(ctx, log, *rr)
	}()

	if rr.ReconcileErr != nil {
		if rr.PropagateErr {
			// Controlelr infra will retry immediately.
			return rr.ControllerRes, rr.ReconcileErr
		}

		if rr.Skipped {
			// no further action needed. Even if failed - honor Reconcile decision.
			return rr.ControllerRes, nil
		}

		// if we do not report error to the controller, we need to requeue for this CR
		if rr.ControllerRes.Requeue || rr.ControllerRes.RequeueAfter > 0 {
			// Reconcile alredy asked for requeue, honor it
			return rr.ControllerRes, nil
		}

		// Reconcile reported an error and did not ask for a requeue.
		// This is OK, we will requeue with incrementing intervals.
		rr.ControllerRes.RequeueAfter = getRequeueDuration(rr.failCount)
	}

	return rr.ControllerRes, nil
}

// Reconcile is invoked when resource spec is created or updated.
func (r *Reconciler) doReconcile(
	ctx context.Context,
	log logrus.FieldLogger,
	req ctrl.Request,
) *ReconcileResult {
	rr := ReconcileResult{}
	in := r.handler.CreateResource()
	if getErr := r.Client().Get(ctx, req.NamespacedName, in); getErr != nil {
		log.WithError(getErr).Errorf("unable to get %s CR", r.Kind())
		// this is likely a controller permission issue
		rr.ControllerRes.RequeueAfter = MaxRequeueInterval()
		rr.ReconcileErr = getErr
		return &rr
	}

	// Status changes trigger Reconcile events, ignore those if spec did not change.
	var hash string
	hash, rr.ReconcileErr = in.GetSpec().Hash()
	if rr.ReconcileErr != nil {
		log.WithError(rr.ReconcileErr).Error("failed to calculate hash")
		// this is very likely a permanent error, no need to retry too frequently
		// still, if new deployment happens, we want the new deployment to retry this CR
		rr.ControllerRes.RequeueAfter = MaxRequeueInterval()
		return &rr
	}

	rr.Skipped = !in.GetStatus().ShouldReconcile(hash, log)
	if rr.Skipped {
		// accurate skip reason is logged by ShouldReconcile
		return &rr
	}

	rr = r.handler.Reconcile(ctx, log, in)
	if rr.Skipped {
		// do not take any action here if impl asked to skip status+hash updates (maybe CR is meant for a diff bento)
		// endReconcile is still called - logging done inside
		return &rr
	}

	updateErr := r.updateStatus(ctx, log, in, &rr)
	if updateErr != nil {
		// If we fail to update status of the CR, this is very likely a permission issue.
		// Override handler's decision in this case and proceed with slow retry.
		rr.ReconcileErr = updateErr
		rr.ControllerRes.RequeueAfter = MaxRequeueInterval()
	}

	// logging done inside endReconcile
	return &rr
}

// updateStatus is called to update CR status based on reconcileErr
func (r *Reconciler) updateStatus(
	ctx context.Context,
	log logrus.FieldLogger,
	in resources.Resource,
	rr *ReconcileResult,
) error {
	// The spec is very unlikely to be changed by the controller, yet recalculate the hash just in case.
	hash, err := in.GetSpec().Hash()
	if err != nil {
		log.WithError(err).Errorf("failed to calculate hash for the %s", r.Kind())
		// logging done inside endReconcile
		return err
	}

	// update the CR's status with the reconcile hash and reconcileErr (if present)
	in.GetStatus().Update(hash, rr.ReconcileErr)
	// capture reconcile fail count so far on this hash
	rr.failCount = in.GetStatus().ReconcileFailCount

	err = r.Client().Status().Update(ctx, in)
	if err != nil {
		log.WithError(err).Errorf("unable to update status for %s CR: %+v", r.Kind(), in.GetStatus())
	}

	return err
}

// Setup registers this Reconciler instance as a controller to process target resources.
func (r *Reconciler) Setup(mgr ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(mgr).
		Named(r.Kind()).
		For(r.handler.CreateResource()).
		Complete(r)
	if err != nil {
		r.log.WithError(err).Errorf("failed to setup Reconciler as a controller for %s resource with k8s manager", r.Kind())
	}
	return err
}

// getRequeueDuration returns the requeue interval to retry, based on number of times this CR failed so far.
func getRequeueDuration(failCount int) time.Duration {
	// updateStatus sets failCount after updating the CR's status
	// it will always be 1 if reported.
	if failCount == 0 {
		// if we did not set the failCount, updateStatus failed, likely due to permission issues
		return MaxRequeueInterval()
	}

	if failCount <= len(requeueIntervals) {
		return requeueIntervals[failCount-1]
	}

	return MaxRequeueInterval()
}
