/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HTTPHeader describes a custom header to be sent in HTTP(S) probes.
type HTTPHeader struct {
	// name is the header field name.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// value is the header field value.
	Value string `json:"value"`
}

// ServiceDependency defines a single dependency that must be reachable before the owner can start.
// Exactly one of `service` or `host` must be specified.
// +kubebuilder:validation:XValidation:rule="!has(self.httpScheme) || has(self.httpPath)",message="httpScheme requires httpPath to be set"
// +kubebuilder:validation:XValidation:rule="!has(self.insecure) || !self.insecure || has(self.httpPath)",message="insecure requires httpPath to be set"
// +kubebuilder:validation:XValidation:rule="!has(self.httpMethod) || has(self.httpPath)",message="httpMethod requires httpPath to be set"
// +kubebuilder:validation:XValidation:rule="!has(self.httpHeaders) || size(self.httpHeaders) == 0 || has(self.httpPath)",message="httpHeaders requires httpPath to be set"
// +kubebuilder:validation:XValidation:rule="!has(self.httpExpectedStatuses) || size(self.httpExpectedStatuses) == 0 || has(self.httpPath)",message="httpExpectedStatuses requires httpPath to be set"
type ServiceDependency struct {
	// service is the name of a Kubernetes Service in the same namespace to wait for.
	// Mutually exclusive with host.
	// +kubebuilder:validation:MinLength=1
	// +optional
	Service string `json:"service,omitempty"`

	// host is an external hostname or IP address to wait for.
	// Use this for dependencies outside the cluster (e.g. a managed database, an external API).
	// Mutually exclusive with service.
	// +kubebuilder:validation:MinLength=1
	// +optional
	Host string `json:"host,omitempty"`

	// port is the TCP port that must be open on the dependency.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// httpPath is an optional HTTP(S) path to probe instead of a raw TCP check.
	// When set, the controller and init container perform an HTTP GET to
	// {httpScheme}://{target}:{port}{httpPath} and wait until a 2xx response is received.
	// When omitted, a plain TCP connection check is used.
	// +kubebuilder:validation:Pattern=`^/.*`
	// +optional
	HTTPPath string `json:"httpPath,omitempty"`

	// httpScheme is the URL scheme to use when httpPath is set.
	// Must be "http" or "https". Defaults to "http" when omitted.
	// Only meaningful when httpPath is set.
	// +kubebuilder:validation:Enum=http;https
	// +optional
	HTTPScheme string `json:"httpScheme,omitempty"`

	// insecure controls whether TLS certificate verification is skipped for HTTPS probes.
	// When true, the controller and init container accept any certificate, including
	// self-signed ones. Only meaningful when httpScheme is "https".
	// Defaults to false.
	// +optional
	Insecure bool `json:"insecure,omitempty"`

	// httpMethod is the HTTP verb to use when httpPath is set (e.g. GET, POST, HEAD).
	// Must be an uppercase HTTP method name. Defaults to GET when omitted.
	// Only meaningful when httpPath is set.
	// +kubebuilder:validation:Pattern=`^[A-Z]+$`
	// +optional
	HTTPMethod string `json:"httpMethod,omitempty"`

	// httpHeaders is a list of custom HTTP headers to include in the probe request.
	// Useful for endpoints that require an Authorization header or other custom headers.
	// Only meaningful when httpPath is set.
	// +optional
	HTTPHeaders []HTTPHeader `json:"httpHeaders,omitempty"`

	// httpExpectedStatuses is a list of HTTP status codes that are considered a healthy response.
	// When omitted, any 2xx status code (200–299) is accepted.
	// Use this when a health endpoint returns a non-standard code such as 204 No Content.
	// Only meaningful when httpPath is set.
	// +optional
	HTTPExpectedStatuses []int32 `json:"httpExpectedStatuses,omitempty"`

	// timeout is how long to wait for this dependency before giving up.
	// Defaults to 60s if not specified.
	// +kubebuilder:default="60s"
	// +optional
	Timeout string `json:"timeout,omitempty"`
}

// BootDependencySpec defines the desired state of BootDependency.
type BootDependencySpec struct {
	// dependsOn is the list of services that must be reachable before the
	// Deployment with the same name in this namespace is allowed to start.
	// +kubebuilder:validation:MinItems=1
	DependsOn []ServiceDependency `json:"dependsOn"`
}

// BootDependencyStatus defines the observed state of BootDependency.
type BootDependencyStatus struct {
	// conditions represent the current state of the BootDependency.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// resolvedDependencies is a human-readable summary of how many dependencies
	// are currently reachable, e.g. "2/3".
	// +optional
	ResolvedDependencies string `json:"resolvedDependencies,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Resolved",type="string",JSONPath=".status.resolvedDependencies"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// BootDependency is the Schema for the bootdependencies API
type BootDependency struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of BootDependency
	// +required
	Spec BootDependencySpec `json:"spec"`

	// status defines the observed state of BootDependency
	// +optional
	Status BootDependencyStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// BootDependencyList contains a list of BootDependency
type BootDependencyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []BootDependency `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BootDependency{}, &BootDependencyList{})
}
