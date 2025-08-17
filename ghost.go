package main

import (
	"bufio"
	"crypto/tls"
	"io"
	"log"
	"net"
	"sync"

	ircgomsg "github.com/ergochat/irc-go/ircmsg"
)

func ghost(serverAddress, listenAddress string) {
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", listenAddress, err)
	}
	defer listener.Close()

	log.Printf("IRC Bouncer listening on %s", listenAddress)
	log.Printf("Connecting clients to IRC server: %s", serverAddress)

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept client connection: %v", err)

			continue
		}

		go handleClientConnection(clientConn, serverAddress)
	}
}

func handleClientConnection(clientConn net.Conn, serverAddress string) {
	log.Printf("Client connected from: %s", clientConn.RemoteAddr())

	// ircServerConn, err := net.Dial("tcp", serverAddress)
	ircServerConn, err := tls.Dial("tcp", serverAddress, nil)
	if err != nil {
		log.Printf("Failed to connect to IRC server %s: %v", serverAddress, err)
		clientConn.Close()

		return
	}

	log.Printf("Successfully connected to IRC server: %s", serverAddress)

	done := make(chan struct{})

	var once sync.Once

	closeConnections := func() {
		once.Do(func() {
			clientConn.Close()
			ircServerConn.Close()
			close(done)
		})
	}

	go relay(clientConn, ircServerConn, closeConnections)

	go relay(ircServerConn, clientConn, closeConnections)

	<-done
	log.Printf("Connection closed for client: %s", clientConn.RemoteAddr())
}

func relay(src, dest net.Conn, closer func()) {
	reader := bufio.NewReader(src)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("Relay read error on: %v", err)
			}
			closer()

			return
		}

		msg, err := ircgomsg.ParseLine(line)
		if err != nil {
			log.Printf("Failed to parse IRC message: %v", err)
		}

		if msg.Command == "PRIVMSG" {
		}

		if msg.Command == "MSG" {
		}

		log.Println(msg)

		_, err = io.WriteString(dest, line)
		if err != nil {
			closer()

			return
		}
	}
}
