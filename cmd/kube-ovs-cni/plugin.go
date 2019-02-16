package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"runtime"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ipam"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containernetworking/plugins/pkg/utils"
	"github.com/digitalocean/go-openvswitch/ovs"
	"github.com/j-keck/arping"
	"github.com/vishvananda/netlink"
)

const defaultBridgeName = "kube-ovs0"

type NetConf struct {
	types.NetConf

	BridgeName string `json:"bridge"`

	// copied from bridge plugin, clean up later
	IsGW         bool `json:"isGateway`
	IsDefaultGW  bool `json:"isDefaultGateway"`
	ForceAddress bool `json:"forceAddress"`
	IPMasq       bool `json:"ipMasq"`
	MTU          int  `json:"mtu"`
	HairpinMode  bool `json:"hairpinMode"`
	PromiscMode  bool `json:"promiscMode"`
}

type gwInfo struct {
	gws               []net.IPNet
	family            int
	defaultRouteFound bool
}

func init() {
	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}

func loadNetConf(bytes []byte) (*NetConf, string, error) {
	n := &NetConf{
		BridgeName: defaultBridgeName,
	}
	if err := json.Unmarshal(bytes, n); err != nil {
		return nil, "", fmt.Errorf("failed to load netconf: %v", err)
	}
	return n, n.CNIVersion, nil
}

func bridgeByName(name string) (netlink.Link, error) {
	br, err := netlink.LinkByName(name)
	if err != nil {
		return nil, fmt.Errorf("could not lookup %q: %v", name, err)
	}

	return br, nil
}

func setupBridgeIfNotExists(n *NetConf, ovs *ovs.Client) (*current.Interface, error) {
	// create bridge if necessary
	err := ovs.VSwitch.AddBridge(n.BridgeName)
	if err != nil {
		return nil, fmt.Errorf("failed to create OVS bridge %q: %v", n.BridgeName, err)
	}

	br, err := bridgeByName(n.BridgeName)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch bridge %q: %v", n.BridgeName, err)
	}

	return &current.Interface{
		Name: br.Attrs().Name,
		Mac:  br.Attrs().HardwareAddr.String(),
	}, nil
}

