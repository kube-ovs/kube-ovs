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
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/coreos/go-iptables/iptables"
	kovs "github.com/kube-ovs/kube-ovs/apis/generated/clientset/versioned"
	kovsinformer "github.com/kube-ovs/kube-ovs/apis/generated/informers/externalversions"
	"github.com/kube-ovs/kube-ovs/connection"
	"github.com/kube-ovs/kube-ovs/controllers/openflow"
	"github.com/kube-ovs/kube-ovs/controllers/ports"
	"github.com/mdlayher/arp"
	"github.com/vishvananda/netlink"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
)

const (
	cniConfigPath           = "/etc/cni/net.d/10-kube-ovs.json"
	bridgeName              = "kube-ovs0"
	defaultControllerTarget = "tcp:127.0.0.1:6653"

	// TODO: set this via a CLI flag
	defaultClusterCIDR = "100.96.0.0/11"
	// TODO: set this via a CLI flag
	defaultServiceCIDR = "100.64.0.0/13"
)

func main() {
	klog.InitFlags(flag.CommandLine)
	klog.Info("starting kube-ovs")

	restConfig, err := rest.InClusterConfig()
	if err != nil {
		klog.Errorf("error create in cluster config: %v", err)
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

	addr, err := netlinkAddrForCIDR(defaultClusterCIDR, podCIDR)
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

	if err := setSecureFailMode(); err != nil {
		klog.Errorf("failed to set fail-mode to 'secure': %v", err)
		os.Exit(1)
	}

	if err := setupModulesAndSysctls(); err != nil {
		klog.Errorf("failed to setup sysctls: %v", err)
		os.Exit(1)
	}

	if err := setupBridgeForwarding(podCIDR); err != nil {
		klog.Errorf("failed to setup bridge forwarding: %v", err)
		os.Exit(1)
	}

	bridge, err := net.InterfaceByName(bridgeName)
	if err != nil {
		klog.Errorf("failed to get bridge interface: %v", err)
		os.Exit(1)
	}

	arpClient, err := arp.Dial(bridge)
	if err != nil {
		klog.Errorf("failed to create arp client: %v")
		os.Exit(1)
	}

	// TODO: poll until VSwitchConfig exists for node since it won't exist
	// for new clusters until kube-ovs-controller is running
	vswitchConfig, err := kovsClientset.KubeovsV1alpha1().VSwitchConfigs().Get(curNode.Name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("error getting vswitch config for node: %v", err)
		os.Exit(1)
	}

	stopCh := make(chan struct{})

	term := make(chan os.Signal, 1)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-term
		close(stopCh)
	}()

	// TODO: adjust resync period when queue is implemented
	informerFactory := informers.NewSharedInformerFactory(clientset, time.Minute)
	nodeInformer := informerFactory.Core().V1().Nodes()
	podInformer := informerFactory.Core().V1().Pods()
	endpointsInformer := informerFactory.Core().V1().Endpoints()

	// TODO: adjust resync period when queue is implemented
	kovsInformerFactory := kovsinformer.NewSharedInformerFactory(kovsClientset, time.Minute)
	vswitchInformer := kovsInformerFactory.Kubeovs().V1alpha1().VSwitchConfigs()

	connectionManager, err := connection.NewOFConnect()
	if err != nil {
		klog.Errorf("error starting open flow connection manager: %v", err)
		os.Exit(1)
	}

	c := openflow.NewController(connectionManager, nodeInformer, podInformer, arpClient, curNode.Name, podCIDR, defaultClusterCIDR)

	vswitchInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.OnAddVSwitch,
		UpdateFunc: c.OnUpdateVSwitch,
		DeleteFunc: c.OnDeleteVSwitch,
	})
	endpointsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.OnAddEndpoints,
		UpdateFunc: c.OnUpdateEndpoints,
		DeleteFunc: c.OnDeleteEndpoints,
	})

	err = c.Initialize()
	if err != nil {
		klog.Errorf("error initializing openflow controller: %v", err)
		os.Exit(1)
	}

	klog.Info("setting up default flows")
	err = c.AddDefaultFlows(bridgeName)
	if err != nil {
		klog.Errorf("error adding default flows: %v", err)
		os.Exit(1)
	}

	vxlanPorts := ports.NewVxlanPorts(bridgeName, vswitchConfig, vswitchInformer)
	vswitchInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    vxlanPorts.OnAddVSwitch,
		UpdateFunc: vxlanPorts.OnUpdateVSwitch,
		DeleteFunc: vxlanPorts.OnDeleteVSwitch,
	})

	informerFactory.WaitForCacheSync(stopCh)
	kovsInformerFactory.WaitForCacheSync(stopCh)

	informerFactory.Start(stopCh)
	kovsInformerFactory.Start(stopCh)

	go connectionManager.ProcessQueue()
	go connectionManager.Serve()
	go c.Run()

	<-stopCh
}

