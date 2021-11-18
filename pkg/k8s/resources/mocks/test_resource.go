package mocks

import (
	"strings"

	"github.com/getoutreach/gobox/pkg/k8s/resources"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

const (
	TestNamespace = "ns"
	TestGroup     = "TestGroup"
	TestKind      = "TestKind"
	TestVer       = "ver"
	InitialHash   = "initial_hash"
)

var (
	// GroupVersion is group version used to register test objects
	GroupVersion = schema.GroupVersion{Group: TestGroup, Version: TestVer}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}
)

// TestResourceSpec serves as a fake spec for the TestResource.
type TestResourceSpec struct {
	FakeHash string
}

// Hash implements ResourceSpec interface
func (s *TestResourceSpec) Hash() (string, error) {
	return s.FakeHash, nil
}

// TestResource can be used as a CRD for Reconciler testing purposes.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type TestResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              TestResourceSpec
	Status            resources.ResourceStatus
}

// GetSpec satisfies resources.Resource interface
func (r *TestResource) GetSpec() resources.ResourceSpec {
	return &r.Spec
}

// GetStatus satisfies resources.Resource interface
func (r *TestResource) GetStatus() *resources.ResourceStatus {
	return &r.Status
}

// GetTestResourceDefinition creates a CRD for the TestResource. This CRD should be registered with the
// (fake) client before using TestResource objects.
func GetTestResourceDefinition() *apiextensions.CustomResourceDefinition {
	return &apiextensions.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TestKind,
			Namespace: TestNamespace,
		},
		Spec: apiextensions.CustomResourceDefinitionSpec{
			Group:   TestGroup,
			Version: TestVer,
			Names: apiextensions.CustomResourceDefinitionNames{
				Kind:     TestKind,
				Plural:   strings.ToLower(TestKind) + "s",
				Singular: strings.ToLower(TestKind),
			},
		},
	}
}

// NewTestResource creates a resource with a given name in the test namespace TestNamespace.
// New test resource has its FakeHash set to mocks.InitialHash and no status.
func NewTestResource(name string) *TestResource {
	return &TestResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: TestNamespace,
		},
		Spec: TestResourceSpec{
			FakeHash: InitialHash,
		},
	}
}

// init registers the test CRD
//nolint:gochecknoinits // Why: must easier to register it once via init than ask each test to do so and maintain state.
func init() {
	SchemeBuilder.Register(&TestResource{})
}
