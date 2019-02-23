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

	"github.com/kube-ovs/kube-ovs/controllers/pods"

	"github.com/Kmotiko/gofc"
	"github.com/Kmotiko/gofc/ofprotocol/ofp13"
)

const (
	// TODO: make this configurable, though in most cases this shouldn't need to change
	listenPort = 6653
)

type Server struct {
	listener *net.TCPListener
}

func NewServer() (*Server, error) {
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", listenPort))
	if err != nil {
		return nil, err
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return nil, err
	}

	return &Server{listener: listener}, nil
}

func (s *Server) RegisterControllers() {
	podController := pods.NewPodController()
	// don't love this pattern, should change it
	// maybe something that follows a builder pattern where an interfaace can be
	// passed into a central controller?
	gofc.GetAppManager().RegistApplication(podController)
}

func (s *Server) Serve() {
	for {
		conn, err := s.listener.AcceptTCP()
		if err != nil {
			// TODO: log this error
			return
		}

		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn) {
	// TODO: pass in registered controllers
	ofconn := NewOFConn(conn)
	go ofconn.ReadMessages()
	go ofconn.SendMessages()

}
