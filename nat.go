package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"time"
)

func natTest(serverHost string, serverPort int, localPort int) (publicIP string, isPublicIP int, publicPort int, err error) {
	conn, err := net.ListenPacket("udp", fmt.Sprintf(":%d", localPort))
	if err != nil {
		return "", 0, 0, err
	}
	defer conn.Close()

	dst, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", serverHost, serverPort))
	if err != nil {
		return "", 0, 0, err
	}

	// The connection can write data to the desired address.
	msg, err := newMessage(MsgNATDetect, 0, &NatDetectReq{SrcPort: localPort, EchoPort: EchoPort})
	_, err = conn.WriteTo(msg, dst)
	if err != nil {
		return "", 0, 0, err
	}
	deadline := time.Now().Add(NatTestTimeout)
	err = conn.SetReadDeadline(deadline)
	if err != nil {
		return "", 0, 0, err
	}
	buffer := make([]byte, 1024)
	nRead, _, err := conn.ReadFrom(buffer)
	if err != nil {
		gLog.Println(LevelERROR, "NAT detect error:", err)
		return "", 0, 0, err
	}
	natRsp := NatDetectRsp{}
	err = json.Unmarshal(buffer[openP2PHeaderSize:nRead], &natRsp)
	return natRsp.IP, natRsp.IsPublicIP, natRsp.Port, nil
}

func getNATType(host string, udp1 int, udp2 int) (publicIP string, NATType int, err error) {
	// the random local port may be used by other.
	go echo()
	localPort := int(rand.Uint32()%10000 + 50000)
	ip1, isPublicIP, port1, err := natTest(host, udp1, localPort)
	gLog.Printf(LevelDEBUG, "local port:%d  nat port:%d", localPort, port1)
	if err != nil {
		return "", 0, err
	}
	if isPublicIP == 1 {
		return ip1, NATNone, nil
	}
	ip2, _, port2, err := natTest(host, udp2, localPort)
	gLog.Printf(LevelDEBUG, "local port:%d  nat port:%d", localPort, port2)
	if err != nil {
		return "", 0, err
	}
	if ip1 != ip2 {
		return "", 0, fmt.Errorf("ip have changed, please retry again")
	}
	natType := NATSymmetric
	if port1 == port2 {
		natType = NATCone
	}
	//TODO: NATNone
	return ip1, natType, nil
}

const (
	UDPPort1 = 27182
	UDPPort2 = 27183
	EchoPort = 31415
)

func echo() {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: EchoPort})
	if err != nil {
		gLog.Println(LevelERROR, "echo server listen error:", err)
		return
	}
	buf := make([]byte, 1600)
	defer conn.Close()
	// wait 5s for echo testing
	conn.SetReadDeadline(time.Now().Add(time.Second * 5))
	n, addr, err := conn.ReadFromUDP(buf)
	if err != nil {
		return
	}
	conn.WriteToUDP(buf[0:n], addr)
}