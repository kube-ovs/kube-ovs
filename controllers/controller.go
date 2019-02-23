package controllers

import (
	"net"
)

type Controller interface {
	HandleConn(conn *net.TCPConn)
	DialConn(addr string) (net.Conn, error)
}
