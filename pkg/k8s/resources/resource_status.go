package resources

import (
	"time"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ResourceStatus holds fields shared across all CR status sub-resources.
//+kubebuilder:object:generate=true
type ResourceStatus struct {
	// LastReconcileSuccessHash holds the spec hash of the last successfull application of this CR by the operator.
	// If empty, operator never applied this CR successfully. This field is NOT reset on failures.
	LastReconcileSuccessHash string `json:"lastReconcileSuccessHash"`

	// LastReconcileSuccessTime holds the time of the last successfull application of this CR by the operator.
	// If empty, operator never applied this CR successfully. This field is NOT reset on failures.
	LastReconcileSuccessTime metav1.Time `json:"lastReconcileSuccessTime"`

	// LastReconcileError holds the final error message reported by the operator upon failed application of this CR.
	// This field is reset if operator succeeds to reconcile the CR.
	LastReconcileError string `json:"lastReconcileError"`

	// LastReconcileErrorHash holds the spec hash of the last failed application of this CR.
	// This field is reset if operator succeeds to reconcile the CR.
	LastReconcileErrorHash string `json:"lastReconcileErrorHash"`

	// LastReconcileErrorTime holds the time of the last failed application of this CR.
	// This field is reset to zero (stored as JSON null) if operator succeeds to reconcile this CR.
	LastReconcileErrorTime metav1.Time `json:"lastReconcileErrorTime"`

	// NextReconcileTime holds the time of the next scheduled application of this CR.
	// This field is reset to zero (stored as JSON null) if operator succeeds to reconcile this CR or
	// if reconcile failure was deemed permanent.
	// We need this field for two purposes:
	// * Reconcile is triggered on status updates, in order to truly 'delay' the reconcile we set the status
	//   first (with reconcile status and NextReconcileTime), and then when it reconciles due to status change
	//   we requeue it based on the 'delta' time left until NextReconcileTime.
	// * In case of an error, we can use status to find out when next reconcile try is due. It can also be used
	//   to cancel pending reconcile (by setting NextReconcileTime to null) or reschedule it to a different time.
	NextReconcileTime metav1.Time `json:"nextReconcileTime"`

	// ReconcileFailCount holds number of tries current CR spec application failed (so far).
	// This counter is reset when CR spec changes or when application succeeds.
	ReconcileFailCount int `json:"reconcileFailCount"`
}

// Helper struct to indicate reconcile check status.
type ShouldReconcileResult struct {
	// Reconcile is true if controller must proceed with reconcile.
	Reconcile bool
	// Requeue is set to non-zero if controller must delay the reconcile without updating the status.
	// Requeue is ignored if Reconcile is true (so no use to set both).
	Requeue time.Duration
}

func (rs *ResourceStatus) ShouldReconcile(hash string, log logrus.FieldLogger) ShouldReconcileResult {
	if hash == rs.LastReconcileSuccessHash {
		// this CR is already applied, this is an echo Reconcile from the status change
		log.Infof("Received already applied CR with unchanged hash %s, skipping.", hash)
		return ShouldReconcileResult{}
	}

	if hash != rs.LastReconcileErrorHash {
		log.Infof("Received new CR spec with hash %s, processing.", hash)
		// always reconcile
		return ShouldReconcileResult{Reconcile: true}
	}

	// This CR has failed in past. This case is tricky, cause status updates also trigger reconcile.
	// If spec hash did not change, we must check NextReconcileTime to decide if it this CR is due to retry.
	if rs.NextReconcileTime.IsZero() {
		// previous failure was permanent - thus no retry until spec changes
		log.Warnf("Previous failure on this CR did not schedule retry, skipping.")
		return ShouldReconcileResult{}
	}

	timeLeft := time.Until(rs.NextReconcileTime.Time)
	if timeLeft > 0 {
		// we still have time left till next reconcile
		log.Infof("Received reconcile, but it is not due yet, rescheduling in %s", timeLeft)
		return ShouldReconcileResult{Requeue: timeLeft}
	}

	log.Infof("ReconcileFailCount is %d, retrying now.", rs.ReconcileFailCount)
	return ShouldReconcileResult{Reconcile: true}
}

// Update refreshes the resource status fields based on the success of failure of the reconcile operation.
func (rs *ResourceStatus) Update(hash string, err error) {
	if err != nil {
		if hash == rs.LastReconcileErrorHash {
			rs.ReconcileFailCount++
		} else {
			rs.ReconcileFailCount = 1
		}

		rs.LastReconcileError = err.Error()
		rs.LastReconcileErrorHash = hash
		rs.LastReconcileErrorTime = metav1.Now()
		// leaving LastReconcileSuccess* fields as is for other components to know when past version of this CR was applied successfully
	} else {
		rs.LastReconcileSuccessHash = hash
		rs.LastReconcileSuccessTime = metav1.Now()

		rs.ReconcileFailCount = 0
		rs.LastReconcileError = ""
		rs.LastReconcileErrorHash = ""
		rs.LastReconcileErrorTime = metav1.Time{} // epoch is marshaled as null
	}
}

// UpdateSchedule sets the NextReconcileTime based on the controller's result.
func (rs *ResourceStatus) UpdateSchedule(res ctrl.Result) {
	switch {
	case res.RequeueAfter > 0:
		rs.NextReconcileTime = metav1.Time{Time: metav1.Now().Add(res.RequeueAfter)}
	case res.Requeue:
		rs.NextReconcileTime = metav1.Now()
	default:
		rs.NextReconcileTime = metav1.Time{} // zero is marshaled as null
	}
}
