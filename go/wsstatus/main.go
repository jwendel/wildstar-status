// originally inspired fromTestPacketConnICMPEcho in http://golang.org/src/pkg/net/ipraw_test.go

// Pinger
package main

import (
	"fmt"
	// "github.com/jwendel/wildstar-status/go/ping"
	// "bytes"
	// "errors"
	"net"
	"os"
	// "strings"
	"time"
)

func main() {

	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s {hostname}\n", os.Args[0])
		os.Exit(1)
	}
	host := os.Args[1]

	fmt.Println("enter PacketConnICMPEcho")
	for i := 0; i < 1; i++ {
		PacketConnICMPEcho(host)
		time.Sleep(1 * time.Second)
	}
}

func (p *icmpEcho) Timestamp() (time.Time, error) {
	t := time.Time{}
	err := t.UnmarshalBinary(p.Data[:15])
	if err != nil {
		fmt.Println("icmpEcho.UnmarshalBinary problem: ", err)
	}

	return t, err
}

func PacketConnICMPEcho(host string) {

	c, err := net.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		fmt.Printf("ListenPacket failed: %v\n", err)
		panic("Need a raw socket to ping (which requires root/admin)")
	}
	c.SetDeadline(time.Now().Add(2000 * time.Millisecond))
	defer c.Close()

	ra, err := net.ResolveIPAddr("ip4:icmp", host)
	ra2, err := net.ResolveIPAddr("ip4:icmp", "ffxiv.com")
	if err != nil {
		fmt.Printf("ResolveIPAddr failed: %v\n", err)
	}
	typ := icmpv4EchoRequest
	xid, xseq := os.Getpid()&0xffff, 1
	before := time.Now()
	tdata, err := before.MarshalBinary()
	if err != nil {
		fmt.Println("time.MarshalBinary failed: ", err)
	}
	wb, err := (&icmpMessage{
		Type: typ, Code: 0,
		Body: &icmpEcho{
			ID:   xid,
			Seq:  xseq,
			Data: tdata,
		},
	}).Marshal()

	wb2, err := (&icmpMessage{
		Type: typ, Code: 0,
		Body: &icmpEcho{
			ID:   xid + 1,
			Seq:  xseq + 1,
			Data: tdata,
		},
	}).Marshal()

	if err != nil {
		fmt.Printf("icmpMessage.Marshal failed: %v\n", err)
	}
	if _, err := c.WriteTo(wb, ra); err != nil {
		fmt.Printf("PacketConn.WriteTo failed: %v\n", err)
	}
	if _, err := c.WriteTo(wb2, ra2); err != nil {
		fmt.Printf("PacketConn.WriteTo failed: %v\n", err)
	}
	var m *icmpMessage
	// c.ReadFrom puts the IP header into rb as a temp buffer
	// Needs to be 20 bytes larger for IPv4 header
	// rb := make([]byte, 20+len(wb))
	rb := make([]byte, 512)
	for {
		c.SetDeadline(time.Now().Add(2000 * time.Millisecond))
		n, addr, err := c.ReadFrom(rb)
		if err != nil {
			fmt.Printf("PacketConn.ReadFrom failed: %v\n", err)
			continue
		}
		data := rb[:n]
		if m, err = parseICMPMessage(data); err != nil {
			fmt.Printf("parseICMPMessage failed: %v\n", err)
		}

		switch m.Type {
		case icmpv4EchoRequest:
			continue
		case icmpv4EchoReply:
			switch p := m.Body.(type) {
			case *icmpEcho:
				tm, err := p.Timestamp()
				if err != nil {
					fmt.Println("icmpMessage timestamp parse problem")
				}
				fmt.Println("from packet time:", time.Since(tm))
				fmt.Println("from local time: ", time.Since(before))
				fmt.Printf("got id=%v, seqnum=%v, addr=%s\n", p.ID, p.Seq, addr.String())
			default:
				fmt.Printf("got type=%v, code=%v\n", m.Type, m.Code)
			}

		}
	}

}
