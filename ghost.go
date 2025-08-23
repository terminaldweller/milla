package main

import (
	"bufio"
	"crypto/tls"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"sync"

	ircgomsg "github.com/ergochat/irc-go/ircmsg"
	"golang.org/x/net/proxy"
)

func RunGhost(ghostNetwork GhostNetwork, name string) {
	var listener net.Listener

	var err error

	if ghostNetwork.ServerKey == "" || ghostNetwork.ServerCert == "" {
		log.Printf("Ghost %s: either one or both of ServerKey and ServerCert were not provided. ghosty will not be listening on TLS.", name)

		listener, err = net.Listen("tcp", ghostNetwork.ListenAddress)
		if err != nil {
			log.Fatalf("Ghost %s: Failed to listen on %s: %v", name, ghostNetwork.ListenAddress, err)
		}
	} else {
		tlsCert, err := os.ReadFile(ghostNetwork.ServerCert)
		if err != nil {
			log.Fatalf("Ghost %s: Failed to read TLS certificate file: %v", name, err)
		}

		tlsKey, err := os.ReadFile(ghostNetwork.ServerKey)
		if err != nil {
			log.Fatalf("Ghost %s: Failed to read TLS key file: %v", name, err)
		}

		cert, err := tls.X509KeyPair(tlsCert, tlsKey)
		if err != nil {
			log.Fatalf("Ghost %s: Failed to load TLS key pair: %v", name, err)
		}

		listener, err = tls.Listen("tcp", ghostNetwork.ListenAddress, &tls.Config{
			MinVersion:   tls.VersionTLS13,
			Certificates: []tls.Certificate{cert},
		})
		if err != nil {
			log.Fatalf("Ghost %s: Failed to listen on %s: %v", name, ghostNetwork.ListenAddress, err)
		}
	}

	defer listener.Close()

	log.Printf("Ghost %s: IRC Bouncer listening on %s", name, ghostNetwork.ListenAddress)
	log.Printf("Ghost %s: Connecting clients to IRC server: %s", name, ghostNetwork.ServerAddress)

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			log.Printf("Ghost %s: Failed to accept client connection: %v", name, err)

			continue
		}

		go handleClientConnection(clientConn, ghostNetwork, name)
	}
}

func proxyConnectinoTLS(clientConn net.Conn, ghostNetwork GhostNetwork, name string) *tls.Conn {
	dialer, err := proxy.SOCKS5("tcp", ghostNetwork.UpstreamProxy, nil, proxy.Direct)
	if err != nil {
		log.Fatalf("Ghost %s: Failed to create SOCKS5 dialer: %v", name, err)
	}

	proxyCon, err := dialer.Dial("tcp", ghostNetwork.ServerAddress)
	if err != nil {
		log.Printf("Ghost %s: Failed to connect to IRC server %s via proxy %s: %v", name, ghostNetwork.ServerAddress, ghostNetwork.UpstreamProxy, err)
		clientConn.Close()

		return nil
	}

	var tlsConfig *tls.Config

	if ghostNetwork.CertPath != "" && ghostNetwork.KeyPath != "" {
		clientCert, err := tls.LoadX509KeyPair(ghostNetwork.CertPath, ghostNetwork.KeyPath)
		if err != nil {
			log.Fatalf("Ghost %s: Failed to load client TLS key pair: %v", name, err)
		}

		tlsConfig = &tls.Config{
			ServerName:         ghostNetwork.ServerName,
			Certificates:       []tls.Certificate{clientCert},
			InsecureSkipVerify: ghostNetwork.SkipTLSVerify,
			MinVersion:         tls.VersionTLS13,
		}
	} else {
		tlsConfig = &tls.Config{
			ServerName:         ghostNetwork.ServerName,
			InsecureSkipVerify: ghostNetwork.SkipTLSVerify,
			MinVersion:         tls.VersionTLS13,
		}
	}

	conn := tls.Client(proxyCon, tlsConfig)

	err = conn.Handshake()
	if err != nil {
		log.Printf("Ghost %s: TLS handshake with IRC server %s failed: %v", name, ghostNetwork.ServerAddress, err)
		clientConn.Close()

		return nil
	}

	return conn
}

