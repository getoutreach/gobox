package regionpicker

// CloudName is a cloud that's able to be discovered
type CloudName string

// Cloud is an interface that returns regions exposed by a cloud
type Cloud interface {
	Regions() []region
}
