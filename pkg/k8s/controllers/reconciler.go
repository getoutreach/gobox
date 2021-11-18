// Copyright 2021 Outreach Corporation. All Rights Reserved.

// Description: This file defines a kubernetes controller for v1/PostgresqlDevenvDatabase.
package controllers

import (
	"context"
	"time"

	"github.com/getoutreach/gobox/pkg/k8s/resources"
	"github.com/sirupsen/logrus"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// MaxRequeueInterval is used when either CRD has issues (or spec defined wrong) or
	// if there is some permission issue to read CRD or update its status or when we reached the max time to retry
	// on same CR.
	// In this case we want controllers to keep trying, but very slow (likely waiting for the fix to be pushed).
	MaxRequeueInterval = 60 * time.Minute
)

// requeueIntervals are the intervals with which we will requeue failed CRs
// MaxRequeueInterval should be used after len of this list is exhausted
var requeueIntervals = []time.Duration{
	2 * time.Minute,
	5 * time.Minute,
	20 * time.Minute,
}

type Handler interface {
	// CreateResource is called to create an empty CRD resource object.
	CreateResource() resources.Resource
	// Reconcile is called to perform the actual reconciliation of CRD, when its spec changes
	Reconcile(
		ctx context.Context,
		log logrus.FieldLogger,
		in resources.Resource) *ReconcileResult
	// EndReconcile is called when reconciliation finishes. It is always called, even if reconcile fails before calling
	// the Handler's Reconcile method.
	EndReconcile(ctx context.Context, log logrus.FieldLogger, rr *ReconcileResult)
	// Close is called when reconciler's controller shuts down
	Close(ctx context.Context)
}

// Reconciler is a controller for CRD resources.
type Reconciler struct {
	client.Client
	// Kind is the CRD kind served by this reconciler
	Kind string
	// Version is the version of CRD served by this controller
	Version string
	// Log preconfigured with reconciler fields.
	Log logrus.FieldLogger
	// Handler is the reconciler's implementaion.
	Handler Handler
}

// reconcileResult holds the outcome of the reconciler
type ReconcileResult struct {
	// Skipped indicates that reconciler has decided to skip this CRD
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

// endReconcile is invoked when Reconciler finishes. This method is for logging and metrics.
func (r *Reconciler) endReconcile(
	ctx context.Context, //nolint:unparam // Why: ctx might be ignored
	log logrus.FieldLogger,
	rr *ReconcileResult,
) {
	r.Handler.EndReconcile(ctx, log, rr)

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
	log := r.Log.WithField("resourceName", req.NamespacedName).WithField("kind", r.Kind)
	rr := r.doReconcile(ctx, log, req)
	// invoking endReconcile via the defer mechanism caused many bugs due to shadowing
	r.endReconcile(ctx, log, rr)

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
	rr := &ReconcileResult{}
	in := r.Handler.CreateResource()
	if getErr := r.Get(ctx, req.NamespacedName, in); getErr != nil {
		log.WithError(getErr).Errorf("unable to get %s CR", r.Kind)
		// this can be controller permission issue, so retrying immediately won't help
		rr.ControllerRes.RequeueAfter = MaxRequeueInterval
		rr.ReconcileErr = getErr
		return rr
	}

	// Status changes trigger Reconcile events, ignore those if spec did not change.
	var hash string
	hash, rr.ReconcileErr = in.GetSpec().Hash()
	if rr.ReconcileErr != nil {
		log.WithError(rr.ReconcileErr).Error("failed to calculate hash, no retries allowed until new code is deployed")
		// this is very likely a permanent error, no need to retry too frequently
		// still, when new deployment happens, we want the new deployment to retry this CRD
		rr.ControllerRes.RequeueAfter = MaxRequeueInterval
		return rr
	}

	rr.Skipped = !in.GetStatus().ShouldReconcile(hash, log)
	if rr.Skipped {
		// accurate skip reason is logged by ShouldReconcile
		return rr
	}

	rr = r.Handler.Reconcile(ctx, log, in)
	if rr.Skipped {
		// do not take any action here if impl asked to skip status+hash updates (maybe CR is meant for a diff bento)
		// endReconcile is still called - logging done inside
		return rr
	}

	updateErr := r.updateStatus(ctx, log, in, rr)
	if updateErr != nil {
		// If we fail to update status of the CRD, this is very likely a permission issue.
		// Override handler's decision in this case and proceed with slow retry.
		rr.ReconcileErr = updateErr
		rr.ControllerRes.RequeueAfter = MaxRequeueInterval
	}

	// logging done inside endReconcile
	return rr
}

// updateStatus is called to update CRD status based on reconcileErr
func (r *Reconciler) updateStatus(
	ctx context.Context,
	log logrus.FieldLogger,
	in resources.Resource,
	rr *ReconcileResult,
) error {
	// The spec is very unlikely to be changed by the controller, yet recalculate the hash just in case.
	hash, err := in.GetSpec().Hash()
	if err != nil {
		log.WithError(err).Error("failed to calculate hash for the PostgresqlDevenvDatabaseSpec")
		// logging done inside endReconcile
		return err
	}

	// update the CR's status with the reconcile hash and reconcileErr (if present)
	in.GetStatus().Update(hash, rr.ReconcileErr)
	// capture reconcile fail count so far on this hash
	rr.failCount = in.GetStatus().ReconcileFailCount

	err = r.Status().Update(ctx, in)
	if err != nil {
		log.WithError(err).Errorf("unable to update status for PostgresqlDevenvDatabase CR: %+v", in.GetStatus())
	}

	return err
}

// Setup registers the PostgresqlDevenvDatabaseReconciler as a controller to process PostgresqlDevenvDatabase resources.
func (r *Reconciler) Setup(mgr ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(mgr).
		Named(r.Kind).
		For(r.Handler.CreateResource()).
		Complete(r)
	if err != nil {
		r.Log.WithError(err).Error("failed to setup PostgresqlDevenvDatabaseReconciler as a controller for PostgresqlDevenvDatabase resource with k8s manager")
	}
	return err
}

// Close cleans up the controller upon exit
func (r *Reconciler) Close(ctx context.Context) error {
	r.Handler.Close(ctx)
	return nil
}

// getRequeueDuration returns the requeue interval to retry, based on number of times this CR failed so far.
func getRequeueDuration(failCount int) time.Duration {
	// updateStatus sets failCount after updating the CR's status
	// it will always be 1 if reported.
	if failCount == 0 {
		// if we did not set the failCount, updateStatus failed, likely due to permission issues
		return MaxRequeueInterval
	}

	if len(requeueIntervals) <= failCount {
		return requeueIntervals[failCount-1]
	}

	return MaxRequeueInterval
}
