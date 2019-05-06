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

package main

import (
	"os"

	kovs "github.com/kube-ovs/kube-ovs/apis/generated/clientset/versioned"
	kovsinformer "github.com/kube-ovs/kube-ovs/apis/generated/informers/externalversions"
	"github.com/kube-ovs/kube-ovs/controllers/tunnel"

	coreinformer "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
)

func main() {
	klog.Info("starting kube-ovs-controller")

	restConfig, err := rest.InClusterConfig()
	if err != nil {
		klog.Errorf("error creating in-cluster config: %v", err)
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		klog.Errorf("error getting kubernetes client: %v", err)
		os.Exit(1)
	}

	kovsClientset, err := kovs.NewForConfig(restConfig)
	if err != nil {
		klog.Errorf("error getting kube-ovs clientset: %v", err)
		os.Exit(1)
	}

	kovsInformerFactory := kovsinformer.NewSharedInformerFactory(kovsClientset, 0)
	vswitchInformer := kovsInformerFactory.Kubeovs().V1alpha1().VSwitchConfigs().Informer()

	coreInformerFactory := coreinformer.NewSharedInformerFactory(clientset, 0)
	nodeInformer := coreInformerFactory.Core().V1().Nodes().Informer()

	kovsInformerFactory.WaitForCacheSync(nil)
	coreInformerFactory.WaitForCacheSync(nil)

	tunnelController := tunnel.NewTunnelIDAllocator(kovsClientset)
	vswitchInformer.AddEventHandler(tunnelController)

	kovsInformerFactory.Start(nil)
	coreInformerFactory.Start(nil)
}