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
	"github.com/Kmotiko/gofc/ofprotocol/ofp13"
	"github.com/kube-ovs/kube-ovs/controllers"
)

type echoController struct {
	sendBuffer chan ofp13.OFMessage
}

func NewEchoController(sendBuffer chan ofp13.OFMessage) controllers.Controller {
	return &echoController{
		sendBuffer: sendBuffer,
	}
}

func (e *echoController) HandleMessage(msg ofp13.OFMessage) error {
	switch msgType := msg.(type) {
	case *ofp13.OfpHeader:
		if msgType.Type == ofp13.OFPT_ECHO_REQUEST {
			echoReply := ofp13.NewOfpEchoReply()
			e.sendBuffer <- echoReply
			return nil
		}

		if msgType.Type == ofp13.OFPT_ECHO_REPLY {
			// TODO: log the echo request if requested
			return nil
		}

	default:
	}

	return nil
}
