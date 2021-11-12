package resources

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"reflect"

	"github.com/pkg/errors"
)

// Hash returns the hash of the subresource fields, it is used to detect changes in the k8s
// subresource such as its Status or Spec.
//
// Generated hash string is URL friendly (JSON -> sha256 -> base64 with URL encoding).
//
// Do NOT call this method for the resource itsef or on objects that change too frequenty;
// for example, all k8s resources have Meta object info that contains generated annotations) -
// their hash will change every time resource changes rendering the hash unusable.
func Hash(subresource interface{}) (string, error) {
	// JSON is the format used by k8s to store spec/status, so it must be supported.
	bytes, err := json.Marshal(subresource)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal subresource of type %s", reflect.TypeOf(subresource))
	}

	sum := sha256.Sum256(bytes)
	// base64 to make it visually readable / easier to compare
	return base64.URLEncoding.EncodeToString(sum[:]), nil
}
