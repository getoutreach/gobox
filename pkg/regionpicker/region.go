package regionpicker

// RegionName is the name of region
type RegionName string

type region struct {
	// name is the name of this region
	name RegionName

	// endpoint is the endpoint to test against
	endpoint string
}
