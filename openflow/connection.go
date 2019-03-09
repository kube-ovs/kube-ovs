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
	"bufio"
	"fmt"
	"io"
	"net"

	"github.com/Kmotiko/gofc/ofprotocol/ofp13"
	"github.com/kube-ovs/kube-ovs/controllers"
	"github.com/kube-ovs/kube-ovs/controllers/echo"
	"github.com/kube-ovs/kube-ovs/controllers/flows"
	"github.com/kube-ovs/kube-ovs/controllers/hello"
	"github.com/kube-ovs/kube-ovs/openflow/protocol"

	"k8s.io/klog"
)

// OFConn handles message processes coming from a specific connection
// There should be one instance of OFConn per connection from the switch
type OFConn struct {
	conn        *net.TCPConn
	controllers []controllers.Controller
	datapathID  uint64
}

func NewOFConn(conn *net.TCPConn) *OFConn {
	return &OFConn{
		conn:        conn,
		controllers: DefaultControllers(conn),
	}
}

func DefaultControllers(conn *net.TCPConn) []controllers.Controller {
	helloController := hello.NewHelloController(conn)
	echoController := echo.NewEchoController(conn)
	flowsController := flows.NewFlowsController(conn)

	return []controllers.Controller{
		helloController,
		echoController,
		flowsController,
	}
}

func (of *OFConn) InitializeControllers() error {
	for _, controller := range of.controllers {
		err := controller.Initialize()
		if err != nil {
			return fmt.Errorf("error initializing controller %q, err: %v", controller.Name(), err)
		}
	}

	return nil
}

func (of *OFConn) ReadMessages() {
	defer of.conn.Close()
	klog.Info("reading messages from connection")

	for {
		reader := bufio.NewReader(of.conn)

		// peak into the first 8 bytes (the size of OF header messages)
		// the header message contains the length of the entire message
		// which we need later to move the reader forward
		header, err := reader.Peek(8)
		if err != nil {
			klog.Errorf("could not peek at message header: %v", err)
			continue
		}

		msgLen := protocol.MessageLength(header)
		buf := make([]byte, msgLen)

		_, err = reader.Read(buf)
		if err != nil {
			if opErr, ok := err.(*net.OpError); !ok || !opErr.Timeout() {
				if err != io.EOF {
					klog.Errorf("error reading connection: %s", err.Error())
				}
				return
			}

			continue
		}

		msg := protocol.ParseMessage(buf)
		klog.Infof("received message %+v", msg)

		// handle special case of switch feature request message to get datapath ID
		if features, ok := msg.(*ofp13.OfpSwitchFeatures); ok {
			of.datapathID = features.DatapathId
			klog.Infof("set datapath ID to %d", features.DatapathId)

			continue
		}

		err = of.DispatchToControllers(msg)
		if err != nil {
			klog.Errorf("error handling msg from controller: %v", err)
		}
	}
}

// DispatchToControllers sends the OFMessage to each controller
func (of *OFConn) DispatchToControllers(msg ofp13.OFMessage) error {
	for _, controller := range of.controllers {
		err := controller.HandleMessage(msg)
		if err != nil {
			klog.Errorf("error handling message from %q controller, err: %v", controller.Name(), err)
			continue
		}
	}

	return nil
}
