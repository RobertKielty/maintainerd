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

// CodeScannerSnykSpec defines the desired state of CodeScannerSnyk
type CodeScannerSnykSpec struct {
	// ProjectName is the name of the CNCF project to scan
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ProjectName string `json:"projectName"`

	// ConfigMapName is the name of the ConfigMap to create for this scanner
	// +optional
	ConfigMapName string `json:"configMapName,omitempty"`
}

// CodeScannerSnykStatus defines the observed state of CodeScannerSnyk.
type CodeScannerSnykStatus struct {
	// ConfigMapRef is the namespace/name reference to the created ConfigMap
	// +optional
	ConfigMapRef string `json:"configMapRef,omitempty"`

	// Conditions represent the latest available observations of the resource's state
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Project",type=string,JSONPath=`.spec.projectName`
// +kubebuilder:printcolumn:name="ConfigMap",type=string,JSONPath=`.status.configMapRef`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// CodeScannerSnyk is the Schema for the codescannersnyks API
type CodeScannerSnyk struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of CodeScannerSnyk
	// +required
	Spec CodeScannerSnykSpec `json:"spec"`

	// status defines the observed state of CodeScannerSnyk
	// +optional
	Status CodeScannerSnykStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// CodeScannerSnykList contains a list of CodeScannerSnyk
type CodeScannerSnykList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []CodeScannerSnyk `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CodeScannerSnyk{}, &CodeScannerSnykList{})
}
