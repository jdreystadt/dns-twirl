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
	"net"
	"os"
	"os/signal"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

const dnsPort = ":1053"

var myLogger *log.Logger

type messageType interface {
	msgType() string
}

type signalMessage struct {
	signalValue os.Signal
}

func (p *signalMessage) msgType() string {
	return "signal"
}

type heartbeatMessage struct {
	hbValue time.Time
}

func (p *heartbeatMessage) msgType() string {
	return "heartbeat"
}

type statusMessage struct {
	statusValue string
}

func (p *statusMessage) msgType() string {
	return "status"
}

func main() {

	// Setup communication channels

	var responseChan = make(chan messageType, 1)
	var dnsControlChan = make(chan messageType, 1)

	myLogger = log.New(os.Stderr, "faux:", log.Lshortfile|log.LUTC|log.LstdFlags)
	_ = myLogger.Output(1, "Starting")

	go signalCatcher(responseChan)

	go heart(responseChan)

	go dnsServer(dnsControlChan, responseChan)

	// Wait for message on a channel

mainLoop:
	for true {
		select {
		case message := <-responseChan:
			switch message.msgType() {
			case "signal":
				break mainLoop
			case "heartbeat":
				select {
				case dnsControlChan <- message:
				default:
					_ = myLogger.Output(1, "Control channel is blocked")
				}
				dnsControlChan <- message
			case "status":
				var statusMsg = message.(*statusMessage)
				_ = myLogger.Output(1, fmt.Sprint("Got ", statusMsg.statusValue))
			}
			if message.msgType() == "signal" {
				break mainLoop
			}
		}
	}

	_ = myLogger.Output(1, "Ending")

}

func signalCatcher(resChan chan messageType) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	signal.Notify(signalChan, os.Kill)
	for true {
		caughtSignal := <-signalChan
		message := new(signalMessage)
		message.signalValue = caughtSignal
		resChan <- message
	}
}

func heart(resChan chan messageType) {
	for true {
		time.Sleep(2 * time.Second)
		message := new(heartbeatMessage)
		message.hbValue = time.Now()
		resChan <- message
	}
}

func dnsServer(controlChan, resChan chan messageType) {
	// Open the UDP port
	pc, err := net.ListenPacket("udp", dnsPort)
	if err != nil {
		return
	}
	// Close down when exiting
	defer pc.Close()
	// Create a buffer to hold the received messages
	var buffer = make([]byte, 65536)

mainLoop:
	for true {
		var deadline = time.Now().Add(1 * time.Second)
		err = pc.SetReadDeadline(deadline)
		if err != nil {
			_ = myLogger.Output(1, "Can't set a deadline")
			return
		}
		// Finally time to read
		n, addr, err := pc.ReadFrom(buffer)
		if err == nil {
			_ = myLogger.Output(1, fmt.Sprint("Got a buffer of length ", n, " from addr ", addr))
			var p dnsmessage.Parser
			if _, err := p.Start(buffer); err != nil {
				_ = myLogger.Output(1, fmt.Sprint("Parser did not Start ", err.Error()))
			}
			for {
				q, err := p.Question()
				if err == dnsmessage.ErrSectionDone {
					break
				}
				if err != nil {
					_ = myLogger.Output(1, fmt.Sprint("Error on Question ", err.Error()))
					break
				}
				_ = myLogger.Output(1, fmt.Sprint("Got a question ", q.Name.String()))
			}
		} else {
			// Got an error but this might be a timeout.
			nerr, ok := err.(net.Error)
			if !ok {
				// Type assertion failed
				_ = myLogger.Output(1, "Type assertion failure")
				return
			}
			if !nerr.Timeout() {
				// Error found that is not a timeout
				_ = myLogger.Output(1, fmt.Sprint("Error ", err.Error()))
				return
			}
		}
	drainControlChannel:
		for {
			select {
			case controlMsg := <-controlChan:
				switch controlMsg.msgType() {
				case "heartbeat":
					var message = new(statusMessage)
					message.statusValue = "Heartbeat on control channel"
					resChan <- message
				case "signal":
					break mainLoop
				}
			default:
				break drainControlChannel
			}
		}
	}
}
