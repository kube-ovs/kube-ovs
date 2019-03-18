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

package echo

import (
	"errors"
	"fmt"
	"net"

	"github.com/Kmotiko/gofc/ofprotocol/ofp13"
	"github.com/kube-ovs/kube-ovs/controllers"

	"k8s.io/klog"
)

type echoController struct {
	conn *net.TCPConn
}

var _ controllers.Controller = &echoController{}

func NewEchoController() controllers.Controller {
	return &echoController{}
}

func (e *echoController) Name() string {
	return "echo"
}

func (e *echoController) RegisterConnection(conn *net.TCPConn) {
	e.conn = conn
}

func (e *echoController) Initialize() error {
	if e.conn == nil {
		return errors.New("controller must have a registered connection to the switch")
	}

	// send initial echo request
	echoReq := ofp13.NewOfpEchoRequest()
	_, err := e.conn.Write(echoReq.Serialize())
	if err != nil {
		return fmt.Errorf("error sending initial echo request: %v", err)
	}

	klog.Info("initial echo request sent")
	return nil
}

func (e *echoController) HandleMessage(msg ofp13.OFMessage) error {
	switch msgType := msg.(type) {
	case *ofp13.OfpHeader:
		if msgType.Type == ofp13.OFPT_ECHO_REQUEST {
			echoReply := ofp13.NewOfpEchoReply()
			_, err := e.conn.Write(echoReply.Serialize())
			return err
		}

		if msgType.Type == ofp13.OFPT_ECHO_REPLY {
			klog.Info("received echo reply from switch")
			return nil
		}

	default:
	}

	return nil
}

func (e *echoController) OnAdd(obj interface{}) {
}

func (e *echoController) OnUpdate(oldObj, newObj interface{}) {
}

func (e *echoController) OnDelete(obj interface{}) {
}
