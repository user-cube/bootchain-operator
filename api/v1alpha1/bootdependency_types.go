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

// ServiceDependency defines a single service that must be reachable before the owner can start.
type ServiceDependency struct {
	// service is the name of the Kubernetes Service to wait for.
	// +kubebuilder:validation:MinLength=1
	Service string `json:"service"`

	// port is the TCP port that must be open on the service.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

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
