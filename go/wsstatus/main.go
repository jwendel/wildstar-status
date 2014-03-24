package main

import (
	"fmt"
	// "github.com/jwendel/wildstar-status/go/ping"
	"bytes"
	"errors"
	"net"
	"os"
	"strings"
	"time"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s {hostname}\n", os.Args[0])
		os.Exit(1)
	}
	host := os.Args[1]

	fmt.Println("enter ConnICMPEcho")
	for i := 0; i < 10; i++ {
		ConnICMPEcho()
		time.Sleep(1 * time.Second)
	}
	fmt.Println("leave")
	fmt.Println("enter PacketConnICMPEcho")
	for i := 0; i < 10; i++ {
		PacketConnICMPEcho()
		time.Sleep(1 * time.Second)
	}
	fmt.Println("leave")
}

var icmpEchoTests = []struct {
	net   string
	laddr string
	raddr string
}{
	{"ip4:icmp", "0.0.0.0", "69.164.194.198"},
	// {"ip6:ipv6-icmp", "::", "::1"},
}

const (
	icmpv4EchoRequest = 8
	icmpv4EchoReply   = 0
	icmpv6EchoRequest = 128
	icmpv6EchoReply   = 129
)

// icmpMessage represents an ICMP message.
type icmpMessage struct {
	Type     int             // type
	Code     int             // code
	Checksum int             // checksum
	Body     icmpMessageBody // body
}

// icmpMessageBody represents an ICMP message body.
type icmpMessageBody interface {
	Len() int
	Marshal() ([]byte, error)
}

// imcpEcho represenets an ICMP echo request or reply message body.
type icmpEcho struct {
	ID   int    // identifier
	Seq  int    // sequence number
	Data []byte // data
}

// Len returns message length in bytes
func (p *icmpEcho) Len() int {
	if p == nil {
		return 0
	}
	return 4 + len(p.Data)
}

// Marshal returns the binary enconding of the ICMP echo request or
// reply message body p.
func (p *icmpEcho) Marshal() ([]byte, error) {
	b := make([]byte, 4+len(p.Data))
	b[0], b[1] = byte(p.ID>>8), byte(p.ID)
	b[2], b[3] = byte(p.Seq>>8), byte(p.Seq)
	copy(b[4:], p.Data)
	return b, nil
}

// parseICMPEcho parses b as an ICMP echo request or reply message
// body.
func parseICMPEcho(b []byte) (*icmpEcho, error) {
	bodylen := len(b)
	p := &icmpEcho{ID: int(b[0])<<8 | int(b[1]), Seq: int(b[2])<<8 | int(b[3])}
	if bodylen > 4 {
		p.Data = make([]byte, bodylen-4)
		copy(p.Data, b[4:])
	}
	return p, nil
}

func ConnICMPEcho(host raddr) {

	c, err := net.Dial("ip4:icmp", tt.raddr)
	if err != nil {
		fmt.Printf("Dial failed: %v\n", err)
		panic("ahhh")
	}
	c.SetDeadline(time.Now().Add(1000 * time.Millisecond))
	defer c.Close()

	typ := icmpv4EchoRequest
	if strings.Contains(tt.net, "ip6") {
		typ = icmpv6EchoRequest
	}
	xid, xseq := os.Getpid()&0xffff, i+1
	wb, err := (&icmpMessage{
		Type: typ, Code: 0,
		Body: &icmpEcho{
			ID: xid, Seq: xseq,
			Data: bytes.Repeat([]byte("Go Go Gadget Ping!!!"), 3),
		},
	}).Marshal()
	if err != nil {
		fmt.Printf("icmpMessage.Marshal failed: %v\n", err)
	}
	before := time.Now()
	if _, err := c.Write(wb); err != nil {
		fmt.Printf("Conn.Write failed: %v\n", err)
	}
	var m *icmpMessage
	rb := make([]byte, 20+len(wb))
	for {
		if _, err := c.Read(rb); err != nil {
			fmt.Printf("Conn.Read failed: %v\n", err)
		}
		if strings.Contains(tt.net, "ip4") {
			rb = ipv4Payload(rb)
		}
		if m, err = parseICMPMessage(rb); err != nil {
			fmt.Printf("parseICMPMessage failed: %v\n", err)
		}
		fmt.Println("Conn data: ", m)
		switch m.Type {
		case icmpv4EchoRequest, icmpv6EchoRequest:
			continue
		}
		break
	}
	fmt.Println(time.Since(before))

	switch p := m.Body.(type) {
	case *icmpEcho:
		fmt.Printf("got id=%v, seqnum=%v; expected id=%v, seqnum=%v\n", p.ID, p.Seq, xid, xseq)
	default:
		fmt.Printf("got type=%v, code=%v; expected type=%v, code=%v\n", m.Type, m.Code, typ, 0)
	}
}

