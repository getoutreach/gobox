package resources

import (
	"encoding/json"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// epoch is used as an 'absence of value' indicator for timestamps below.
// Storing for nil or golang 'zero' time do not work well as dateTime and cause weird invalid format errors.
// This value is for internal package and store use only, epoch value is translated back to Time{} upon unmarshalling.
var epoch metav1.Time = metav1.Unix(0, 0)

// ResourceStatus holds fields shared across all CR status sub-resources.
//+kubebuilder:object:generate=true
type ResourceStatus struct {
	// LastApplySuccessHash holds the spec hash of the last successfull application of this CR by the operator.
	// If empty, operator never applied this CR successfully. This field is NOT reset on failures.
	LastApplySuccessHash string `json:"lastApplySuccessHash"`

	// LastApplySuccessTime holds the time of the last successfull application of this CR by the operator.
	// If empty, operator never applied this CR successfully. This field is NOT reset on failures.
	LastApplySuccessTime metav1.Time `json:"lastApplySuccessTime"`

	// LastApplyError holds the final error message reported by the operator upon failed application of this CR.
	// This field is reset if operator succeeds to apply the CR.
	LastApplyError string `json:"lastApplyError"`

	// LastApplyErrorHash holds the spec hash of the last failed application of this CR.
	// This field is reset if operator succeeds to apply the CR.
	LastApplyErrorHash string `json:"lastApplyErrorHash"`

	// LastApplyErrorTime holds the time of the last failed application of this CR.
	// This field is reset to Epoch in storage (swapped with zero time in Go) if operator succeeds to apply this CR.
	LastApplyErrorTime metav1.Time `json:"lastApplyErrorTime"`

	// ApplyFailCount holds number of tries current CR spec application failed (so far).
	// This counter is reset when CR spec changes or when application succeeds.
	ApplyFailCount int `json:"applyFailCount"`
}

type resourceStatus ResourceStatus

// MarshalJSON implements a json.Marshaler
func (rs *ResourceStatus) MarshalJSON() ([]byte, error) {
	cp := resourceStatus(*rs)
	if cp.LastApplyErrorTime.IsZero() {
		cp.LastApplyErrorTime = epoch
	}
	if cp.LastApplySuccessTime.IsZero() {
		cp.LastApplySuccessTime = epoch
	}
	return json.Marshal(cp)
}

func (rs *ResourceStatus) UnmarshalJSON(data []byte) error {
	var cp resourceStatus
	if err := json.Unmarshal(data, &cp); err != nil {
		return err
	}

	if cp.LastApplyErrorTime == epoch {
		cp.LastApplyErrorTime = metav1.Time{}
	}
	if cp.LastApplySuccessTime == epoch {
		cp.LastApplySuccessTime = metav1.Time{}
	}

	return nil
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
		rs.LastApplyErrorTime = now
		// leaving LastApplySuccess* fields as is for other components to know when past version of this CR was applied successfully
	} else {
		rs.LastApplySuccessHash = hash
		rs.LastApplySuccessTime = now

		rs.ApplyFailCount = 0
		rs.LastApplyError = ""
		rs.LastApplyErrorHash = ""
		// note: we replcae golang 'zero' time with epoch time before sending it to k8s (and vice versa upon recv)
		rs.LastApplyErrorTime = metav1.Time{}
	}
}
