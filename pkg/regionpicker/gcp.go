package regionpicker

const (
	CloudGCP CloudName = "gcp"
)

type GCP struct{}

// Regions returns a list of all known-gcp regions. This should not be used
// over native GCP region fetching.
// IDEA: One day we could probably get this from GCP. That is authenticated though.
func (*GCP) Regions() []region {
	return []region{
		{name: "us", endpoint: "https://us-docker.pkg.dev"},
		{name: "eu", endpoint: "https://us-docker.pkg.dev"},
	}
}
