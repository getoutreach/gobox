package resources

import "sigs.k8s.io/controller-runtime/pkg/client"

// ResourceSpec allows generic Reconciler's implementation to get Hash value of the CR's Spec.
// Hash is used to determine if spec has changed since the last status update or not.
type ResourceSpec interface {
	Hash() (string, error)
}

// Resource represents the custom resource object.
type Resource interface {
	client.Object
	GetSpec() ResourceSpec
	GetStatus() *ResourceStatus
}
