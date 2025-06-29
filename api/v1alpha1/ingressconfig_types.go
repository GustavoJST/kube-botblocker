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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IngressConfigSpec defines the desired state of IngressConfig.
type IngressConfigSpec struct {
	// List of User-Agents to be added to the blocklist in each protected Ingress
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Required
	// +listType=set
	BlockedUserAgents []string `json:"blockedUserAgents"`
}

// IngressConfigStatus defines the observed state of IngressConfig.
type IngressConfigStatus struct {
	// LastUpdated is the timestamp when the IngressConfig spec was last modified,
	// triggering a potential reconciliation of associated Ingresses.
	// This field is updated when the .spec of IngressConfig changes.
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// LastConditionStatus is the status of the last Condition applied to a IngressConfig object
	LastConditionStatus metav1.ConditionStatus `json:"lastConditionStatus,omitempty"`

	// LastConditionStatus is the message of the last Condition applied to a IngressConfig object
	LastConditionMessage string `json:"lastConditionMessage,omitempty"`

	// Conditions provide observations of the IngressConfig's state.
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// SpecHash is the SHA256 hash of the .spec field of the IngressConfig.
	SpecHash string `json:"specHash,omitempty"`

	// ObservedGeneration is the most recent generation observed for this IngressConfig.
	// It corresponds to the IngressConfig's generation.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.lastConditionStatus"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.lastConditionMessage"
// +kubebuilder:printcolumn:name="Last Updated",type="date",JSONPath=".status.lastUpdated"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// IngressConfig is the Schema for the ingressconfigs API.
type IngressConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IngressConfigSpec   `json:"spec,omitempty"`
	Status IngressConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IngressConfigList contains a list of IngressConfig.
type IngressConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IngressConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IngressConfig{}, &IngressConfigList{})
}

const (
	ConditionTypeUpdateSucceeded            string = "UpdateSucceeded"
	ConditionReasonReconciliationInProgress string = "ReconciliationInProgress"
	ConditionReasonReconciliationSuccessful string = "ReconciliationSuccessful"

	ConditionTypeCleanupSucceeded    string = "CleanupSucceeded"
	ConditionReasonCleanupInProgress string = "CleanupInProgress"
)
