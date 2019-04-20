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

package controllers

import (
	"github.com/Kmotiko/gofc/ofprotocol/ofp13"

	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
)

type connectionManager interface {
	Receive() ofp13.OFMessage
	Send(msg ofp13.OFMessage)
}

type controller struct {
	datapathID   uint64
	connManager  connectionManager
	vxlanHandler cache.ResourceEventHandler
}

func NewController(connManager connectionManager) *controller {
	return &controller{
		connManager: connManager,
	}
}

func (c *controller) Initialize() error {
	// send initial hello which is required to establish a proper connection
	// with an open flow switch.
	hello := ofp13.NewOfpHello()
	c.connManager.Send(hello)

	err := c.setupBaseFlows()
	if err != nil {
		return err
	}

	c.vxlanHandler = newVxLANHandler(c.connManager)

	klog.Info("OF_HELLO message sent to switch")
	return nil
}

func (c *controller) VxLANHandler() cache.ResourceEventHandler {
	return c.vxlanHandler
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

		default:
		}
	}
}
