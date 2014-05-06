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
	sendCount = 5
	sendEvery = 10
	sendDelay = 1
	ID_OFFSET = 1100
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

type targetHost struct {
	pingSent    chan int
	pingHandler chan *ping
	host        string
	ip          *IPAddr
}

type Pinger struct {
	Results   <-chan Result
	targets   []targetHost
	hosts     []string
	receivers []chan *ping
	rchan     chan Result
	running   bool
	listen    net.PacketConn
}

type ping struct {
	data    *icmpEcho
	rcvTime time.Time
}

func NewPinger() *Pinger {
	p := new(Pinger)
	p.hosts = make([]string, 0, 10)
	p.targets = make([]targetHost, 0, 10)
	p.receivers = make([]chan *ping, 0, 10)
	p.rchan = make(chan Result, 10)
	p.Results = p.rchan
	return p
}

func (p *Pinger) AddHost(host string) int {
	idx := len(p.hosts)
	p.targets = append(p.targets, &targetHost{})
	p.hosts = append(p.hosts, host)
	p.receivers = append(p.receivers, make(chan *ping))
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
		rcvd := p.receivers[id]
		sent := make(chan int, sendCount)
		go p.receiver(sent, rcvd)
		go p.pingHost(h, id+ID_OFFSET, sent)
	}
}

func (p *Pinger) pingHost(h string, id int, sent chan<- int) {
	ra, err := net.ResolveIPAddr("ip4:icmp", h)
	if err != nil {
		fmt.Printf("ResolveIPAddr failed: %v\n", err)
	}

	count := 0
	for {
		for i := 0; i < sendCount; i++ {
			p.sendEchoReq(ra, id, count)
			sent <- count
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

func (p *Pinger) receiver(sent <-chan int, rcvd chan *ping) {

	for {
		select {
		case _ = <-sent:
			// fmt.Println("from sent:", s)
		case r := <-rcvd:
			// fmt.Println("from rcvd:", r)
			tm, err := r.data.Decode()
			if err != nil {
				fmt.Println("icmpMessage timestamp parse problem")
				continue
			}
			fmt.Printf("got id=%v, seqnum=%v, time:%v\n", r.data.ID, r.data.Seq, r.rcvTime.Sub(tm))
		}
	}

}

func (p Pinger) handler(id int) (chan<- *ping, error) {
	idx := id - ID_OFFSET
	if idx >= 0 && idx < len(p.receivers) {
		return p.receivers[idx], nil
	}
	return nil, fmt.Errorf("Invalid id %v returned.  Index would be %v.", id, idx)
}

func (p *Pinger) icmpReciever() {

	var m *icmpMessage
	// Needs to be 20 bytes larger for the IPv4 header
	// rb := make([]byte, 20+len(wb))
	rb := make([]byte, 512)
	for {
		// p.listen.SetDeadline(time.Now().Add(1000 * time.Millisecond))
		n, _, err := p.listen.ReadFrom(rb)
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
			switch ie := m.Body.(type) {
			case *icmpEcho:
				res := &ping{ie, time.Now()}
				handler, err := p.handler(ie.ID)
				if err == nil {
					handler <- res
				} else {
					fmt.Println(err)
				}
			default:
				fmt.Printf("got type=%v, code=%v\n", m.Type, m.Code)
			}

		}
	}
}

// sendEchoReq will issue an IPv4 ICMP Echo Request to the given raddr.
func (p *Pinger) sendEchoReq(raddr *net.IPAddr, id, seq int) {
	fmt.Printf("sending to %v id:%v seq:%v\n", raddr, id, seq)

	// Do we even need a write timeout for ICMP?  I think not.
	// p.listen.SetWriteDeadline(time.Now().Add(2 * time.Second))

	// Encode current time into echo packet data
	before := time.Now()
	tdata, err := before.MarshalBinary()
	if err != nil {
		fmt.Println("time.MarshalBinary failed: ", err)
	}

	wb, err := (&icmpMessage{
		Type: icmpv4EchoRequest,
		Code: 0,
		Body: &icmpEcho{
			ID:   id,
			Seq:  seq,
			Data: tdata,
		},
	}).Marshal()

	if err != nil {
		fmt.Printf("icmpMessage.Marshal failed: %v\n", err)
		return
	}
	if _, err := p.listen.WriteTo(wb, raddr); err != nil {
		fmt.Printf("PacketConn.WriteTo failed: %v\n", err)
	}

}
