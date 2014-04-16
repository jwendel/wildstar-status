// originally inspired fromTestPacketConnICMPEcho in http://golang.org/src/pkg/net/ipraw_test.go

// Pinger
package main

import (
	"fmt"
	// "github.com/jwendel/wildstar-status/go/ping"
	// "bytes"
	// "errors"
	// "net"
	"os"
	// "strings"
	// "time"
)

func main() {

	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s {hostname}\n", os.Args[0])
		os.Exit(1)
	}
	host := os.Args[1]

	// fmt.Println("enter PacketConnICMPEcho")
	// for i := 0; i < 1; i++ {
	// 	PacketConnICMPEcho(host)
	// 	time.Sleep(1 * time.Second)
	// }
	p := NewPinger()
	p.AddHost(host)
	err := p.Start()
	if err != nil {
		fmt.Println(err)
		return
	}

	for {
		select {
		case ping := <-p.Results:
			fmt.Println("avg ping:", ping.Avg)
		}
	}

}
