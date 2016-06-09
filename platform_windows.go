//+build windows
package main

import (
	"os"
	"os/signal"
)

func registerSignals(s chan string) {
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt)
	go func() {
		for sig := range sigchan {
			switch sig {
			case os.Interrupt:
				s <- "interrupt"
			}
		}
	}()
}
