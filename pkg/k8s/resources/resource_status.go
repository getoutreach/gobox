package resources

import (
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceStatus holds fields shared across all CR status sub-resources.
//+kubebuilder:object:generate=true
type ResourceStatus struct {
	// LastApplySuccessHash holds the spec hash of the last successfull application of this CR by the operator.
	// If empty, operator never applied this CR successfully. This field is NOT reset on failures.
	LastApplySuccessHash string `json:"lastApplySuccessHash"`

	// LastApplySuccessTime holds the time of the last successfull application of this CR by the operator.
	// If empty, operator never applied this CR successfully. This field is NOT reset on failures.
	LastApplySuccessTime *metav1.Time `json:"lastApplySuccessTime,omitempty"`

	// LastApplyError holds the final error message reported by the operator upon failed application of this CR.
	// This field is reset if operator succeeds to apply the CR.
	LastApplyError string `json:"lastApplyError"`

	// LastApplyErrorHash holds the spec hash of the last failed application of this CR.
	// This field is reset if operator succeeds to apply the CR.
	LastApplyErrorHash string `json:"lastApplyErrorHash"`

	// LastApplyErrorTime holds the time of the last failed application of this CR.
	// This field is reset to Epoch if operator succeeds to apply this CR.
	LastApplyErrorTime *metav1.Time `json:"lastApplyErrorTime,omitempty"`

	// ApplyFailCount holds number of tries current CR spec application failed (so far).
	// This counter is reset when CR spec changes or when application succeeds.
	ApplyFailCount int `json:"applyFailCount"`
}

func (rs *ResourceStatus) ShouldApply(hash string, log logrus.FieldLogger) bool {
	if hash == rs.LastApplySuccessHash {
		// this CR is already applied, this is an echo Reconcile from the status change
		return false
	}

	if hash == rs.LastApplyErrorHash {
		// TODO(nissimn)[QSS-QSS-818]: allow two retries for now, need retry with expo backoff + config for the backoff
		if rs.ApplyFailCount > 2 {
			log.Error("ApplyFailCount is %d and it is exceeded its limit", rs.ApplyFailCount)
			return false
		}

		log.Infof("ApplyFailCount is %d, retrying", rs.ApplyFailCount)
		return true
	}

	log.Infof("Received new CR spec with hash %s", hash)
	return true
}

// Update refreshes the resource status fields based on the success of failure of the reconcile operation.
func (rs *ResourceStatus) Update(hash string, err error) {
	now := metav1.Now()
	if err != nil {
		if hash == rs.LastApplyErrorHash {
			rs.ApplyFailCount++
		} else {
			rs.ApplyFailCount = 1
		}
		// TODO(nissimn)[QSS-QSS-818]: set ctrl.Result.RequeueAfter with time increasing using the ApplyFailCount so far (upcoming PR).
		// Need to check how retry works if Status triggers CR change and also the ctrl asks to RequeueAfter on current version.

		rs.LastApplyError = err.Error()
		rs.LastApplyErrorHash = hash
		rs.LastApplyErrorTime = &now
		// leaving LastApplySuccess* fields as is for other components to know when past version of this CR was applied successfully
	} else {
		rs.LastApplySuccessHash = hash
		rs.LastApplySuccessTime = &now

		rs.ApplyFailCount = 0
		rs.LastApplyError = ""
		rs.LastApplyErrorHash = ""
		rs.LastApplyErrorTime = nil
	}
}
