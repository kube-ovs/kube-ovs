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
)

const (
	tableClassification         = 0
	tableL3Rewrites             = 10
	tableL3Forwarding           = 20
	tableL2Rewrites             = 30
	tableL2Forwarding           = 40
	tableNetworkPoliciesIngress = 50
	tableNetworkPoliciesEgress  = 60
	tableProxy                  = 70
	tableNAT                    = 80
	tableAudit                  = 90
)

func (c *controller) setupBaseFlows() error {
	baseFlows := []*ofp13.OfpFlowMod{
		baseFlows(tableClassification),
		baseFlows(tableL3Rewrites),
		baseFlows(tableL3Forwarding),
		baseFlows(tableL2Rewrites),
		baseFlows(tableL2Forwarding),
		baseFlows(tableNetworkPoliciesIngress),
		baseFlows(tableNetworkPoliciesEgress),
		baseFlows(tableProxy),
		baseFlows(tableNAT),
		baseFlows(tableAudit),
	}

	for _, flow := range baseFlows {
		c.connManager.Send(flow)
	}

	return nil
}

// baseFlows returns the FlowMod action to create the base set of flows
// added by default to each kube-ovs table
func baseFlows(tableID uint8) *ofp13.OfpFlowMod {
	instruction := ofp13.NewOfpInstructionActions(ofp13.OFPIT_APPLY_ACTIONS)
	instruction.Actions = append(instruction.Actions, ofp13.NewOfpActionOutput(ofp13.OFPP_NORMAL, 0))

	return ofp13.NewOfpFlowModAdd(0, 0, tableID, 0, 0, ofp13.NewOfpMatch(),
		[]ofp13.OfpInstruction{instruction})
}
