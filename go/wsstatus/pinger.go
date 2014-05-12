package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	// "os"
	"time"
)

const (
	sendCount = 5
	sendEvery = 20
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
	ip          *net.IPAddr
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
	p.targets = append(p.targets, targetHost{host: host})
	p.hosts = append(p.hosts, host)
	p.receivers = append(p.receivers, make(chan *ping))
	return idx
}

func (p *Pinger) Start() error {
	if p.running {
		return errors.New("Pinger already running")
	}

	olduid := setRoot()
	c, err := net.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		log.Printf("ListenPacket failed: %v\n", err)
		return errors.New("Need a raw socket to ping (which requires root/admin)")
	}
	unsetRoot(olduid)
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
		log.Printf("ResolveIPAddr failed: %v\n", err)
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
			// log.Println("from sent:", s)
		case r := <-rcvd:
			// log.Println("from rcvd:", r)
			tm, err := r.data.Decode()
			if err != nil {
				log.Println("icmpMessage timestamp parse problem")
				continue
			}
			log.Printf("got id=%v, seqnum=%v, time:%v\n", r.data.ID, r.data.Seq, r.rcvTime.Sub(tm))
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
			log.Printf("PacketConn.ReadFrom failed: %v\n", err)
			continue
		}
		data := rb[:n]
		if m, err = parseICMPMessage(data); err != nil {
			log.Printf("parseICMPMessage failed. bytecount=%v, data=%v. Error: %v\n", n, data, err)
		}

		log.Println("receieved", data)

		switch m.Type {
		case icmpv4EchoRequest:
			log.Printf("got req type=%v, code=%v\n", m.Type, m.Code)
		case icmpv4EchoReply:
			switch ie := m.Body.(type) {
			case *icmpEcho:
				res := &ping{ie, time.Now()}
				handler, err := p.handler(ie.ID)
				if err == nil {
					handler <- res
				} else {
					log.Println(err)
				}
			default:
				log.Printf("got reply type=%v, code=%v\n", m.Type, m.Code)
			}
		case icmpv4DestUnreachable:
			// TODO: pass error to reciever
			log.Printf("Destination unreachable type=%v code=%v codemsg=%v", m.Type, m.Code, m.DestUnreachableMsg())
		default:
			log.Println("error: ", m)
		}
	}
}

// sendEchoReq will issue an IPv4 ICMP Echo Request to the given raddr.
func (p *Pinger) sendEchoReq(raddr *net.IPAddr, id, seq int) {
	log.Printf("sending to %v id:%v seq:%v\n", raddr, id, seq)

	// Do we even need a write timeout for ICMP?  I think not.
	// p.listen.SetWriteDeadline(time.Now().Add(2 * time.Second))

	// Encode current time into echo packet data
	before := time.Now()
	tdata, err := before.MarshalBinary()
	if err != nil {
		log.Println("time.MarshalBinary failed: ", err)
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
		log.Printf("icmpMessage.Marshal failed: %v\n", err)
		return
	}
	if _, err := p.listen.WriteTo(wb, raddr); err != nil {
		log.Printf("PacketConn.WriteTo failed: %v\n", err)
	}

}
