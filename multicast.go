package main

import (
	"net"
	"fmt"
	"golang.org/x/net/ipv6"
	"encoding/json"
)

func startMulticast(groupIPv6 string, measures <- chan interface{}) {
	group := net.ParseIP(groupIPv6)
	if group == nil || !group.IsMulticast() {
		panic(fmt.Errorf("Could not parse multicast IPv6 %s", groupIPv6))
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		fmt.Print(fmt.Errorf("localAddresses: %+v\n", err.Error()))
		return
	}

	multicastDestinations := make([]ipv6.PacketConn, 1)
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			fmt.Print(fmt.Errorf("localAddresses: %+v\n", err.Error()))
			continue
		}

		fmt.Printf("Adding interface %s for multicasts\n", i.Name)
		inter, err := net.InterfaceByName(i.Name)
		if err != nil {
			fmt.Println(err)
			return
		}

		for _, a := range addrs {
			addr, ok := a.(*net.IPNet)
			if ok && addr.IP.To4() == nil {
				fmt.Printf("Adding %s address to multicast\n", addr)

				c, err := net.ListenUDP("udp6", &net.UDPAddr{IP: addr.IP, Port: 10001, Zone: i.Name})
				if err != nil {
					panic(err)
				}

				p := ipv6.NewPacketConn(c)
				if err := p.JoinGroup(inter, &net.UDPAddr{IP: group}); err != nil {
					panic(err)
				}

				multicastDestinations = append(multicastDestinations, *p)
			}
		}
	}

	for {
		measurement, ok  := (<- measures).(Measurement)
		if !ok {
			fmt.Println("received wrong object")
			continue
		}

		jsonBytes, err := json.Marshal(measurement)
		if err != nil {
			fmt.Println("failed to marshal json")
			continue
		}

		for _, socket := range multicastDestinations {
			socket.WriteTo(jsonBytes, nil, &net.UDPAddr{IP: group, Port: 10000})
		}
	}
}