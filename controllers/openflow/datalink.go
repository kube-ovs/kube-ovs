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

package openflow

import (
	"encoding/json"
	"fmt"
	"net"
	"os/exec"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog"

	"github.com/Kmotiko/gofc/ofprotocol/ofp13"
	arpcache "github.com/mostlygeek/arp"
)

func (c *controller) OnAddEndpoints(obj interface{}) {
	ep, ok := obj.(*corev1.Endpoints)
	if !ok {
		return
	}

	if err := c.syncEndpoint(ep); err != nil {
		klog.Errorf("error syncing endpoint: %v", err)
		return
	}
}

func (c *controller) OnUpdateEndpoints(oldObj, newObj interface{}) {
	ep, ok := newObj.(*corev1.Endpoints)
	if !ok {
		return
	}

	if err := c.syncEndpoint(ep); err != nil {
		klog.Errorf("error syncing endpoint: %v", err)
		return
	}
}

func (c *controller) OnDeleteEndpoints(obj interface{}) {
	_, ok := obj.(*corev1.Endpoints)
	if !ok {
		return
	}
}

func (c *controller) isLocalIP(ip string) (bool, error) {
	_, ipnet, err := net.ParseCIDR(c.podCIDR)
	if err != nil {
		return false, err
	}

	return ipnet.Contains(net.ParseIP(ip)), nil
}

func (c *controller) syncEndpoint(ep *corev1.Endpoints) error {
	flows := []*ofp13.OfpFlowMod{}
	arpcache.CacheUpdate()

	for _, subset := range ep.Subsets {
		for _, address := range subset.Addresses {
			local, err := c.isLocalIP(address.IP)
			if err != nil {
				return fmt.Errorf("error checking if IP %q is local: %v", err)
			}

			if !local {
				continue
			}

			flow, err := c.dataLinkFlowsForLocalIP(address.IP)
			if err != nil {
				return fmt.Errorf("error getting data link flows for IP %q", address.IP)
			}
			flows = append(flows, flow)
		}
	}

	for _, flow := range flows {
		c.connManager.Send(flow)
	}

	return nil
}

func (c *controller) dataLinkFlowsForLocalIP(ip string) (*ofp13.OfpFlowMod, error) {
	macaddr := arpcache.Search(ip)
	if macaddr == "" {
		// TODO: check that address.IP is actually in local pod CIDR
		hwaddr, err := c.arp.Resolve(net.ParseIP(ip))
		if err != nil {
			return nil, fmt.Errorf("error resolving mac addr for IP %q, err: %v", ip, err)
		}

		macaddr = hwaddr.String()
	}

	ipv4Match, err := ofp13.NewOxmIpv4Dst(ip)
	if err != nil {
		return nil, fmt.Errorf("error getting IPv4Dst match: %v", err)
	}

	pod, err := c.getPodWithIP(ip)
	if err != nil {
		return nil, fmt.Errorf("error getting pod with IP %q: %v", ip, err)
	}

	portName, err := findPort(pod.Namespace, pod.Name)
	if err != nil {
		return nil, fmt.Errorf("error finding port for pod %q, err: %v", pod.Name, err)
	}

	ofport, err := ofPortFromName(portName)
	if err != nil {
		return nil, fmt.Errorf("error getting ofport for port %q, err: %v", portName, err)
	}

	// add flow for this endpoint
	match := ofp13.NewOfpMatch()
	match.Append(ofp13.NewOxmEthType(0x0800))
	match.Append(ipv4Match)

	instruction := ofp13.NewOfpInstructionActions(ofp13.OFPIT_APPLY_ACTIONS)
	ethDst, err := ofp13.NewOxmEthDst(macaddr)
	if err != nil {
		return nil, err
	}

	instruction.Append(ofp13.NewOfpActionSetField(ethDst))
	instruction.Append(ofp13.NewOfpActionOutput(ofport, 0))

	return ofp13.NewOfpFlowModAdd(0, 0, tableL2Rewrites, 100, 0, match,
		[]ofp13.OfpInstruction{instruction}), nil
}

func (c *controller) addDataLinkFlowForGateway(ip string, bridge *net.Interface) (*ofp13.OfpFlowMod, error) {
	ipv4Match, err := ofp13.NewOxmIpv4Dst(ip)
	if err != nil {
		return nil, fmt.Errorf("error getting IPv4Dst match: %v", err)
	}

	// add flow for this endpoint
	match := ofp13.NewOfpMatch()
	match.Append(ofp13.NewOxmEthType(0x0800))
	match.Append(ipv4Match)

	instruction := ofp13.NewOfpInstructionActions(ofp13.OFPIT_APPLY_ACTIONS)
	ethDst, err := ofp13.NewOxmEthDst(bridge.HardwareAddr.String())
	if err != nil {
		return nil, err
	}

	instruction.Append(ofp13.NewOfpActionSetField(ethDst))
	instruction.Append(ofp13.NewOfpActionOutput(ofp13.OFPP_NORMAL, 0))

	return ofp13.NewOfpFlowModAdd(0, 0, tableL2Rewrites, 100, 0, match,
		[]ofp13.OfpInstruction{instruction}), nil

}

func (c *controller) getPodWithIP(podIP string) (*corev1.Pod, error) {
	pods, err := c.podLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	for _, pod := range pods {
		if pod.Status.PodIP == podIP {
			return pod, nil
		}
	}

	return nil, fmt.Errorf("couldn't find pod with IP %q", podIP)
}

func findPort(podNamespace, podName string) (string, error) {
	commands := []string{
		"--format=json", "--column=name", "find", "port",
		fmt.Sprintf("external-ids:k8s_pod_namespace=%s", podNamespace),
		fmt.Sprintf("external-ids:k8s_pod_name=%s", podName),
	}

	out, err := exec.Command("ovs-vsctl", commands...).Output()
	if err != nil {
		return "", fmt.Errorf("failed to get OVS port for %s/%s, err: %v",
			podNamespace, podName, err)
	}

	dbData := struct {
		Data [][]string
	}{}
	if err = json.Unmarshal(out, &dbData); err != nil {
		return "", err
	}

	if len(dbData.Data) == 0 {
		// TODO: might make more sense to not return an error here since
		// CNI delete can be called multiple times.
		return "", fmt.Errorf("OVS port for %s/%s was not found, OVS DB data: %v, output: %q",
			podNamespace, podName, dbData.Data, string(out))
	}

	portName := dbData.Data[0][0]
	return portName, nil
}