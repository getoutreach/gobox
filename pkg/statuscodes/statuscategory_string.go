// Code generated by "stringer -type=StatusCategory"; DO NOT EDIT.

package statuscodes

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[CategoryOK-1]
	_ = x[CategoryClientError-2]
	_ = x[CategoryServerError-3]
}

const _StatusCategory_name = "CategoryOKCategoryClientErrorCategoryServerError"

var _StatusCategory_index = [...]uint8{0, 10, 29, 48}

func (i StatusCategory) String() string {
	i -= 1
	if i < 0 || i >= StatusCategory(len(_StatusCategory_index)-1) {
		return "StatusCategory(" + strconv.FormatInt(int64(i+1), 10) + ")"
	}
	return _StatusCategory_name[_StatusCategory_index[i]:_StatusCategory_index[i+1]]
}
