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

package pods

import (
	"fmt"
	"net"

	"github.com/Kmotiko/gofc/ofprotocol/ofp13"

	"k8s.io/klog"
)

var DEFAULT_PORT = 6653

type PodController struct {
	switchAddr string

	sendBuffer chan *ofp13.OFMessage
}

func NewPodController() *PodController {
	pc := &PodController{}
	pc.sendBuffer = make(chan *ofp13.OFMessage, 10)
	return pc
}

func (p *PodController) HandleConn(conn net.Conn) {
	hello := ofp13.NewOfpHello()
	_, err := conn.Write(hello.Serialize())
	if err != nil {
		fmt.Println(err)
		return
	}

	go p.sendLoop(conn)
	go p.recvLoop(conn)
}

func (p *PodController) DialConn() (net.Conn, error) {
	conn, err := net.Dial("tcp", p.switchAddr)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (p *PodController) sendLoop(conn *net.TCPConn) {
	for {
		msg := <-(p.sendBuffer)
		byteData := (*msg).Serialize()
		_, err := conn.Write(byteData)
		if err != nil {
			klog.Errorf("failed to write to connection: %q", err)
			return
		}
	}
}

func (p *PodController) recvLoop(conn *net.TCPConn) {
	buf := make([]byte, 1024*64)

	size, err := conn.Read(buf)
	if err != nil {
		klog.Errorf("error reading connection: %s", err)
		return
	}

	for i := 0; i < size; {
		msgLen := binary.BigEndian.Uint16(buf[i+2:])
		p.handleMsg(buf[i : i+(int)(msgLen)])
		i += (int)(msgLen)
	}

}

func (p *PodController) handleMsg(buf []byte) {
	msg := ofp13.Parse(buf[0:])

	if _, ok := msg.(*ofp13.OfpHello); ok {
		// handle hello
		featureReq := ofp13.NewOfpFeaturesRequest()
		(p.sendBuffer) <- &featureReq
	} else {
		switch msgi := msg.(type) {
		// if message is OfpHeader
		case *ofp13.OfpHeader:
			switch msgi.Type {
			// handle echo request
			case ofp13.OFPT_ECHO_REQUEST:
				p.HandleEchoRequest(msgi)
			default:
			}
		case *ofp13.OfpSwitchFeatures:
			obj.HandleSwitchFeatures(msgi)

		}
	}
}

func (p *PodController) HandleHello(msg *ofp13.OfpHello, dp *Datapath) {
	// send feature request
	featureReq := ofp13.NewOfpFeaturesRequest()
	Send(dp, featureReq)
	(p.sendBuffer) <- &featureReq
}

func (p *PodController) HandleSwitchFeatures(msg *ofp13.OfpSwitchFeatures, dp *Datapath) {
	fmt.Println("recv SwitchFeatures")
}

func (p *PodController) HandleEchoRequest(msg *ofp13.OfpHeader, dp *Datapath) {
	// send EchoReply
	echo := ofp13.NewOfpEchoReply()
	(p.sendBuffer) <- &echo
}
