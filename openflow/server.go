package openflow

import (
	"net"

	"github.com/kube-ovs/kube-ovs/controllers/pods"

	"github.com/Kmotiko/gofc"
	"github.com/Kmotiko/gofc/ofprotocol/ofp13"
)

const (
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

	s := &Server{listener: listener}
	return s, nil
}

func (s *Server) RegisterControllers() {
	podController := pods.NewPodController()
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
	// send hello
	hello := ofp13.NewOfpHello()
	_, err := conn.Write(hello.Serialize())
	if err != nil {
		fmt.Println(err)
		return
	}

	dp := gofc.NewDatapath(conn)
	go dp.recvLoop()
	go dp.sendLoop()
}
