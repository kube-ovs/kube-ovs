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

package tables

import (
	"net"

	"github.com/Kmotiko/gofc/ofprotocol/ofp13"
	"github.com/kube-ovs/kube-ovs/controllers"
)

const (
	tableDefault         = 0
	tableClassification  = 10
	tableNetworkPolicies = 20
	tableProxy           = 30
	tableNAT             = 40
	tableAudit           = 50
)

// tableController implements the controllers.Controller interface
// tableController is responsible for creating the set of open flow tables
type tableController struct {
	conn *net.TCPConn
}

var _ controllers.Controller = &tableController{}

func NewTableController(conn *net.TCPConn) controllers.Controller {
	return &tableController{conn}
}

func (t *tableController) Name() string {
	return "table"
}

func (t *tableController) Initialize() error {
	tables := []*ofp13.OfpTableMod{
		ofp13.NewOfpTableMod(tableDefault, 0),
		ofp13.NewOfpTableMod(tableClassification, 0),
		ofp13.NewOfpTableMod(tableNetworkPolicies, 0),
		ofp13.NewOfpTableMod(tableProxy, 0),
		ofp13.NewOfpTableMod(tableNAT, 0),
		ofp13.NewOfpTableMod(tableAudit, 0),
	}

	for _, table := range tables {
		_, err := t.conn.Write(table.Serialize())
		if err != nil {
			return err
		}
	}

	return nil
}

func (t *tableController) HandleMessage(msg ofp13.OFMessage) error {
	return nil
}
