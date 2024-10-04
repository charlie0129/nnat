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
	"errors"
	"io"
	"net"

	"github.com/sirupsen/logrus"

	"github.com/charlie0129/template-go/pkg/handshake"
	"github.com/charlie0129/template-go/pkg/version"
)

var (
	connPoolSize       = 10
	readBufferSize     = 1024
	serverAddress      = "localhost:9253"
	destinationAddress = "localhost:8080"
	log                = logrus.WithField("component", "nnatc")
)

func main() {
	log.Infof("nnatc version %s", version.Version)

	conn, err := net.Dial("tcp", serverAddress)
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	clientHello := handshake.ClientHello{
		ConnectionSecret: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
	}
	_, err = conn.Write(clientHello.Serialize())
	if err != nil {
		log.Fatalf("Failed to write to server: %v", err)
	}

	serverHello := handshake.ServerHello{}
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		log.Fatalf("Failed to read from server: %v", err)
	}
	serverHello.Deserialize(buf[:n])

	log.Infof("Received server hello: %v", serverHello)

	if serverHello.Code != handshake.ServerHelloCodeOK {
		log.Fatalf("Server rejected connection: %v", serverHello)
	}

	var destinationConn *net.Conn

	for {
		n, err = conn.Read(buf)
		if errors.Is(err, io.EOF) {
			log.Infof("Connection closed by server")
			if destinationConn != nil {
				(*destinationConn).Close()
			}
			break
		}
		if err != nil {
			log.Errorf("Failed to read from server: %v", err)
			continue
		}
		if destinationConn == nil {
			dstConn, err := net.Dial("tcp", destinationAddress)
			if err != nil {
				log.Errorf("Failed to connect to destination: %v", err)
				dstConn.Close()
				break
			}
			destinationConn = &dstConn

			go func() {
				buf := make([]byte, readBufferSize)
				for {
					n, err := dstConn.Read(buf)
					if err != nil {
						log.Errorf("Failed to read from destination: %v", err)
						dstConn.Close()
						break
					}
					_, err = conn.Write(buf[:n])
					if err != nil {
						log.Errorf("Failed to write to server: %v", err)
						dstConn.Close()
						break
					}
				}
			}()
		}
		dstConn := *destinationConn
		_, err = dstConn.Write(buf[:n])
		if err != nil {
			log.Errorf("Failed to write to destination: %v", err)
			dstConn.Close()
			break
		}
	}

}
