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

package vswitchcfg

import (
	"errors"
	"fmt"

	kovs "github.com/kube-ovs/kube-ovs/apis/generated/clientset/versioned"
	kovsv1alpha1 "github.com/kube-ovs/kube-ovs/apis/kubeovs/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

type vswitchConfig struct {
	overlayType string

	kovsClient kovs.Interface
	kubeClient kubernetes.Interface
}

func NewVSwitchConfigController(kubeClient kubernetes.Interface, kovsClient kovs.Interface, overlayType string) *vswitchConfig {
	return &vswitchConfig{
		overlayType: overlayType,
		kovsClient:  kovsClient,
		kubeClient:  kubeClient,
	}
}

func (v *vswitchConfig) OnAdd(obj interface{}) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		klog.Errorf("obj %v was not core/v1 node", obj)
		return
	}

	err := v.syncVSwitchConfig(node)
	if err != nil {
		klog.Errorf("error creating VSwitchConfig: %v", err)
	}
}

func (v *vswitchConfig) OnUpdate(oldObj, newObj interface{}) {
	node, ok := newObj.(*corev1.Node)
	if !ok {
		klog.Errorf("obj %v was not core/v1 node", newObj)
	}

	err := v.syncVSwitchConfig(node)
	if err != nil {
		klog.Errorf("error creating VSwitchConfig: %v", err)
	}
}

func (v *vswitchConfig) OnDelete(obj interface{}) {
	_, ok := obj.(*corev1.Node)
	if !ok {
		klog.Errorf("obj %v was not core/v1 node", obj)
	}

	// TODO: delete VSwitchConfig if node is deleted
}

func (v *vswitchConfig) syncVSwitchConfig(node *corev1.Node) error {
	// TODO: validate overly IP
	overlayIP, err := nodeOverlayIP(node)
	if err != nil {
		return fmt.Errorf("error getting overlay IP for node %q, err: %v", node.Name, err)
	}

	vswitchCfg := nodeToVSwitchConfig(node, v.overlayType, overlayIP)
	_, err = v.kovsClient.KubeovsV1alpha1().VSwitchConfigs().Create(vswitchCfg)
	if err == nil {
		return nil
	}

	if !apierr.IsAlreadyExists(err) {
		return fmt.Errorf("error creating VSwitchConfig for node %q, err: %v", node.Name, err)
	}

	_, err = v.kovsClient.KubeovsV1alpha1().VSwitchConfigs().Update(vswitchCfg)
	if err != nil {
		return fmt.Errorf("error updating VSwitchConfig: %v", err)
	}

	return nil
}

func nodeToVSwitchConfig(node *corev1.Node, overlayType, overlayIP string) *kovsv1alpha1.VSwitchConfig {
	return &kovsv1alpha1.VSwitchConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: node.Name,
		},
		Spec: kovsv1alpha1.VSwitchConfigSpec{
			OverlayIP:   overlayIP,
			OverlayType: overlayType,
		},
	}
}

func nodeOverlayIP(node *corev1.Node) (string, error) {
	var internalIP, externalIP string

	nodeAddresses := node.Status.Addresses
	for _, nodeAddress := range nodeAddresses {
		if nodeAddress.Type == corev1.NodeInternalIP {
			internalIP = nodeAddress.Address
		}

		if nodeAddress.Type == corev1.NodeExternalIP {
			externalIP = nodeAddress.Address
		}
	}

	if internalIP != "" {
		return internalIP, nil
	}

	if externalIP != "" {
		return externalIP, nil
	}

	return "", errors.New("no valid node IP found for tunnel overlay")
}