func PacketConnICMPEcho() {
	for i, tt := range icmpEchoTests {

		c, err := net.ListenPacket("ip4:icmp", tt.laddr)
		if err != nil {
			fmt.Printf("ListenPacket failed: %v\n", err)
		}
		c.SetDeadline(time.Now().Add(1000 * time.Millisecond))
		defer c.Close()

		ra, err := net.ResolveIPAddr(tt.net, tt.raddr)
		if err != nil {
			fmt.Printf("ResolveIPAddr failed: %v\n", err)
		}
		typ := icmpv4EchoRequest
		if strings.Contains(tt.net, "ip6") {
			typ = icmpv6EchoRequest
		}
		xid, xseq := os.Getpid()&0xffff, i+1
		wb, err := (&icmpMessage{
			Type: typ, Code: 0,
			Body: &icmpEcho{
				ID: xid, Seq: xseq,
				Data: bytes.Repeat([]byte("Go Go Gadget Ping!!!"), 3),
			},
		}).Marshal()
		if err != nil {
			fmt.Printf("icmpMessage.Marshal failed: %v\n", err)
		}
		before := time.Now()
		before.Nanosecond()
		if _, err := c.WriteTo(wb, ra); err != nil {
			fmt.Printf("PacketConn.WriteTo failed: %v\n", err)
		}
		var m *icmpMessage
		rb := make([]byte, 20+len(wb))
		for {
			if _, _, err := c.ReadFrom(rb); err != nil {
				fmt.Printf("PacketConn.ReadFrom failed: %v\n", err)
			}
			// See BUG section.
			//if net == "ip4" {
			//	rb = ipv4Payload(rb)
			//}
			if m, err = parseICMPMessage(rb); err != nil {
				fmt.Printf("parseICMPMessage failed: %v\n", err)
			}
			fmt.Println("Packet data: ", m)
			switch m.Type {
			case icmpv4EchoRequest, icmpv6EchoRequest:
				continue
			}
			break
		}
		fmt.Println(time.Since(before))

		switch p := m.Body.(type) {
		case *icmpEcho:
			fmt.Printf("got id=%v, seqnum=%v; expected id=%v, seqnum=%v\n", p.ID, p.Seq, xid, xseq)
		default:
			fmt.Printf("got type=%v, code=%v; expected type=%v, code=%v\n", m.Type, m.Code, typ, 0)
		}
	}
}

// Marshal returns the binary enconding of the ICMP echo request or
// reply message m.
func (m *icmpMessage) Marshal() ([]byte, error) {
	b := []byte{byte(m.Type), byte(m.Code), 0, 0}
	if m.Body != nil && m.Body.Len() != 0 {
		mb, err := m.Body.Marshal()
		if err != nil {
			return nil, err
		}
		b = append(b, mb...)
	}
	switch m.Type {
	case icmpv6EchoRequest, icmpv6EchoReply:
		return b, nil
	}
	csumcv := len(b) - 1 // checksum coverage
	s := uint32(0)
	for i := 0; i < csumcv; i += 2 {
		s += uint32(b[i+1])<<8 | uint32(b[i])
	}
	if csumcv&1 == 0 {
		s += uint32(b[csumcv])
	}
	s = s>>16 + s&0xffff
	s = s + s>>16
	// Place checksum back in header; using ^= avoids the
	// assumption the checksum bytes are zero.
	b[2] ^= byte(^s)
	b[3] ^= byte(^s >> 8)
	return b, nil
}

// parseICMPMessage parses b as an ICMP message.
func parseICMPMessage(b []byte) (*icmpMessage, error) {
	msglen := len(b)
	if msglen < 4 {
		return nil, errors.New("message too short")
	}
	m := &icmpMessage{Type: int(b[0]), Code: int(b[1]), Checksum: int(b[2])<<8 | int(b[3])}
	if msglen > 4 {
		var err error
		switch m.Type {
		case icmpv4EchoRequest, icmpv4EchoReply, icmpv6EchoRequest, icmpv6EchoReply:
			m.Body, err = parseICMPEcho(b[4:])
			if err != nil {
				return nil, err
			}
		}
	}
	return m, nil
}

func ipv4Payload(b []byte) []byte {
	if len(b) < 20 {
		return b
	}
	hdrlen := int(b[0]&0x0f) << 2
	return b[hdrlen:]
}
