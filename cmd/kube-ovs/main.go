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
	"net"
	"os"
	"os/exec"

	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/kube-ovs/kube-ovs/controllers"
	"github.com/kube-ovs/kube-ovs/controllers/echo"
	"github.com/kube-ovs/kube-ovs/controllers/flows"
	"github.com/kube-ovs/kube-ovs/controllers/hello"
	"github.com/kube-ovs/kube-ovs/openflow"
	"github.com/vishvananda/netlink"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
)

const (
	cniConfigPath           = "/etc/cni/net.d/10-kube-ovs.json"
	bridgeName              = "kube-ovs0"
	defaultControllerTarget = "tcp:127.0.0.1:6653"
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

	// Get the current node's name. We're going to assume this was passed
	// via an env var called NODE_NAME for now
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

	podCIDR := curNode.Spec.PodCIDR
	err = installCNIConf(podCIDR)
	if err != nil {
		klog.Errorf("failed to install CNI: %v", err)
		os.Exit(1)
	}

	err = setupBridgeIfNotExists()
	if err != nil {
		klog.Errorf("failed to setup OVS bridge: %v", err)
		os.Exit(1)
	}

	br, err := netlink.LinkByName(bridgeName)
	if err != nil {
		klog.Errorf("failed to get bridge %q, err: %v", bridgeName, err)
		os.Exit(1)
	}

	addr, err := netlinkAddrForCIDR(podCIDR)
	if err != nil {
		klog.Errorf("failed to get netlink addr for CIDR %q, err: %v", podCIDR, err)
		os.Exit(1)
	}

	if err := netlink.AddrReplace(br, addr); err != nil {
		klog.Errorf("could not add addr %q to bridge %q, err: %v",
			podCIDR, bridgeName, err)
		os.Exit(1)
	}

	if err := setControllerTarget(); err != nil {
		klog.Errorf("failed to setup controller: %v", err)
		os.Exit(1)
	}

	server, err := openflow.NewServer()
	if err != nil {
		klog.Errorf("error starting open flow server: %v", err)
		os.Exit(1)
	}

	openFlowControllers := []controllers.Controller{
		hello.NewHelloController(),
		echo.NewEchoController(),
		flows.NewFlowsController(),
	}

	server.RegisterControllers(openFlowControllers...)
	server.Serve()
}

func netlinkAddrForCIDR(podCIDR string) (*netlink.Addr, error) {
	_, ipn, err := net.ParseCIDR(podCIDR)
	if err != nil {
		return nil, err
	}

	nid := ipn.IP.Mask(ipn.Mask)
	gw := ip.NextIP(nid)

	return &netlink.Addr{
		IPNet: &net.IPNet{
			IP:   gw,
			Mask: ipn.Mask,
		},
		Label: "",
	}, nil
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
		"subnet": "%s"
	}
}`, podCIDR)

	return ioutil.WriteFile(cniConfigPath, []byte(conf), 0644)
}

func setupBridgeIfNotExists() error {
	command := []string{
		"--may-exist", "add-br", bridgeName,
	}

	out, err := exec.Command("ovs-vsctl", command...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to setup OVS bridge %q, err: %v, output: %q",
			bridgeName, err, string(out))
	}

	br, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return fmt.Errorf("could not lookup %q: %v", bridgeName, err)
	}

	if err := netlink.LinkSetUp(br); err != nil {
		return fmt.Errorf("failed to bring bridge %q up: %v", bridgeName, err)
	}

	return nil
}

func setControllerTarget() error {
	command := []string{
		"set-controller", bridgeName, defaultControllerTarget,
	}

	out, err := exec.Command("ovs-vsctl", command...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set controller target for bridge %q, err: %v, out: %q",
			bridgeName, err, string(out))
	}

	return nil
}
