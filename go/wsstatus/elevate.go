// +build !windows

package main

import (
	"log"
	"syscall"
)

func setRoot() (olduid int) {
	// lets try to make ourselves root if we can
	olduid = syscall.Getuid()
	if olduid > 0 {
		err := syscall.Setuid(0)
		if err != nil {
			log.Println("Unable to setuid(0) - ", err)
		}
	}
}

func unsetRoot(olduid int) {
	if olduid > 0 {
		err := syscall.Setuid(olduid)
		if err != nil {
			log.Printf("Unable to setuid(%d) - %s", olduid, err)
		}
	}
}
