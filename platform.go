// +build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

func registerSignals(s chan string) {
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt)
	signal.Notify(sigchan, syscall.SIGTSTP)
	go func() {
		for sig := range sigchan {
			switch sig {
			case os.Interrupt:
				s <- "interrupt"
			case syscall.SIGTSTP:
				s <- "stop"
			}
		}
	}()
}
