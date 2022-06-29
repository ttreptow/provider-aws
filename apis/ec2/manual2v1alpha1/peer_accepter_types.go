/*
Copyright 2021 The Crossplane Authors.

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

package manual2v1alpha1

import (
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VPCPeerConnectionAccepterParameters are custom parameters for VPCPeerConnectionAccepter
type VPCPeerConnectionAccepterParameters struct {
	// Region is the region you'd like your VPC to be created in.
	Region string `json:"region"`

	// The ID of the VPC peering connection. You must specify this parameter in the request.
	// +crossplane:generate:reference:type=github.com/crossplane-contrib/provider-aws/apis/ec2/v1alpha1.VPCPeeringConnection
	VPCPeeringConnectionID *string `json:"vpcPeeringConnectionID,omitempty"`
	// VPCPeeringConnectionIDRef is a reference to an API used to set
	// the VPCPeeringConnectionID.
	// +optional
	VPCPeeringConnectionIDRef *xpv1.Reference `json:"vpcPeeringConnectionIDRef,omitempty"`
	// VPCPeeringConnectionIDSelector selects references to API used
	// to set the VPCPeeringConnectionID.
	// +optional
	VPCPeeringConnectionIDSelector *xpv1.Selector `json:"vpcPeeringConnectionIDSelector,omitempty"`
}

// An VPCPeerConnectionAccepterSpec defines the desired state of VPCPeerConnectionAccepter.
type VPCPeerConnectionAccepterSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       VPCPeerConnectionAccepterParameters `json:"forProvider"`
}

// An VPCPeerConnectionAccepterStatus represents the observed state of VPCPeerConnectionAccepter.
type VPCPeerConnectionAccepterStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          VPCPeerConnectionAccepterObservation `json:"atProvider,omitempty"`
}

// VPCPeerConnectionAccepterObservation keeps the state for the external resource. The below fields
// follow the VPCPeerConnectionAccepterParameters response object as closely as possible.
type VPCPeerConnectionAccepterObservation struct {
	// The status of the VPC peering connection.
	// +optional
	Status *string `json:"status,omitempty"`

	// A message that provides more information about the status, if applicable.
	// +optional
	Message *string `json:"message,omitempty"`

	// The time that an unaccepted VPC peering connection will expire.
	// +optional
	ExpirationTime *metav1.Time `json:"expirationTime,omitempty"`
}

// +kubebuilder:object:root=true

// VPCPeerConnectionAccepter is a managed resource that
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="ID",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.atProvider.state"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,aws}
type VPCPeerConnectionAccepter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VPCPeerConnectionAccepterSpec   `json:"spec"`
	Status VPCPeerConnectionAccepterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VPCPeerConnectionAccepterList contains a list of VPCPeerConnectionAccepter
type VPCPeerConnectionAccepterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VPCPeerConnectionAccepter `json:"items"`
}
