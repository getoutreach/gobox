package log

// Many aggregates marshaling of many items
//
// This avoids having to build an append list and also simplifies code
type Many []Marshaler

// MarshalLog calls MarshalLog on all the individual elements
func (m Many) MarshalLog(addField func(key string, v interface{})) {
	for _, item := range m {
		if item != nil {
			item.MarshalLog(addField)
		}
	}
}
