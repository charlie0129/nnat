/*
Copyright 2023 Charlie Chiang

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"net"

	"github.com/sirupsen/logrus"

	"github.com/charlie0129/template-go/pkg/handshake"
	"github.com/charlie0129/template-go/pkg/version"
)

var (
	listenAddress = "localhost:9253"
	log           = logrus.WithField("component", "nnats")
)

var (
	secretPortMap = map[[16]byte]uint16{
		[16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}: 18080,
	}
)

func main() {
	log.Infof("nnats version %s", version.Version)

	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	defer listener.Close()

	log.Infof("Listening on %s", listenAddress)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatalf("Failed to accept connection: %v", err)
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		log.Errorf("Failed to read from client: %v", err)
		return
	}

	clientHello := handshake.ClientHello{}
	clientHello.Deserialize(buf[:n])

	log.Infof("Received client hello: %v", clientHello)

	serverPort, ok := secretPortMap[clientHello.ConnectionSecret]
	if !ok {
		log.Errorf("Unknown connection secret: %v", clientHello.ConnectionSecret)
		return
	}

	serverHello := handshake.ServerHello{
		Code:       handshake.ServerHelloCodeOK,
		ServerPort: serverPort,
	}

	_, err = conn.Write(serverHello.Serialize())
	if err != nil {
		log.Errorf("Failed to write to client: %v", err)
		return
	}

	log.Infof("Sent server hello: %v", serverHello)

	// listen on serverPort and forward traffic to conn
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", serverPort))
	if err != nil {
		log.Errorf("Failed to listen on port %d: %v", serverPort, err)
		return
	}

	log.Infof("Listening on port %d", serverPort)

	for {
		serverConn, err := listener.Accept()
		if err != nil {
			log.Errorf("Failed to accept connection: %v", err)
			continue
		}

		go func() {
			defer serverConn.Close()

			for {
				n, err := serverConn.Read(buf)
				if err != nil { // Handle EOF
					log.Errorf("Failed to read from server: %v", err)
					break
				}

				_, err = conn.Write(buf[:n])
				if err != nil {
					log.Errorf("Failed to write to client: %v", err)
					break
				}
			}
		}()

		go func() {
			defer conn.Close()

			for {
				n, err := conn.Read(buf)
				if err != nil { // Handle EOF
					log.Errorf("Failed to read from client: %v", err)
					break
				}

				_, err = serverConn.Write(buf[:n])
				if err != nil {
					log.Errorf("Failed to write to server: %v", err)
					break
				}
			}
		}()
	}
}
