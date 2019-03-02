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

package hello

import (
	"fmt"
	"net"

	"github.com/Kmotiko/gofc/ofprotocol/ofp13"
	"github.com/kube-ovs/kube-ovs/controllers"

	"k8s.io/klog"
)

// helloController implements the Controller interface
// helloController immiediately sends an OF_HELLO message to the switch
type helloController struct {
	conn *net.TCPConn
}

var _ controllers.Controller = &helloController{}

func NewHelloController(conn *net.TCPConn) controllers.Controller {
	return &helloController{conn}
}

func (h *helloController) Name() string {
	return "hello"
}

func (h *helloController) Initialize() error {
	// send initial hello which is required to establish a propoer connection
	// with an open flow switch.
	hello := ofp13.NewOfpHello()
	_, err := h.conn.Write(hello.Serialize())
	if err != nil {
		return fmt.Errorf("error writing to connection: %v", err)
	}

	klog.Info("OF_HELLO message sent to switch")
	return nil
}

func (h *helloController) HandleMessage(msg ofp13.OFMessage) error {
	if _, ok := msg.(*ofp13.OfpHello); !ok {
		return nil
	}

	// hello received, next thing to do is send a feature request message
	// to receive the data path ID of the switch
	featureReq := ofp13.NewOfpFeaturesRequest()
	_, err := h.conn.Write(featureReq.Serialize())
	return err
}
