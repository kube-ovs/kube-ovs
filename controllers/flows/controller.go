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

package flows

import (
	"net"

	"github.com/Kmotiko/gofc/ofprotocol/ofp13"
	"github.com/kube-ovs/kube-ovs/controllers"
)

const (
	tableClassification  = 0
	tableL3Rewrites      = 10
	tableL3Forwarding    = 20
	tableL2Rewrites      = 30
	tableL2Forwarding    = 40
	tableNetworkPolicies = 50
	tableProxy           = 60
	tableNAT             = 70
	tableAudit           = 80
)

// flowsController implements the controllers.Controller interface
// flowsController is responsible for creating flows on the switch
type flowsController struct {
	conn *net.TCPConn
}

var _ controllers.Controller = &flowsController{}

func NewFlowsController(conn *net.TCPConn) controllers.Controller {
	return &flowsController{conn}
}

func (t *flowsController) Name() string {
	return "table"
}

func (t *flowsController) Initialize() error {
	// init all tables by adding output port to NORMAL at the lowest priority
	baseFlows := []*ofp13.OfpFlowMod{
		baseFlows(tableClassification),
		baseFlows(tableL3Rewrites),
		baseFlows(tableL3Forwarding),
		baseFlows(tableL2Rewrites),
		baseFlows(tableL2Forwarding),
		baseFlows(tableNetworkPolicies),
		baseFlows(tableProxy),
		baseFlows(tableNAT),
		baseFlows(tableAudit),
	}

	for _, flow := range baseFlows {
		_, err := t.conn.Write(flow.Serialize())
		if err != nil {
			return err
		}
	}

	return nil
}

func (t *flowsController) HandleMessage(msg ofp13.OFMessage) error {
	return nil
}

func baseFlows(tableID uint8) *ofp13.OfpFlowMod {
	instruction := ofp13.NewOfpInstructionActions(ofp13.OFPIT_APPLY_ACTIONS)
	instruction.Actions = append(instruction.Actions, ofp13.NewOfpActionOutput(ofp13.OFPP_NORMAL, 0))

	return ofp13.NewOfpFlowModAdd(0, 0, tableID, 0, 0, ofp13.NewOfpMatch(),
		[]ofp13.OfpInstruction{instruction})
}
