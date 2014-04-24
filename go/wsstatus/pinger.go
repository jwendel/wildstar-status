package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	// "os"
	"syscall"
	"time"
)

const (
	maxHosts  = 10
	sendCount = 5
	sendEvery = 10
	sendDelay = 1
)

type ResultData struct {
	Success  bool
	Start    *time.Time
	End      *time.Time
	Duration *time.Time
}

type Result struct {
	HostId  int
	Dropped int
	Avg     float32
	Data    [sendCount]ResultData
}

type Pinger struct {
	Results <-chan Result
	hosts   []string
	rchan   chan Result
	running bool
	listen  net.PacketConn
}

func NewPinger() *Pinger {
	p := new(Pinger)
	p.hosts = make([]string, 0, maxHosts)
	p.rchan = make(chan Result, 10)
	p.Results = p.rchan
	return p
}

func (p *Pinger) AddHost(host string) int {
	idx := len(p.hosts)
	p.hosts = p.hosts[:idx+1]
	p.hosts[idx] = host
	return idx
}

func (p *Pinger) Start() error {
	if p.running {
		return errors.New("Pinger already running")
	}

	// lets try to make ourselves root if we can
	uid := syscall.Getuid()
	if uid > 0 {
		err := syscall.Setuid(0)
		if err != nil {
			log.Println("Unable to setuid(0) - ", err)
		}
	}
	c, err := net.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		fmt.Printf("ListenPacket failed: %v\n", err)
		return errors.New("Need a raw socket to ping (which requires root/admin)")
	}
	if uid > 0 {
		err := syscall.Setuid(uid)
		if err != nil {
			log.Printf("Unable to setuid(%d) - %s", uid, err)
		}
	}
	p.listen = c

	p.start()
	return nil
}

func (p *Pinger) start() {
	go p.icmpReciever()
	for id, h := range p.hosts {
		go p.pingHost(h, id+5)
	}
}

func (p *Pinger) pingHost(h string, id int) {
	ra, err := net.ResolveIPAddr("ip4:icmp", h)
	if err != nil {
		fmt.Printf("ResolveIPAddr failed: %v\n", err)
	}

	count := 0
	for {
		for i := 0; i < sendCount; i++ {
			p.packetConnICMPEcho(ra, id, count)
			count++
			select {
			case <-time.After(sendDelay * time.Second):
			}
		}
		select {
		case <-time.After(sendEvery * time.Second):
		}

	}
}

func (p *icmpEcho) Decode() (time.Time, error) {
	t := time.Time{}
	err := t.UnmarshalBinary(p.Data[:15])
	if err != nil {
		fmt.Println("icmpEcho.UnmarshalBinary problem: ", err)
	}

	return t, err
}

func (p *Pinger) icmpReciever() {

	var m *icmpMessage
	// Needs to be 20 bytes larger for IPv4 header
	// rb := make([]byte, 20+len(wb))
	rb := make([]byte, 512)
	for {
		// p.listen.SetDeadline(time.Now().Add(1000 * time.Millisecond))
		n, addr, err := p.listen.ReadFrom(rb)
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
				tm, err := p.Decode()
				if err != nil {
					fmt.Println("icmpMessage timestamp parse problem")
				}
				fmt.Println("from packet time:", time.Since(tm))

				fmt.Printf("got id=%v, seqnum=%v, addr=%s\n", p.ID, p.Seq, addr.String())
			default:
				fmt.Printf("got type=%v, code=%v\n", m.Type, m.Code)
			}

		}
	}
}

func (p *Pinger) packetConnICMPEcho(ra *net.IPAddr, id, seq int) {
	fmt.Printf("sending to %v id:%v seq:%v\n", ra, id, seq)
	typ := icmpv4EchoRequest
	// xid, xseq := os.Getpid()&0xffff, 1
	before := time.Now()
	tdata, err := before.MarshalBinary()
	// p.listen.SetWriteDeadline(time.Second)
	if err != nil {
		fmt.Println("time.MarshalBinary failed: ", err)
	}
	wb, err := (&icmpMessage{
		Type: typ, Code: 0,
		Body: &icmpEcho{
			ID:   id,
			Seq:  seq,
			Data: tdata,
		},
	}).Marshal()

	if err != nil {
		fmt.Printf("icmpMessage.Marshal failed: %v\n", err)
	}
	if _, err := p.listen.WriteTo(wb, ra); err != nil {
		fmt.Printf("PacketConn.WriteTo failed: %v\n", err)
	}

}
