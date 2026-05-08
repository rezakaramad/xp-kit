package xtenant

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
type XTenant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              XTenantSpec   `json:"spec"`
	Status            XTenantStatus `json:"status,omitempty"`
}

type XTenantSpec struct {
	// your fields here
}

type XTenantStatus struct {
	// your fields here
}
