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
	"fmt"
	"io/ioutil"
	"os"

	"github.com/kube-ovs/kube-ovs/openflow"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
)

const (
	cniConfigPath = "/etc/cni/net.d/10-kube-ovs.json"
)

func main() {
	klog.Info("starting kube-ovs")

	restConfig, err := rest.InClusterConfig()
	if err != nil {
		klog.Errorf("error create in cluster config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		klog.Errorf("error getting kubernetes client: %v", err)
		os.Exit(1)
	}

	// Get the curret snode name. We're going to assume this was passed
	// via an env var called NODE_NAME or the cli flag --node-name
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		klog.Error("env variable NODE_NAME is required")
		os.Exit(1)
	}

	curNode, err := clientset.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("failed to get current node resource: %v", err)
		os.Exit(1)
	}

	if curNode.Spec.PodCIDR == "" {
		klog.Errorf("node %q has no pod CIDR assigned, ensure IPAM is enabled on the Kubernetes control plane", curNode.Name)
		os.Exit(1)
	}

	err = installCNIConf(curNode.Spec.PodCIDR)
	if err != nil {
		klog.Errorf("failed to install CNI: %v", err)
		os.Exit(1)
	}

	server, err := openflow.NewServer()
	if err != nil {
		klog.Errorf("error starting open flow server: %v", err)
		os.Exit(1)
	}

	server.Serve()
}

// installCNIConf adds the CNI config file given the pod cidr of the node
func installCNIConf(podCIDR string) error {
	conf := fmt.Sprintf(`{
	"name": "kube-ovs-cni",
	"type": "kube-ovs-cni",
	"bridge": "kube-ovs0",
	"isGateway": true,
	"isDefaultGateway": true,
	"ipam": {
		"type": "host-local",
		"subnet": "%s",
	}
}`, podCIDR)

	return ioutil.WriteFile(cniConfigPath, []byte(conf), 0644)
}