func setupVeth(netns ns.NetNS, ifName string) (*current.Interface, *current.Interface, error) {
	contIface := &current.Interface{}
	hostIface := &current.Interface{}

	err := netns.Do(func(hostNS ns.NetNS) error {
		// create the veth pair in the container and move host end into host netns
		// TODO don't hardcode the MTU
		hostVeth, containerVeth, err := ip.SetupVeth(ifName, 1500, hostNS)
		if err != nil {
			return err
		}
		contIface.Name = containerVeth.Name
		contIface.Mac = containerVeth.HardwareAddr.String()
		contIface.Sandbox = netns.Path()
		hostIface.Name = hostVeth.Name
		// hostIface.Sandbox = hostNS.Path() // this doesn't exist in other plugins
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	// need to lookup hostVeth again as its index has changed during ns move
	hostVeth, err := netlink.LinkByName(hostIface.Name)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to lookup %q: %v", hostIface.Name, err)
	}
	hostIface.Mac = hostVeth.Attrs().HardwareAddr.String()

	// connect host veth end to the bridge
	// TODO: should this actually be done using OVS to `ovs-vsctl add-port br0 hostVeth`?
	// if err := netlink.LinkSetMaster(hostVeth, br); err != nil {
	//	return nil, nil, fmt.Errorf("failed to connect %q to bridge %v: %v", hostVeth.Attrs().Name, br.Attrs().Name, err)
	// }

	// set hairpin mode, do we need this?
	// if err = netlink.LinkSetHairpin(hostVeth, hairpinMode); err != nil {
	//	return nil, nil, fmt.Errorf("failed to setup hairpin mode for %v: %v", hostVeth.Attrs().Name, err)
	// }

	return hostIface, contIface, nil
}

// calcGateways processes the results from the IPAM plugin and does the
// following for each IP family:
//    - Calculates and compiles a list of gateway addresses
//    - Adds a default route if needed
func calcGateways(result *current.Result, n *NetConf) (*gwInfo, *gwInfo, error) {

	gwsV4 := &gwInfo{}
	gwsV6 := &gwInfo{}

	for _, ipc := range result.IPs {

		// Determine if this config is IPv4 or IPv6
		var gws *gwInfo
		defaultNet := &net.IPNet{}
		switch {
		case ipc.Address.IP.To4() != nil:
			gws = gwsV4
			gws.family = netlink.FAMILY_V4
			defaultNet.IP = net.IPv4zero
		case len(ipc.Address.IP) == net.IPv6len:
			gws = gwsV6
			gws.family = netlink.FAMILY_V6
			defaultNet.IP = net.IPv6zero
		default:
			return nil, nil, fmt.Errorf("Unknown IP object: %v", ipc)
		}
		defaultNet.Mask = net.IPMask(defaultNet.IP)

		// All IPs currently refer to the container interface
		ipc.Interface = current.Int(2)

		// If not provided, calculate the gateway address corresponding
		// to the selected IP address
		if ipc.Gateway == nil && n.IsGW {
			ipc.Gateway = calcGatewayIP(&ipc.Address)
		}

		// Add a default route for this family using the current
		// gateway address if necessary.
		if n.IsDefaultGW && !gws.defaultRouteFound {
			for _, route := range result.Routes {
				if route.GW != nil && defaultNet.String() == route.Dst.String() {
					gws.defaultRouteFound = true
					break
				}
			}
			if !gws.defaultRouteFound {
				result.Routes = append(
					result.Routes,
					&types.Route{Dst: *defaultNet, GW: ipc.Gateway},
				)
				gws.defaultRouteFound = true
			}
		}

		// Append this gateway address to the list of gateways
		if n.IsGW {
			gw := net.IPNet{
				IP:   ipc.Gateway,
				Mask: ipc.Address.Mask,
			}
			gws.gws = append(gws.gws, gw)
		}
	}
	return gwsV4, gwsV6, nil
}

func calcGatewayIP(ipn *net.IPNet) net.IP {
	nid := ipn.IP.Mask(ipn.Mask)
	return ip.NextIP(nid)
}

// disableIPV6DAD disables IPv6 Duplicate Address Detection (DAD)
// for an interface, if the interface does not support enhanced_dad.
// We do this because interfaces with hairpin mode will see their own DAD packets
func disableIPV6DAD(ifName string) error {
	// ehanced_dad sends a nonce with the DAD packets, so that we can safely
	// ignore ourselves
	enh, err := ioutil.ReadFile(fmt.Sprintf("/proc/sys/net/ipv6/conf/%s/enhanced_dad", ifName))
	if err == nil && string(enh) == "1\n" {
		return nil
	}
	f := fmt.Sprintf("/proc/sys/net/ipv6/conf/%s/accept_dad", ifName)
	return ioutil.WriteFile(f, []byte("0"), 0644)
}

func enableIPForward(family int) error {
	if family == netlink.FAMILY_V4 {
		return ip.EnableIP4Forward()
	}
	return ip.EnableIP6Forward()
}

func addToSwitch(ovsc *ovs.Client, hostIface string, conIface string, bridge string, contNetnsPath string) error {
	ovsc.VSwitch.AddPort(bridge, hostIface)
	return nil
}

func cmdAdd(args *skel.CmdArgs) error {
	success := false

	ovsClient := ovs.New()

	netConf, cniVersion, err := loadNetConf(args.StdinData)
	if err != nil {
		return err
	}

	// hashing function to get containerID -> OVS port?
	bridge, err := setupBridgeIfNotExists(netConf, ovsClient)
	if err != nil {
		return fmt.Errorf("failed to setup bridge: %v", err)
	}

	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", args.Netns, err)
	}
	defer netns.Close()

	hostInterface, containerInterface, err := setupVeth(netns, args.IfName)
	if err != nil {
		return err
	}

	result := &current.Result{CNIVersion: cniVersion, Interfaces: []*current.Interface{bridge, hostInterface, containerInterface}}

	if netConf.IPAM.Type != "" {
		// run the IPAM plugin and get back the config to apply
		r, err := ipam.ExecAdd(netConf.IPAM.Type, args.StdinData)
		if err != nil {
			return err
		}

		// release IP in case of failure
		defer func() {
			if !success {
				os.Setenv("CNI_COMMAND", "DEL")
				ipam.ExecDel(netConf.IPAM.Type, args.StdinData)
				os.Setenv("CNI_COMMAND", "ADD")
			}
		}()

		// Convert whatever the IPAM result was into the current Result type
		ipamResult, err := current.NewResultFromResult(r)
		if err != nil {
			return err
		}

		result.IPs = ipamResult.IPs
		result.Routes = ipamResult.Routes

		if len(result.IPs) == 0 {
			return errors.New("IPAM plugin returned missing IP config")
		}

		// Gather gateway information for each IP family
		gwsV4, gwsV6, err := calcGateways(result, netConf)
		if err != nil {
			return err
		}

		// Configure the container hardware address and IP address(es)
		if err := netns.Do(func(_ ns.NetNS) error {
			contVeth, err := net.InterfaceByName(args.IfName)
			if err != nil {
				return err
			}

			// Disable IPv6 DAD just in case hairpin mode is enabled on the
			// bridge. Hairpin mode causes echos of neighbor solicitation
			// packets, which causes DAD failures.
			for _, ipc := range result.IPs {
				if ipc.Version == "6" && (netConf.HairpinMode || netConf.PromiscMode) {
					if err := disableIPV6DAD(args.IfName); err != nil {
						return err
					}
					break
				}
			}

			// Add the IP to the interface
			if err := ipam.ConfigureIface(args.IfName, result); err != nil {
				return err
			}

			// Send a gratuitous arp
			for _, ipc := range result.IPs {
				if ipc.Version == "4" {
					_ = arping.GratuitousArpOverIface(ipc.Address.IP, *contVeth)
				}
			}
			return nil
		}); err != nil {
			return err
		}

		if netConf.IsGW {
			var firstV4Addr net.IP
			// Set the IP address(es) on the bridge and enable forwarding
			for _, gws := range []*gwInfo{gwsV4, gwsV6} {
				for _, gw := range gws.gws {
					if gw.IP.To4() != nil && firstV4Addr == nil {
						firstV4Addr = gw.IP
					}

					// TODO: do we need this still?
					// err = ensureBridgeAddr(br, gws.family, &gw, n.ForceAddress)
					// if err != nil {
					// 	return fmt.Errorf("failed to set bridge addr: %v", err)
					// }
				}

				if gws.gws != nil {
					if err = enableIPForward(gws.family); err != nil {
						return fmt.Errorf("failed to enable forwarding: %v", err)
					}
				}
			}
		}

		if netConf.IPMasq {
			chain := utils.FormatChainName(netConf.Name, args.ContainerID)
			comment := utils.FormatComment(netConf.Name, args.ContainerID)
			for _, ipc := range result.IPs {
				if err = ip.SetupIPMasq(ip.Network(&ipc.Address), chain, comment); err != nil {
					return err
				}
			}
		}
	}

	// Refetch the bridge since its MAC address may change when the first
	// veth is added or after its IP address is set
	br, err := bridgeByName(netConf.BridgeName)
	if err != nil {
		return err
	}
	bridge.Mac = br.Attrs().HardwareAddr.String()

	result.DNS = netConf.DNS

	// re: defer statement above for cleaning up IPAM
	success = true

	return types.PrintResult(result, cniVersion)
}

func cmdDel(args *skel.CmdArgs) error {
	return nil
}

func cmdGet(args *skel.CmdArgs) error {
	// TODO: implement
	return fmt.Errorf("not implemented")
}
