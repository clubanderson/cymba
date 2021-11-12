/*
Copyright 2021.

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

// PodDeviceSpec defines the desired state of PodDevice
type PodDeviceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of Pod. Edit pod_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// PodDeviceStatus defines the observed state of Pod
type PodDeviceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// PodDevice is the Schema for the poddevice API
type PodDevice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PodDeviceSpec   `json:"spec,omitempty"`
	Status PodDeviceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PodDeviceList contains a list of PodDevices
type PodDeviceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PodDevice `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PodDevice{}, &PodDeviceList{})
}
