package openflow

import (
	"net"

	"github.com/Kmotiko/gofc"
)

const (
	defaultBridgeTCPAddr string = "127.0.0.1:6653"
)

func DialBridge() (net.Conn, error) {
	conn, err := net.Dial("tcp", defaultBridgeTCPAddr)
	if err != nil {
		return nil, err
	}

	return c, nil
}
