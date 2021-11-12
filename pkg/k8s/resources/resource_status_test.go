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
				LastApplySuccessHash: "abc",
				LastApplySuccessTime: afterNow,
			},
		},
		{
			Name: "success over succes",
			Target: resources.ResourceStatus{
				LastApplySuccessHash: "abc",
				LastApplySuccessTime: past,
			},
			Hash: "abc",
			Err:  nil,
			Expected: resources.ResourceStatus{
				LastApplySuccessHash: "abc",
				LastApplySuccessTime: afterNow,
			},
		},
		{
			Name: "success over failure",
			Target: resources.ResourceStatus{
				LastApplySuccessHash: "abc",
				LastApplySuccessTime: past,
				LastApplyError:       "oops",
				LastApplyErrorHash:   "errhash",
				LastApplyErrorTime:   past,
				ApplyFailCount:       4,
			},
			Hash: "def",
			Err:  nil,
			Expected: resources.ResourceStatus{
				LastApplySuccessHash: "def",
				LastApplySuccessTime: afterNow,
				// success should reset error state
			},
		},
		{
			Name:   "failure over empty",
			Target: resources.ResourceStatus{},
			Hash:   "hhh",
			Err:    anyErr,
			Expected: resources.ResourceStatus{
				LastApplyError:     anyErr.Error(),
				LastApplyErrorHash: "hhh",
				LastApplyErrorTime: afterNow,
				ApplyFailCount:     1,
			},
		},
		{
			Name: "failure over same failure hash",
			Target: resources.ResourceStatus{
				LastApplyError:     "can be a diff err",
				LastApplyErrorHash: "hhh",
				LastApplyErrorTime: past,
				ApplyFailCount:     4,
			},
			Hash: "hhh",
			Err:  anyErr,
			Expected: resources.ResourceStatus{
				LastApplyError:     anyErr.Error(),
				LastApplyErrorHash: "hhh",
				LastApplyErrorTime: afterNow,
				// should increase fail count if hash did not change
				ApplyFailCount: 5,
			},
		},
		{
			Name: "failure over diff failure hash",
			Target: resources.ResourceStatus{
				LastApplyError:     "can be a diff err",
				LastApplyErrorHash: "other",
				LastApplyErrorTime: past,
				ApplyFailCount:     4,
			},
			Hash: "hhh",
			Err:  anyErr,
			Expected: resources.ResourceStatus{
				LastApplyError:     anyErr.Error(),
				LastApplyErrorHash: "hhh",
				LastApplyErrorTime: afterNow,
				// should restart fail count if hash changes
				ApplyFailCount: 1,
			},
		},
		{
			Name: "failure over success",
			Target: resources.ResourceStatus{
				LastApplySuccessHash: "abc",
				LastApplySuccessTime: past,
			},
			Hash: "hhh",
			Err:  anyErr,
			Expected: resources.ResourceStatus{
				// failed apply MUST preserve pass success time/hash for observers to know that this resource's past version might
				// still be fully operational and shall be tried (while new updates fail)
				LastApplySuccessHash: "abc",
				LastApplySuccessTime: past,
				LastApplyError:       anyErr.Error(),
				LastApplyErrorHash:   "hhh",
				LastApplyErrorTime:   afterNow,
				ApplyFailCount:       1,
			},
		},
	}

	for _, tt := range tests {
		tt.Target.Update(tt.Hash, tt.Err)
		assert.Equal(t, tt.Expected.ApplyFailCount, tt.Target.ApplyFailCount, tt.Name)
		assert.Equal(t, tt.Expected.LastApplyError, tt.Target.LastApplyError, tt.Name)
		assert.Equal(t, tt.Expected.LastApplyErrorHash, tt.Target.LastApplyErrorHash, tt.Name)
		assert.Equal(t, tt.Expected.LastApplySuccessHash, tt.Target.LastApplySuccessHash, tt.Name)

		if tt.Expected.LastApplyErrorTime == afterNow {
			// should be in range [afterNow, Now()]
			assertInRange(t, tt.Target.LastApplyErrorTime.Time, afterNow.Time, time.Now(), tt.Name)
		} else {
			assert.Equal(t, tt.Target.LastApplyErrorTime, tt.Expected.LastApplyErrorTime, tt.Name)
		}
		if tt.Expected.LastApplySuccessTime == afterNow {
			// should be in range [afterNow, Now()]
			assertInRange(t, tt.Target.LastApplySuccessTime.Time, afterNow.Time, time.Now(), tt.Name)
		} else {
			assert.Equal(t, tt.Target.LastApplySuccessTime, tt.Expected.LastApplySuccessTime, tt.Name)
		}
	}
}

// assertInRange enforces that target time is in range [from, to], both inclusive
func assertInRange(t *testing.T, target, from, to time.Time, name string) {
	assert.Check(t, !from.After(target) && !to.Before(target), name)
}

func TestShouldApply(t *testing.T) {
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
				LastApplySuccessHash: "abc",
				LastApplySuccessTime: past,
			},
			Hash:     "abc",
			Expected: false,
		},
		{
			Name: "on diff success hash",
			Target: resources.ResourceStatus{
				LastApplySuccessHash: "abc",
				LastApplySuccessTime: past,
			},
			Hash:     "def",
			Expected: true,
		},
		{
			Name: "on same failure hash, fail count within limits",
			Target: resources.ResourceStatus{
				LastApplyErrorHash: "abc",
				LastApplyErrorTime: past,
				ApplyFailCount:     2,
			},
			Hash:     "abc",
			Expected: true,
		},
		{
			// TODO(nissimn)[QSS-QSS-818]: allow two retries for now, need retry with expo backoff + config for the backoff
			Name: "on same failure hash, fail count exceeded",
			Target: resources.ResourceStatus{
				LastApplyErrorHash: "abc",
				LastApplyErrorTime: past,
				ApplyFailCount:     3,
			},
			Hash:     "abc",
			Expected: false,
		},
	}

	log := logrus.New()

	for _, tt := range tests {
		shouldApply := tt.Target.ShouldApply(tt.Hash, log)
		assert.Equal(t, shouldApply, tt.Expected, tt.Name)
	}
}
