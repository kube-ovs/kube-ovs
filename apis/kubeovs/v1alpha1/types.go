/*
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VSwitchConfig is a specification for the Open vSwitch configuration
type VSwitchConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec VSwitchConfigSpec `json:"spec"`
}

// VSwitchSpec is the spec for the Open vSwitch configuration
type VSwitchConfigSpec struct {
	PodCIDR     string `json:"podCidr"`
	ClusterCIDR string `json:"clusterCidr"`
	ServiceCIDR string `json:"serviceCidr"`

	OverlayIP       string `json:"overlayIP"`
	OverlayType     string `json:"overlayType"`
	OverlayTunnelID int32  `json:"overlayTunnelID"`
}

// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VSwitchConfigList is a list of VSwitchConfig resources
type VSwitchConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []VSwitchConfig `json:"items"`
}