func connectionTLS(clientConn net.Conn, ghostNetwork GhostNetwork, name string) *tls.Conn {
	clientCert, err := tls.LoadX509KeyPair(ghostNetwork.CertPath, ghostNetwork.KeyPath)
	if err != nil {
		log.Fatalf("Ghost %s: Failed to load client TLS key pair: %v", name, err)
	}

	tlsConfig := &tls.Config{
		ServerName:         ghostNetwork.ServerName,
		Certificates:       []tls.Certificate{clientCert},
		InsecureSkipVerify: ghostNetwork.SkipTLSVerify,
		MinVersion:         tls.VersionTLS13,
	}

	conn, err := tls.Dial("tcp", ghostNetwork.ServerAddress, tlsConfig)
	if err != nil {
		log.Printf("Ghost %s: Failed to connect to IRC server %s with TLS: %v", name, ghostNetwork.ServerAddress, err)
		clientConn.Close()

		return nil
	}

	return conn
}

func proxyConnection(clientConn net.Conn, ghostNetwork GhostNetwork, name string) net.Conn {
	dialer, err := proxy.SOCKS5("tcp", ghostNetwork.UpstreamProxy, nil, proxy.Direct)
	if err != nil {
		log.Fatalf("Ghost %s: Failed to create SOCKS5 dialer: %v", name, err)
	}

	proxyCon, err := dialer.Dial("tcp", ghostNetwork.ServerAddress)
	if err != nil {
		log.Printf("Ghost %s: Failed to connect to IRC server %s via proxy %s: %v", name, ghostNetwork.ServerAddress, ghostNetwork.UpstreamProxy, err)
		clientConn.Close()

		return nil
	}

	return proxyCon
}

func handleClientConnection(clientConn net.Conn, ghostNetwork GhostNetwork, name string) {
	log.Printf("Ghost %s: Client connected from: %s", name, clientConn.RemoteAddr())

	var ircServerConn net.Conn

	var err error

	if ghostNetwork.UpstreamProxy == "" {
		if ghostNetwork.UseTLS {
			ircServerConn = connectionTLS(clientConn, ghostNetwork, name)
		} else {
			ircServerConn, err = net.Dial("tcp", ghostNetwork.ServerAddress)
			if err != nil {
				log.Printf("Ghost %s: Failed to connect to IRC server %s: %v", name, ghostNetwork.ServerAddress, err)
				clientConn.Close()

				return
			}
		}
	} else {
		if ghostNetwork.UseTLS {
			ircServerConn = proxyConnectinoTLS(clientConn, ghostNetwork, name)
		} else {
			ircServerConn = proxyConnection(clientConn, ghostNetwork, name)
		}
	}

	if ircServerConn == nil {
		return
	}

	log.Printf("Ghost %s: Successfully connected to IRC server: %s", name, ghostNetwork.ServerAddress)

	done := make(chan struct{})

	var once sync.Once

	closeConnections := func() {
		once.Do(func() {
			clientConn.Close()
			ircServerConn.Close()
			close(done)
		})
	}

	go relay(ghostNetwork.LogRaw, clientConn, ircServerConn, closeConnections, name)

	go relay(ghostNetwork.LogRaw, ircServerConn, clientConn, closeConnections, name)

	<-done
}

func relay(logRaw bool, src, dest net.Conn, closer func(), name string) {
	reader := bufio.NewReader(src)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if !errors.Is(err, io.EOF) {
				log.Printf("Ghost %s: Relay read error on: %v", name, err)
			}

			closer()

			return
		}

		msg, err := ircgomsg.ParseLine(line)
		if err != nil {
			log.Printf("Ghost %s: Failed to parse IRC message: %v", name, err)
		}

		if msg.Command == "PRIVMSG" {
		}

		if msg.Command == "MSG" {
		}

		if logRaw {
			log.Printf("Ghost %s: %v", name, msg)
		}

		_, err = io.WriteString(dest, line)
		if err != nil {
			closer()

			return
		}
	}
}
