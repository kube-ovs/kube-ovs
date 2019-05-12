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

package arp

import (
	"errors"
	"net"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"k8s.io/klog"
)

func GenerateARPReply(data []byte, gatewayMAC, gatewayIP string) ([]byte, error) {
	p := gopacket.NewPacket(data, layers.LayerTypeEthernet, gopacket.Default)

	ethLayer := p.Layer(layers.LayerTypeEthernet)
	if ethLayer == nil {
		return nil, errors.New("couldn't get ethernet layer")
	}

	_ = ethLayer.(*layers.Ethernet)

	arpLayer := p.Layer(layers.LayerTypeARP)
	if arpLayer == nil {
		return nil, errors.New("couldn't get arp layer")
	}

	arpData := arpLayer.(*layers.ARP)

	destinationHWAddr := arpData.SourceHwAddress
	destinationIPAddr := arpData.SourceProtAddress

	sourceHWAddr, err := net.ParseMAC("aa:aa:aa:aa:aa:aa")
	if err != nil {
		return nil, err
	}

	// for ARP requests against the gateway IP, we should add the actual
	// mac address of the bridge
	// if string(arpData.DstProtAddress) == gatewayIP {
	// 	sourceHWAddr, err = net.ParseMAC(gatewayMAC)
	// 	if err != nil {
	//		return nil, err
	//	}
	// }

	sourceIPAddr := arpData.DstProtAddress

	eth := layers.Ethernet{
		SrcMAC:       sourceHWAddr,
		DstMAC:       destinationHWAddr,
		EthernetType: layers.EthernetTypeARP,
	}
	arp := layers.ARP{
		AddrType:          layers.LinkTypeEthernet,
		Protocol:          layers.EthernetTypeIPv4,
		HwAddressSize:     6,
		ProtAddressSize:   4,
		Operation:         layers.ARPReply,
		SourceHwAddress:   []byte(sourceHWAddr),
		SourceProtAddress: sourceIPAddr,
		DstHwAddress:      []byte(destinationHWAddr),
		DstProtAddress:    destinationIPAddr,
	}

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{}

	err = gopacket.SerializeLayers(buf, opts, &eth, &arp)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
