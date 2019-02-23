package openflow

import (
	"bytes"
	"net"

	"github.com/Kmotiko/gofc/ofprotocol/ofp13"
)

// DataPath code in gofc doesnt' export the send/recv loops that are pretty important...
// might be worth while to do packet handling ourselves here and only leverage gofc for the binary encoding of OF messages.

// OFConn handles message processes coming from a specific connection
// There should be one instance of OFConn per connection from the switch
type OFConn struct {
	buffer     chan *bytes.Buffer
	sendBuffer chan *ofp13.OFMessage
	conn       *net.TCPConn
	datapathID uint64
}

func NewOFConn(conn *net.TCPConn) *OFConn {
	return &OFConn{
		sendBuffer: make(chan *ofp13.OFMessage, 10),
		conn:       conn,
	}
}

func (of *OFConn) ReadMessages() {
	// is this max size of OF message?
	buf := make([]byte, 1024*64)

	for {
		size, err := of.conn.Read(buf)
		if err != nil {
			// TODO: log error once logging library is decided.
			return
		}

		for i := 0; i < size; {
			msgLen := binary.BigEndian.Uint16(buf[i+2:])
			of.processPacket(buf[i : i+(int)(msgLen)])
			i += (int)(msgLen)
		}
	}
}

func (of *OFConn) SendMessages() {
	for {
		msg := <-(of.sendBuffer)
		byteData := (*msg).Serialize()
		_, err := of.conn.Write(byteData)
		if err != nil {
			// TODO: log error once logging lib is decided
			return
		}
	}
}

// processPacket is a generic placeholder for how OF messages will be handled
func (of *OFConn) processPacket(buf []byte) {
	// not implemented yet
	return
}
