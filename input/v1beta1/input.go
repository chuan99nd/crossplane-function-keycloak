// Package v1beta1 contains the input type for this Function
// +kubebuilder:object:generate=true
// +groupName=template.fn.crossplane.io
// +versionName=v1beta1
package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FunctionType string

const (
	FunctionTypeFetchUser   FunctionType = "FetchUser"
	FunctionTypeDedupeUsers FunctionType = "DedupeUsers"
)

// This isn't a custom resource, in the sense that we never install its CRD.
// It is a KRM-like object, so we generate a CRD to describe its schema.

// TODO: Add your input type here! It doesn't need to be called 'Input', you can
// rename it to anything you like.

// Input can be used to provide input to this Function.
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:categories=crossplane
type Input struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	FunctionType FunctionType `json:"functionType"`

	GroupList   `json:"groupList,omitempty"`
	OutputField string `json:"outputField,omitempty"`

	GroupsPriority []TransformData `json:"groupsPriority,omitempty"`
}

type GroupList struct {
	FromCompositeField string `json:"fromCompositeField,omitempty"`
}

type TransformData struct {
	FromPathsList []string `json:"fromPathsList"`
	ToPath        string   `json:"toPath"`
}
