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
	"net"

	"github.com/Kmotiko/gofc/ofprotocol/ofp13"
	"github.com/kube-ovs/kube-ovs/controllers"
	"github.com/kube-ovs/kube-ovs/controllers/echo"
	"github.com/kube-ovs/kube-ovs/controllers/hello"
	"github.com/kube-ovs/kube-ovs/openflow/protocol"

	"k8s.io/klog"
)

// OFConn handles message processes coming from a specific connection
// There should be one instance of OFConn per connection from the switch
type OFConn struct {
	conn        *net.TCPConn
	controllers []controllers.Controller
	// TODO: set this field
	datapathID uint64
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

	return []controllers.Controller{
		helloController,
		echoController,
	}
}

func (of *OFConn) ReadMessages() {
	var buf []byte

	for {
		size, err := of.conn.Read(buf)
		if err != nil {
			// TODO: log error once logging library is decided.
			klog.Errorf("error reading from connection: %v", err)
			return
		}

		for i := 0; i < size; {
			msgLen := protocol.MessageLength(buf)
			msg := protocol.ParseMessage(buf[i : i+msgLen])

			err := of.DispatchToControllers(msg)
			if err != nil {
				// TODO: log
				continue
			}

			i += msgLen
		}
	}
}

// DispatchToControllers sends the OFMessage to each controller
func (of *OFConn) DispatchToControllers(msg ofp13.OFMessage) error {
	for _, controller := range of.controllers {
		err := controller.HandleMessage(msg)
		if err != nil {
			// TODO: log this
			// return early?
			continue
		}
	}

	return nil
}
