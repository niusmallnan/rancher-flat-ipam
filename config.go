package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/rancher/rancher-cni-ipam/ipfinder/metadata"
	"github.com/vishvananda/netlink"
)

// IPAMConfig is used to load the options specified in the configuration file
type IPAMConfig struct {
	types.CommonArgs
	Type                 string        `json:"type"`
	LogToFile            string        `json:"logToFile"`
	IsDebugLevel         string        `json:"isDebugLevel"`
	SubnetPrefixSize     string        `json:"subnetPrefixSize"`
	Routes               []types.Route `json:"routes"`
	RancherContainerUUID types.UnmarshallableString
	IPAddress            types.UnmarshallableString
}

// Net loads the options of the CNI network configuration file
type Net struct {
	Name     string      `json:"name"`
	IPAM     *IPAMConfig `json:"ipam"`
	BrSubnet string      `json:"bridgeSubnet"`
	BrName   string      `json:"bridge"`
}

func (n *Net) getSubnetSize() (prefixSize string) {
	if n.IPAM.SubnetPrefixSize != "" {
		prefixSize = n.IPAM.SubnetPrefixSize
	} else {
		prefixSize = "/" + strings.SplitN(n.BrSubnet, "/", 2)[1]
	}
	return prefixSize
}

func (n *Net) getMetadataRoute() (route types.Route, err error) {
	metadataAddress := os.Getenv("RANCHER_METADATA_ADDRESS")
	if metadataAddress == "" {
		metadataAddress = metadata.DefaultMetadataAddress
	}
	_, metadataNet, err := net.ParseCIDR(fmt.Sprintf("%s/32", metadataAddress))
	if err != nil {
		return route, err
	}

	l, err := netlink.LinkByName(n.BrName)
	if err != nil {
		return route, err
	}
	addrs, err := netlink.AddrList(l, netlink.FAMILY_V4)
	if err != nil {
		return route, err
	}
	if len(addrs) == 0 {
		return route, fmt.Errorf("error getting no IP from flat bridge %s", n.BrName)
	}
	if len(addrs) > 1 {
		return route, fmt.Errorf("error getting one more IP from flat bridge %s", n.BrName)
	}

	bridgeIP := net.ParseIP(strings.SplitN(addrs[0].IPNet.String(), "/", 2)[0])

	return types.Route{Dst: *metadataNet, GW: bridgeIP}, nil
}

// LoadCNIConfig loads the IPAM configuration from the given bytes
func LoadCNIConfig(bytes []byte, args string) (*Net, error) {
	n := &Net{}
	if err := json.Unmarshal(bytes, n); err != nil {
		return nil, fmt.Errorf("failed to load netconf: %v", err)
	}

	if n.IPAM == nil {
		return nil, fmt.Errorf("IPAM config missing 'ipam' key")
	}

	if err := types.LoadArgs(args, n.IPAM); err != nil {
		return nil, fmt.Errorf("failed to parse args %s: %v", args, err)
	}

	return n, nil
}