func netlinkAddrForCIDR(clusterCIDR, podCIDR string) (*netlink.Addr, error) {
	_, podIPNet, err := net.ParseCIDR(podCIDR)
	if err != nil {
		return nil, err
	}

	gw := ip.NextIP(podIPNet.IP.Mask(podIPNet.Mask))

	_, clusterIPNet, err := net.ParseCIDR(clusterCIDR)
	if err != nil {
		return nil, err
	}

	return &netlink.Addr{
		IPNet: &net.IPNet{
			IP:   gw,
			Mask: clusterIPNet.Mask,
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

func setSecureFailMode() error {
	command := []string{
		"set-fail-mode", bridgeName, "secure",
	}

	out, err := exec.Command("ovs-vsctl", command...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set fail mode for bridge %q to 'secure', err: %v, out: %q",
			bridgeName, err, string(out))
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

func setupBridgeForwarding(podCIDR string) error {
	ipt, err := iptables.NewWithProtocol(iptables.ProtocolIPv4)
	if err != nil {
		return err
	}

	rules := []string{"-o", bridgeName, "-j", "ACCEPT"}
	err = ipt.AppendUnique("filter", "FORWARD", rules...)
	if err != nil {
		return err
	}

	rules = []string{"-i", bridgeName, "-j", "ACCEPT"}
	err = ipt.AppendUnique("filter", "FORWARD", rules...)
	if err != nil {
		return err
	}

	rules = []string{"-s", podCIDR, "!", "-o", bridgeName, "-j", "MASQUERADE"}
	err = ipt.AppendUnique("nat", "POSTROUTING", rules...)
	if err != nil {
		return err
	}

	rules = []string{"!", "-d", defaultClusterCIDR, "-m", "comment", "--comment", "kube-ovs: SNAT for outbound traffic from cluster CIDR", "-m", "addrtype", "!", "--dst-type", "LOCAL", "-j", "MASQUERADE"}
	err = ipt.AppendUnique("nat", "POSTROUTING", rules...)
	if err != nil {
		return err
	}

	rules = []string{"!", "-d", defaultServiceCIDR, "-m", "comment", "--comment", "kube-ovs: SNAT for outbound traffic from service CIDR", "-m", "addrtype", "!", "--dst-type", "LOCAL", "-j", "MASQUERADE"}
	err = ipt.AppendUnique("nat", "POSTROUTING", rules...)
	if err != nil {
		return err
	}

	return nil
}

func setupModulesAndSysctls() error {
	if out, err := exec.Command("modprobe", "br_netfilter").CombinedOutput(); err != nil {
		return fmt.Errorf("failed to enable br_netfilter module, err: %v, out: %q", err, string(out))
	}

	if err := ioutil.WriteFile("/proc/sys/net/bridge/bridge-nf-call-iptables", []byte(strconv.Itoa(1)), 0640); err != nil {
		return fmt.Errorf("failed to set /proc/sys/net/bridge/bridge-nf-call-iptables, err: %v", err)
	}

	return nil
}
