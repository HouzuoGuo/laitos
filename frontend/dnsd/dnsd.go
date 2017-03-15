package named

import (
	"bytes"
	"fmt"
	"net"
)

const MaxPacketSize = 1500 // Maximum acceptable UDP packet size

type DNSD struct {
	ForwardTo string `json:"ForwardTo"` // Forward DNS queries to this address

	ForwardUDPAddr *net.UDPAddr `json:"-"` // Constructed UDP address of the forwarder
}

// Send the UDP request packet to forwarder
func (dnsd *DNSD) AskForwarder(me *net.UDPConn, who *net.UDPAddr, packet []byte) {

}

func (dnsd *DNSD) ForwardRequest(me *net.UDPConn, who *net.UDPAddr, packet []byte) {
	remoteDNS, err := net.ResolveUDPAddr("udp", "8.8.8.8:53")
	if err != nil {
		panic(err)
	}
	remoteConn, err := net.DialUDP("udp", nil, remoteDNS)
	if err != nil {
		panic(err)
	}
	fmt.Println("Dialed")
	if _, err := remoteConn.Write(packet); err != nil {
		panic(err)
	}
	fmt.Println("Writen to remote")
	buf := make([]byte, 9000)
	n, _, err := remoteConn.ReadFromUDP(buf)
	if err != nil {
		panic(err)
	}
	fmt.Println("Read from remote")
	if _, err := me.WriteTo(buf[:n], who); err != nil {
		panic(err)
	}
}

func (dnsd *DNSD) StartAndBlock() error {
	addr, err := net.ResolveUDPAddr("udp", "0.0.0.0:53")
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	buf := make([]byte, 9000) // Highly doubt that UDP will work for that much data
	for {
		length, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			return err
		}
		isQuery := bytes.HasSuffix(buf[:length], []byte{0, 1, 0, 1})
		fmt.Println("Client is", clientAddr)
		fmt.Println("Is query?", isQuery)
		dnsd.ForwardRequest(conn, clientAddr, buf[:length])
		if isQuery {
			name := buf[13 : length-5]
			for i, b := range name {
				if b == 6 || b == 3 {
					name[i] = '.'
				}
			}
			fmt.Println("Query name is", string(name))
		}
	}
}
