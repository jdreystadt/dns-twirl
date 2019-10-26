// This is the main file for the dns-twirl program. This program is just a
// simple DNS repeater that uses a simple file of local addresses to provide
// name lookups for the local network.
//
// The purpose of this program is mostly just to be an example for later
// work.

package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"
)

var myLogger *log.Logger

type messageType interface {
	msgType() string
}

type signalMessage struct {
	signalValue os.Signal
}

func (msg signalMessage) msgType() string {
	return "signal"
}

type heartbeatMessage struct {
	hbValue time.Time
}

func (msg heartbeatMessage) msgType() string {
	return "heartbeat"
}

func main() {

	// Setup communication channels

	responseChan := make(chan messageType, 1)

	myLogger = log.New(os.Stderr, "faux:", log.Lshortfile|log.LUTC|log.LstdFlags)
	_ = myLogger.Output(1, "Starting")

	go signalCatcher(responseChan)

	go heart(responseChan)

	// Wait for message on a channel

mainLoop:
	for true {
		select {
		case message := <-responseChan:
			switch message.msgType() {
			case "signal":
				break mainLoop
			case "heartbeat":
				hbMessage := message.(*heartbeatMessage)
				_ = myLogger.Output(1, fmt.Sprint("Heartbeat ", hbMessage.hbValue))
			}
		}
	}

	_ = myLogger.Output(1, "Ending")

}

func signalCatcher(resChan chan messageType) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	for true {
		caughtSignal := <-signalChan
		message := new(signalMessage)
		message.signalValue = caughtSignal
		resChan <- message
	}
}

func heart(resChan chan messageType) {
	for true {
		time.Sleep(1 * time.Second)
		message := new(heartbeatMessage)
		message.hbValue = time.Now()
		resChan <- message
	}
}
