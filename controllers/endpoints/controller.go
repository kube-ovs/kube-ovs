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

package endpoints

import (
	"net"

	"github.com/Kmotiko/gofc/ofprotocol/ofp13"
	"github.com/kube-ovs/kube-ovs/controllers"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
)

type endpointsController struct {
	conn *net.TCPConn
}

func NewEndpointsController() controllers.Controller {
	return &endpointsController{}
}

func (e *endpointsController) Name() string {
	return "endpoints"
}

func (e *endpointsController) RegisterConnection(conn *net.TCPConn) {
	e.conn = conn
}

func (e *endpointsController) Initialize() error {
	return nil
}

func (e *endpointsController) HandleMessage(msg ofp13.OFMessage) error {
	return nil
}

func (e *endpointsController) OnAdd(obj interface{}) {
	// ignore this event if there are no open flow connections established
	if e.conn == nil {
		return
	}

	ep, ok := obj.(*corev1.Endpoints)
	if !ok {
		return
	}

	klog.Infof("received OnAdd event for endpoint %q", ep.Name)
}

func (e *endpointsController) OnUpdate(oldObj, newObj interface{}) {
	// ignore this event if there are no open flow connections established
	if e.conn == nil {
		return
	}

	ep, ok := newObj.(*corev1.Endpoints)
	if !ok {
		return
	}

	klog.Infof("received OnUpdate event for endpoint %q", ep.Name)
}

func (e *endpointsController) OnDelete(obj interface{}) {
	// ignore this event if there are no open flow connections established
	if e.conn == nil {
		return
	}

	ep, ok := obj.(*corev1.Endpoints)
	if !ok {
		return
	}

	klog.Infof("received OnDelete event for endpoint %q", ep.Name)
}
