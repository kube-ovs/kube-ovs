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
	"bytes"
	"net"

	"github.com/Kmotiko/gofc/ofprotocol/ofp13"
	"github.com/kube-ovs/kube-ovs/controllers"
	"github.com/kube-ovs/kube-ovs/openflow/protocol"
)

// DataPath code in gofc doesnt' export the send/recv loops that are pretty important...
// might be worth while to do packet handling ourselves here and only leverage gofc for the binary encoding of OF messages.

// OFConn handles message processes coming from a specific connection
// There should be one instance of OFConn per connection from the switch
type OFConn struct {
	buffer      chan *bytes.Buffer
	sendBuffer  chan ofp13.OFMessage
	controllers []controllers.Controller
	conn        *net.TCPConn
	datapathID  uint64
}

func NewOFConn(conn *net.TCPConn, controllers []controllers.Controller) *OFConn {
	return &OFConn{
		sendBuffer:  make(chan ofp13.OFMessage, 10),
		conn:        conn,
		controllers: controllers,
	}
}

func (of *OFConn) ReadMessages() {
	// is this max size of OF message?
	buf := make([]byte, 1024*64)

	for {
		size, err := of.conn.Read(buf)
		if err != nil {
			// TODO: log error once logging library is decided.
			return
		}

		for i := 0; i < size; {
			msgLen := binary.BigEndian.Uint16(buf[i+2:])
			err := of.processPacket(buf[i : i+(int)(msgLen)])
			if err != nil {
				// TODO: log
				continue
			}

			i += (int)(msgLen)
		}
	}
}

func (of *OFConn) SendMessages() {
	for {
		msg := <-(of.sendBuffer)
		byteData := msg.Serialize()
		_, err := of.conn.Write(byteData)
		if err != nil {
			// TODO: log error once logging lib is decided
			return
		}
	}
}

// processPacket is a generic placeholder for how OF messages will be handled
func (of *OFConn) processPacket(buf []byte) {
	// not implemented yet
	msg := protocol.ParseMessage(buf)
	return of.DispatchToControllers(msg)
}

// DispatchToControllers sends the OFMessage to each controller
// passing any valid returned values to sendBuffer to be sent back out
// the connection
func (of *OFConn) DispatchToControllers(msg ofp13.OFMessage) {
	for _, controller := range of.controllers {
		err := controller.HandleMessage(msg)
		if err != nil {
			// TODO: log this
			// return early?
			continue
		}
	}
}
