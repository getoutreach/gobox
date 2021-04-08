package differs

import "reflect"

// AnyMap allows any map to be matched against it when
// differs.Custom is passed to cmp
func AnyMap() CustomComparer {
	return Customf(func(o interface{}) bool {
		return reflect.ValueOf(o).Kind() == reflect.Map
	})
}
