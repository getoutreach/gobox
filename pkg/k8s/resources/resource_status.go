package resources

import (
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	// This field is reset to Epoch if operator succeeds to reconcile this CR.
	LastReconcileErrorTime metav1.Time `json:"lastReconcileErrorTime"`

	// ReconcileFailCount holds number of tries current CR spec application failed (so far).
	// This counter is reset when CR spec changes or when application succeeds.
	ReconcileFailCount int `json:"reconcileFailCount"`
}

func (rs *ResourceStatus) ShouldReconcile(hash string, log logrus.FieldLogger) bool {
	if hash == rs.LastReconcileSuccessHash {
		// this CR is already applied, this is an echo Reconcile from the status change
		return false
	}

	if hash == rs.LastReconcileErrorHash {
		log.Infof("ReconcileFailCount is %d, retrying.", rs.ReconcileFailCount)
		return true
	}

	log.Infof("Received new CR spec with hash %s", hash)
	return true
}

// Update refreshes the resource status fields based on the success of failure of the reconcile operation.
func (rs *ResourceStatus) Update(hash string, err error) {
	if err != nil {
		if hash == rs.LastReconcileErrorHash {
			rs.ReconcileFailCount++
		} else {
			rs.ReconcileFailCount = 1
		}
		// TODO(nissimn)[QSS-QSS-818]: set ctrl.Result.RequeueAfter with time increasing using the ReconcileFailCount so far (upcoming PR).
		// Need to check how retry works if Status triggers CR change and also the ctrl asks to RequeueAfter on current version.

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
