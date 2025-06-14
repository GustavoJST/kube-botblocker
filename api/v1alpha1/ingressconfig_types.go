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

// ProtectedIngressStats defines the statistics for ingresses protected by an IngressConfig.
type ProtectedIngressStats struct {
	// Total number of Ingresses that are configured to use this IngressConfig.
	// +optional
	Total int32 `json:"total,omitempty"`

	// Number of Ingresses that have been successfully reconciled with the latest IngressConfig spec.
	// This count is reset to 0 when the IngressConfig spec changes and increments as Ingresses are updated.
	// +optional
	Updated int32 `json:"updated,omitempty"`
}

// IngressConfigStatus defines the observed state of IngressConfig.
type IngressConfigStatus struct {
	// Statistics about the reconcile process for IngressConfig.
	// +optional
	ProtectedIngress ProtectedIngressStats `json:"protectedIngress,omitempty"`

	// LastUpdated is the timestamp when the IngressConfig spec was last modified,
	// triggering a potential reconciliation of associated Ingresses.
	// This field is updated when the .spec of IngressConfig changes.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// Conditions provide observations of the IngressConfig's state.
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// SpecHash is the SHA256 hash of the .spec field of the IngressConfig.
	// +optional
	SpecHash string `json:"specHash,omitempty"`

	// ObservedGeneration is the most recent generation observed for this IngressConfig.
	// It corresponds to the IngressConfig's generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"UpdateSucceeded\")].status"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"UpdateSucceeded\")].message"
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
)
