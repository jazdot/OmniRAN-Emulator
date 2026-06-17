package service

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"OmniRAN-Emulator/internal/control_test_engine/ue/context"
	"net"
	"strings"
	"syscall"
)

func InitDataPlane(ue *context.UEContext, pduSessionId uint8, gnbIp []byte) {

	// get UE GNB IP.
	ue.SetGnbIp(pduSessionId, gnbIp)

	// create interface for data plane.
	gatewayIp := ue.GetGatewayIp(pduSessionId)
	ueIp := ue.GetIp(pduSessionId)
	ueGnbIp := ue.GetGnbIp(pduSessionId)
	nameInf := fmt.Sprintf("uetun%d", pduSessionId)

	newInterface := &netlink.Iptun{
		LinkAttrs: netlink.LinkAttrs{
			Name: nameInf,
		},
		Local:  ueGnbIp,
		Remote: gatewayIp,
	}

	if err := netlink.LinkAdd(newInterface); err != nil {
		log.Info("[UE][DATA] Error in setting virtual interface: ", err)
		return
	}

	// Set IP interface up
	if err := netlink.LinkSetUp(newInterface); err != nil {
		log.Info("[UE][DATA] Error in setting virtual interface up: ", err)
		return
	}

	// Parse IP addresses (could be comma-separated for dual-stack)
	ipStrings := strings.Split(ueIp, ",")
	for _, ipStr := range ipStrings {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			log.Info("[UE][DATA] Invalid IP address: ", ipStr)
			continue
		}

		var ipNet *net.IPNet
		var dst *net.IPNet
		var src net.IP
		var family int

		if ip.To4() != nil {
			src = ip.To4()
			family = syscall.AF_INET
			ipNet = &net.IPNet{
				IP:   src,
				Mask: net.CIDRMask(32, 32),
			}
			dst = &net.IPNet{
				IP:   net.IPv4zero,
				Mask: net.CIDRMask(0, 32),
			}
		} else {
			src = ip
			family = syscall.AF_INET6
			ipNet = &net.IPNet{
				IP:   src,
				Mask: net.CIDRMask(64, 128),
			}
			dst = &net.IPNet{
				IP:   net.IPv6zero,
				Mask: net.CIDRMask(0, 128),
			}
		}

		// add IP address to link device
		addrTun := &netlink.Addr{
			IPNet: ipNet,
		}
		if err := netlink.AddrAdd(newInterface, addrTun); err != nil {
			log.Info("[UE][DATA] Error in adding IP ", ipStr, " for virtual interface: ", err)
			return
		}

		// create route in linux matching PDU table ID
		ueRoute := netlink.Route{
			LinkIndex: newInterface.Attrs().Index,
			Src:       src,
			Dst:       dst,
			Table:     int(pduSessionId),
		}
		if err := netlink.RouteAdd(&ueRoute); err != nil {
			log.Info("[UE][DATA] Error in setting route for ", ipStr, ": ", err)
			return
		}
		ue.SetTunRoute(pduSessionId, ueRoute)

		// create policy routing rule for source IP
		ueRule := netlink.NewRule()
		ueRule.Src = ipNet
		ueRule.Family = family
		ueRule.Table = int(pduSessionId)
		if err := netlink.RuleAdd(ueRule); err != nil {
			log.Info("[UE][DATA] Error in setting rule for ", ipStr, ": ", err)
			return
		}
		ue.SetTunRule(pduSessionId, *ueRule)
	}

	log.Info("[UE][DATA] UE is ready for using data plane")

	// context of tun interface
	ue.SetTunInterface(pduSessionId, newInterface)
}
