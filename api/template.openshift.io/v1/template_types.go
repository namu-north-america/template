/*
Copyright 2025.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Template is the Schema for the templates API.
// It mimics the OpenShift template structure with top-level 'objects' and 'parameters'.
type Template struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Objects contains a list of resources to be instantiated when processing the template.
	// +kubebuilder:pruning:PreserveUnknownFields
	Objects []runtime.RawExtension `json:"objects,omitempty"`

	// Parameters defines the input parameters for this template.
	// +optional
	Parameters []TemplateParameter `json:"parameters,omitempty"`

	// Status defines the observed state of the template.
	Status TemplateStatus `json:"status,omitempty"`
}

// TemplateParameter defines a template input parameter.
type TemplateParameter struct {
	// Name is the parameter name.
	Name string `json:"name"`

	// Description of the parameter.
	// +optional
	Description string `json:"description,omitempty"`

	// Value is the default value.
	// +optional
	Value string `json:"value,omitempty"`

	// From is an expression used to generate the value.
	// +optional
	From string `json:"from,omitempty"`

	// Generate indicates how the value should be generated.
	// +optional
	Generate string `json:"generate,omitempty"`
}

// TemplateStatus holds observed state (optional).
type TemplateStatus struct {
	// Define fields here if needed.
}

//+kubebuilder:object:root=true

// TemplateList contains a list of Template.
type TemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Template `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Template{}, &TemplateList{})
}
