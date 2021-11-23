// Copyright 2021 Outreach Corporation. All Rights Reserved.

// Description: This file defines a common Reconciler implementation for kubernetes controllers that use ResourceStatus.
package controllers

import (
	"context"

	"github.com/getoutreach/gobox/pkg/k8s/resources"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
	// the Handler's Reconcile method and also if resource is Not Found (with NotFound flag = true).
	// This method is for logging and metrics, ReconcileResult is intentionally passed by value so there is no point modifying it.
	EndReconcile(ctx context.Context, log logrus.FieldLogger, rr ReconcileResult)

	// NotFound callback is called when resource is detected as Not Found. Be careful handling deleted database objects as they
	// can lead to accidental and total data loss!
	// Note: NotFound callback does not have to set NotFound flag on the result (although no harm doing so because
	// the infra will set it on ReconcileResult right after invoking the NotFound callback anyway).
	NotFound(ctx context.Context, log logrus.FieldLogger, resourceName types.NamespacedName) ReconcileResult
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
	// ReconcileErr is set when reconciliation fails, or when status changes fail.
	ReconcileErr error
	// Skipped with no ReconcileErr indicates that reconciler has decided to skip this CR.
	// Skipped with ReconcileErr indicates that reconciler has decided to permanently fail and skip further
	// retries for this CR, until its spec changes again. This is rare...
	Skipped bool
	// PropagateErr is set when reconciler error needs to be reported as a failure to the controller and retry immedaitely
	// (rather than in intervals). Do not set PropagateErr for permanent errors.
	PropagateErr bool
	// ControllerRes is the result to be returned back to the controller's infra.
	ControllerRes ctrl.Result
	// NotFound indicates that CR has been deleted (this might be the last reconcile call on this CR).
	// If true, Handler.NotFound callback is invoked instead of the regular Reconcile.
	// Handler does not have to (and should not) set it - it is set by the infra before calling EndReconcile.
	NotFound bool
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

	if rr.NotFound {
		log.Info("Reconciler has finished processing previously deleted resource.")
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

	r.endReconcile(ctx, log, *rr)

	if rr.ReconcileErr != nil && rr.PropagateErr {
		// Controlelr infra will retry immediately and we already enforced Requeue.
		return rr.ControllerRes, rr.ReconcileErr
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
		if client.IgnoreNotFound(getErr) == nil {
			// If CR has been deleted, we invoke Handler.NotFound callback instead of the regular Reconcile one.
			return r.notFound(ctx, log, req.NamespacedName)
		}

		// this is likely a controller permission issue
		rr.ControllerRes.RequeueAfter = MaxRequeueInterval()
		rr.ReconcileErr = getErr
		return &rr
	}

	var hash string
	hash, rr.ReconcileErr = in.GetSpec().Hash()
	if rr.ReconcileErr != nil {
		log.WithError(rr.ReconcileErr).Error("failed to calculate hash")
		// this is very likely a permanent error, no need to retry too frequently
		// still, if new deployment happens, we want the new deployment to retry this CR
		rr.ControllerRes.RequeueAfter = MaxRequeueInterval()
		return &rr
	}

	// Status changes trigger Reconcile events, ignore those if spec did not change.
	// Also, if reconcile is 'scheduled', ShouldReconcile will skip and set Requeue to the delta time left.
	res := in.GetStatus().ShouldReconcile(hash, log)
	if !res.Reconcile {
		// accurate skip reason is logged by ShouldReconcile
		rr.Skipped = true
		// We do not update status in this case (doing so triggers another reconcile event).
		// Instead, if ShouldReconcile asked for requeue (to continue processing at scheduled time), we report it
		// bacl to the controller to requeue it.
		rr.ControllerRes.RequeueAfter = res.Requeue // can be 0 (no requeue)
		return &rr
	}

	rr = r.handler.Reconcile(ctx, log, in)
	if rr.ReconcileErr == nil && rr.Skipped {
		// do not take any action here if impl asked to skip status+hash updates (maybe CR is meant for a diff bento)
		// endReconcile is still called - logging done inside
		return &rr
	}

	// update status and schedule next one if/as needed
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

	// update the requeue settings in case next reconcile should be scheduled
	r.processResult(rr, in.GetStatus().ReconcileFailCount)

	// update the CR's next requeue, we should do it on success and failure (e.g. reconciler
	// can decide to skip and force the requeue if it was triggered before scheduled time)
	in.GetStatus().UpdateSchedule(rr.ControllerRes)

	// persist the status
	err = r.Client().Status().Update(ctx, in)
	if err != nil {
		log.WithError(err).Errorf("unable to update status for %s CR: %+v", r.Kind(), in.GetStatus())
	}

	return err
}

// processResult calculates the requeue interval if such is needed based on the result and fail count so far
func (r *Reconciler) processResult(rr *ReconcileResult, failCount int) {
	if rr.ReconcileErr == nil {
		// If no error reported, reconciler may still request requeue. Whether it does or not - honor its decision.
		return
	}

	// error case

	if rr.PropagateErr {
		// Controlelr infra will retry immediately. In order for this retry to be unblocked, set the
		// Requeue flag internally to ensure Status sets NextReconcileTime to now(). If we do not do so,
		// next reconcile will start and do nothing due to ShouldReconcile rejecting it.
		if !rr.ControllerRes.Requeue && rr.ControllerRes.RequeueAfter == 0 {
			// Note: there is no point in propagating err back if requeue is not needed, so forcing it.
			// If controller just wants to complain about permanent err and not requeue, do NOT set PropagateErr.
			rr.ControllerRes.Requeue = true
		}
		return
	}

	// Note: if CR has been deleted and Handler.NotFound returns an error, we want to retry cause Handler probably
	// tried to do something (send email, or cleanup or whatever action it takes on Deleted CR) - and failed.
	// If handler is a no-op on Delete, than no error and we won't retry.

	if rr.Skipped {
		// no further action needed. Even if failed - honor Reconcile decision and do not override requeue (if not set)
		return
	}

	// if we do not report error to the controller, we need to requeue for this CR
	if rr.ControllerRes.Requeue || rr.ControllerRes.RequeueAfter > 0 {
		// Reconcile already asked for requeue, honor its value
		return
	}

	// Reconcile reported an error and did not ask for a requeue.
	// This is OK, we will requeue with incrementing intervals.
	rr.ControllerRes.RequeueAfter = requeueDuration(failCount)
}

// notFound handles the case resource is not found (e.g. deleted) in k8s
func (r *Reconciler) notFound(ctx context.Context, log logrus.FieldLogger, resourceName types.NamespacedName) *ReconcileResult {
	// If the CR is deleted in k8s, we have several choices:
	// * Completely cleanup all resources - this can be dangerous since it can lead to huge data loss on accidental CR deletion
	// * Keep retrying every hour hoping CR is recreated. This is also not the best choice cause maybe we realy want the CR
	//   to be gone - we do not want operators to loop on the missing CRs forever (and we do not have a storage to know how
	//   many times we retired so far).
	// * Log only and do nothing (do not return error so no furher processing will be done on the CR). This is prob the best choice
	//   for now.
	r.log.Errorf("Resource %s is Not Found!", resourceName)
	rr := r.handler.NotFound(ctx, log, resourceName)
	// status update not possible on this CR, ensure flag is set to skip it and let EndReconcile get full result
	rr.NotFound = true
	return &rr
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
