package resources_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/getoutreach/gobox/pkg/k8s/resources"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUpdate(t *testing.T) {
	past := metav1.Time{Time: time.Now().Add(-5 * time.Minute)}
	afterNow := metav1.Now()
	anyErr := fmt.Errorf("ouch")

	tests := []struct {
		Name     string
		Target   resources.ResourceStatus
		Hash     string
		Err      error
		Expected resources.ResourceStatus
	}{
		{
			Name:   "success on empty",
			Target: resources.ResourceStatus{},
			Hash:   "abc",
			Err:    nil,
			Expected: resources.ResourceStatus{
				LastReconcileSuccessHash: "abc",
				LastReconcileSuccessTime: afterNow,
			},
		},
		{
			Name: "success over succes",
			Target: resources.ResourceStatus{
				LastReconcileSuccessHash: "abc",
				LastReconcileSuccessTime: past,
			},
			Hash: "abc",
			Err:  nil,
			Expected: resources.ResourceStatus{
				LastReconcileSuccessHash: "abc",
				LastReconcileSuccessTime: afterNow,
			},
		},
		{
			Name: "success over failure",
			Target: resources.ResourceStatus{
				LastReconcileSuccessHash: "abc",
				LastReconcileSuccessTime: past,
				LastReconcileError:       "oops",
				LastReconcileErrorHash:   "errhash",
				LastReconcileErrorTime:   past,
				ReconcileFailCount:       4,
			},
			Hash: "def",
			Err:  nil,
			Expected: resources.ResourceStatus{
				LastReconcileSuccessHash: "def",
				LastReconcileSuccessTime: afterNow,
				// success should reset error state
			},
		},
		{
			Name:   "failure over empty",
			Target: resources.ResourceStatus{},
			Hash:   "hhh",
			Err:    anyErr,
			Expected: resources.ResourceStatus{
				LastReconcileError:     anyErr.Error(),
				LastReconcileErrorHash: "hhh",
				LastReconcileErrorTime: afterNow,
				ReconcileFailCount:     1,
			},
		},
		{
			Name: "failure over same failure hash",
			Target: resources.ResourceStatus{
				LastReconcileError:     "can be a diff err",
				LastReconcileErrorHash: "hhh",
				LastReconcileErrorTime: past,
				ReconcileFailCount:     4,
			},
			Hash: "hhh",
			Err:  anyErr,
			Expected: resources.ResourceStatus{
				LastReconcileError:     anyErr.Error(),
				LastReconcileErrorHash: "hhh",
				LastReconcileErrorTime: afterNow,
				// should increase fail count if hash did not change
				ReconcileFailCount: 5,
			},
		},
		{
			Name: "failure over diff failure hash",
			Target: resources.ResourceStatus{
				LastReconcileError:     "can be a diff err",
				LastReconcileErrorHash: "other",
				LastReconcileErrorTime: past,
				ReconcileFailCount:     4,
			},
			Hash: "hhh",
			Err:  anyErr,
			Expected: resources.ResourceStatus{
				LastReconcileError:     anyErr.Error(),
				LastReconcileErrorHash: "hhh",
				LastReconcileErrorTime: afterNow,
				// should restart fail count if hash changes
				ReconcileFailCount: 1,
			},
		},
		{
			Name: "failure over success",
			Target: resources.ResourceStatus{
				LastReconcileSuccessHash: "abc",
				LastReconcileSuccessTime: past,
			},
			Hash: "hhh",
			Err:  anyErr,
			Expected: resources.ResourceStatus{
				// failed reconcile MUST preserve pass success time/hash for observers to know that this resource's past version might
				// still be fully operational and shall be tried (while new updates fail)
				LastReconcileSuccessHash: "abc",
				LastReconcileSuccessTime: past,
				LastReconcileError:       anyErr.Error(),
				LastReconcileErrorHash:   "hhh",
				LastReconcileErrorTime:   afterNow,
				ReconcileFailCount:       1,
			},
		},
	}

	for _, tt := range tests {
		tt.Target.Update(tt.Hash, tt.Err)
		assert.Equal(t, tt.Expected.ReconcileFailCount, tt.Target.ReconcileFailCount, tt.Name)
		assert.Equal(t, tt.Expected.LastReconcileError, tt.Target.LastReconcileError, tt.Name)
		assert.Equal(t, tt.Expected.LastReconcileErrorHash, tt.Target.LastReconcileErrorHash, tt.Name)
		assert.Equal(t, tt.Expected.LastReconcileSuccessHash, tt.Target.LastReconcileSuccessHash, tt.Name)

		if tt.Expected.LastReconcileErrorTime == afterNow {
			// should be in range [afterNow, Now()]
			assertInRange(t, tt.Target.LastReconcileErrorTime.Time, afterNow.Time, time.Now(), tt.Name)
		} else {
			assert.Equal(t, tt.Target.LastReconcileErrorTime, tt.Expected.LastReconcileErrorTime, tt.Name)
		}
		if tt.Expected.LastReconcileSuccessTime == afterNow {
			// should be in range [afterNow, Now()]
			assertInRange(t, tt.Target.LastReconcileSuccessTime.Time, afterNow.Time, time.Now(), tt.Name)
		} else {
			assert.Equal(t, tt.Target.LastReconcileSuccessTime, tt.Expected.LastReconcileSuccessTime, tt.Name)
		}
	}
}

// assertInRange enforces that target time is in range [from, to], both inclusive
func assertInRange(t *testing.T, target, from, to time.Time, name string) {
	assert.Check(t, !from.After(target) && !to.Before(target), name)
}

func TestShouldReconcile(t *testing.T) {
	past := metav1.Time{Time: time.Now().Add(-5 * time.Minute)}

	tests := []struct {
		Name     string
		Target   resources.ResourceStatus
		Hash     string
		Expected bool
	}{
		{
			Name:     "on empty",
			Target:   resources.ResourceStatus{},
			Hash:     "abc",
			Expected: true,
		},
		{
			Name: "on same success hash",
			Target: resources.ResourceStatus{
				LastReconcileSuccessHash: "abc",
				LastReconcileSuccessTime: past,
			},
			Hash:     "abc",
			Expected: false,
		},
		{
			Name: "on diff success hash",
			Target: resources.ResourceStatus{
				LastReconcileSuccessHash: "abc",
				LastReconcileSuccessTime: past,
			},
			Hash:     "def",
			Expected: true,
		},
		{
			Name: "on same failure hash",
			Target: resources.ResourceStatus{
				LastReconcileErrorHash: "abc",
				LastReconcileErrorTime: past,
				// ShouldReconcile does not apply limit on failures, we retry indefinitely with
				// increasing requeue intervals.
				ReconcileFailCount: 125,
			},
			Hash:     "abc",
			Expected: true,
		},
	}

	log := logrus.New()

	for _, tt := range tests {
		shouldReconcile := tt.Target.ShouldReconcile(tt.Hash, log)
		assert.Equal(t, shouldReconcile, tt.Expected, tt.Name)
	}
}
