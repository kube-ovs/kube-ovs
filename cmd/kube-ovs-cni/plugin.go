package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ipam"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/digitalocean/go-openvswitch/ovs"
	"github.com/j-keck/arping"
	"github.com/vishvananda/netlink"
)

const defaultBridgeName = "kube-ovs0"

type NetConf struct {
	types.NetConf

	BridgeName string `json:"bridge"`
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
		// hostIface.Sandbox = hostNS.Path() // this doesn't exist in other plugins, should this be removed?
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

		// Configure the container hardware address and IP address(es)
		if err := netns.Do(func(_ ns.NetNS) error {
			contVeth, err := net.InterfaceByName(args.IfName)
			if err != nil {
				return err
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
