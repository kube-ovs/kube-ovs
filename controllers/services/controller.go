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

package services

import (
	"net"

	"github.com/Kmotiko/gofc/ofprotocol/ofp13"
	"github.com/kube-ovs/kube-ovs/controllers"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
)

type servicesController struct {
	conn *net.TCPConn
}

func NewServicesController() controllers.Controller {
	return &servicesController{}
}

func (s *servicesController) Name() string {
	return "services"
}

func (s *servicesController) RegisterConnection(conn *net.TCPConn) {
	s.conn = conn
}

func (s *servicesController) Initialize() error {
	return nil
}

func (s *servicesController) HandleMessage(msg ofp13.OFMessage) error {
	return nil
}

func (s *servicesController) OnAdd(obj interface{}) {
	// ignore this event if there are no open flow connections established
	if s.conn == nil {
		return
	}

	svc, ok := obj.(*corev1.Service)
	if !ok {
		return
	}

	klog.Infof("received OnAdd event for service %q", svc.Name)
}

func (s *servicesController) OnUpdate(oldObj, newObj interface{}) {
	// ignore this event if there are no open flow connections established
	if s.conn == nil {
		return
	}

	svc, ok := newObj.(*corev1.Service)
	if !ok {
		return
	}

	klog.Infof("received OnUpdate event for service %q", svc.Name)
}

func (s *servicesController) OnDelete(obj interface{}) {
	// ignore this event if there are no open flow connections established
	if s.conn == nil {
		return
	}

	svc, ok := obj.(*corev1.Service)
	if !ok {
		return
	}

	klog.Infof("received OnDelete event for service %q", svc.Name)
}
