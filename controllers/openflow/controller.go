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
	"errors"
	"net"
	"sync"

	"github.com/Kmotiko/gofc/ofprotocol/ofp13"
	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/kube-ovs/kube-ovs/arp"

	v1informer "k8s.io/client-go/informers/core/v1"
	v1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog"
)

type connectionManager interface {
	Receive() ofp13.OFMessage
	Send(msg ofp13.OFMessage)
}

type controller struct {
	datapathID  uint64
	connManager connectionManager

	nodeName    string
	gatewayIP   string
	gatewayMAC  string
	podCIDR     string
	clusterCIDR string

	nodeLister v1lister.NodeLister
	podLister  v1lister.PodLister

	podNetSpecMap podNetSpecMap
	netSpecLock   sync.Mutex
}

func NewController(connManager connectionManager,
	nodeInformer v1informer.NodeInformer,
	podInformer v1informer.PodInformer,
	gatewayMAC, nodeName, podCIDR, clusterCIDR string) *controller {
	// TODO: handle err
	_, podIPNet, _ := net.ParseCIDR(podCIDR)
	gatewayIP := ip.NextIP(podIPNet.IP.Mask(podIPNet.Mask)).String()

	return &controller{
		connManager:   connManager,
		nodeName:      nodeName,
		gatewayIP:     gatewayIP,
		gatewayMAC:    gatewayMAC,
		podCIDR:       podCIDR,
		clusterCIDR:   clusterCIDR,
		nodeLister:    nodeInformer.Lister(),
		podLister:     podInformer.Lister(),
		podNetSpecMap: make(podNetSpecMap),
	}
}

func (c *controller) Initialize() error {
	// send initial hello which is required to establish a proper connection
	// with an open flow switch.
	hello := ofp13.NewOfpHello()
	c.connManager.Send(hello)

	klog.Info("OF_HELLO message sent to switch")
	return nil
}

func (c *controller) Run() {
	for {
		msg := c.connManager.Receive()

		switch msgVal := msg.(type) {
		case *ofp13.OfpHeader:
			if msgVal.Type == ofp13.OFPT_ECHO_REQUEST {
				echoReply := ofp13.NewOfpEchoReply()
				klog.V(5).Info("echo reply sent to switch")
				c.connManager.Send(echoReply)
			}

			if msgVal.Type == ofp13.OFPT_ECHO_REPLY {
				klog.V(5).Info("received echo reply from switch")
			}

		case *ofp13.OfpHello:
			// hello received, next thing to do is send a feature request message
			// to receive the data path ID of the switch
			featureReq := ofp13.NewOfpFeaturesRequest()
			c.connManager.Send(featureReq)

		case *ofp13.OfpSwitchFeatures:
			c.datapathID = msgVal.DatapathId
			klog.Infof("set datapath ID to %d", c.datapathID)

		case *ofp13.OfpPacketIn:
			packetIn := msgVal

			inPort, err := inPortForPacket(packetIn)
			if err != nil {
				klog.Errorf("error fetching in port: %v", err)
				continue
			}

			arpReply, err := arp.GenerateARPReply(packetIn.Data, c.gatewayMAC, c.gatewayIP)
			if err != nil {
				klog.Errorf("error parsing ARP packet: %v", err)
				continue
			}
			klog.Infof("arp reply going to in-port %d: %#x", inPort, arpReply)

			packetOut := ofp13.NewOfpPacketOut(
				packetIn.BufferId,
				ofp13.OFPP_CONTROLLER,
				[]ofp13.OfpAction{ofp13.NewOfpActionOutput(inPort, 0)},
				arpReply,
			)
			c.connManager.Send(packetOut)

		default:
		}
	}
}

func inPortForPacket(packetIn *ofp13.OfpPacketIn) (uint32, error) {
	fields := packetIn.Match.OxmFields
	for _, field := range fields {
		if field.OxmField() == ofp13.OFPXMT_OFB_IN_PORT {
			inPort, ok := field.(*ofp13.OxmInPort)
			if !ok {
				return 0, errors.New("could not fetch OxmInPort")
			}
			return inPort.Value, nil
		}
	}

	return 0, errors.New("could not find in port match for packet")
}
