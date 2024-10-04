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
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/charlie0129/nnat/pkg/handshake"
	"github.com/charlie0129/nnat/pkg/nnats"
	"github.com/charlie0129/nnat/pkg/version"
)

var (
	listenAddress  = "localhost:9253"
	log            = logrus.WithField("component", "nnats")
	readBufferSize = 1024
)

var (
	listeners         *nnats.NNATSListeners
	connPool          *nnats.NNATCConnections
	secretPortStorage *nnats.SecretPortStorage
	conf              = secretPortMap{}
)

type secretPortMap map[[16]byte]uint16

func (s secretPortMap) String() string {
	return fmt.Sprintf("%#v", s)
}

func (s secretPortMap) Set(value string) error {
	v := strings.Split(value, ":")
	if len(v) != 2 {
		return errors.New("invalid secret port map")
	}

	// base64 decode connection secret
	connectionSecretBytes, err := base64.StdEncoding.DecodeString(v[0])
	if err != nil {
		return fmt.Errorf("failed to decode connection secret: %v", err)
	}
	if len(connectionSecretBytes) != 16 {
		return fmt.Errorf("invalid connection secret length: %d, must be %d", len(connectionSecretBytes), 16)
	}

	connectionSecret := [16]byte(connectionSecretBytes)
	portInt, err := strconv.Atoi(v[1])
	if err != nil || portInt < 0 || portInt > 65535 {
		return fmt.Errorf("invalid port: %v", err)
	}
	port := uint16(portInt)

	s[connectionSecret] = port
	return nil
}

func init() {
	secretPortStorage = nnats.NewSecretPortStorage()
	connPool = nnats.NewNNATCConnections(secretPortStorage)
	listeners = nnats.NewNNATSListeners(connPool)
}

func main() {
	flag.StringVar(&listenAddress, "listen-address", listenAddress, "Listen address")
	flag.Var(conf, "conf", "Connection secret and port")
	flag.Parse()

	for secret, port := range conf {
		secretPortStorage.Set(secret, port)
	}

	log.Infof("nnats version %s", version.Version)

	logrus.SetLevel(logrus.DebugLevel)

	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	defer listener.Close()

	log.Infof("Listening on %s for nnatc connections", listenAddress)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatalf("Failed to accept connection: %v", err)
		}

		go handleNNATCConnection(conn)
	}
}

func handleNNATCConnection(nnatcConn net.Conn) {
	buf := make([]byte, readBufferSize)
	n, err := nnatcConn.Read(buf)
	if errors.Is(err, io.EOF) {
		log.Infof("Client closed connection")
		return
	}
	if err != nil {
		log.Errorf("Failed to read from nnatc: %v", err)
		return
	}

	clientHello := handshake.ClientHello{}
	clientHello.Deserialize(buf[:n])

	log.Debugf("Received client hello: %v", clientHello)

	serverPort, ok := conf[clientHello.ConnectionSecret]
	if !ok {
		log.Errorf("Unknown connection secret: %v", clientHello.ConnectionSecret)
		serverHello := handshake.ServerHello{
			Code:       handshake.ServerHelloCodeInvalidSecret,
			ServerPort: serverPort,
		}

		_, err = nnatcConn.Write(serverHello.Serialize())
		if err != nil {
			log.Errorf("Failed to write to client: %v", err)
			return
		}
		return
	}

	serverHello := handshake.ServerHello{
		Code:       handshake.ServerHelloCodeOK,
		ServerPort: serverPort,
	}

	_, err = nnatcConn.Write(serverHello.Serialize())
	if err != nil {
		log.Errorf("Failed to write to client: %v", err)
		return
	}

	log.Debugf("Sent server hello: %v", serverHello)

	connPool.AddConnection(clientHello.ConnectionSecret, nnatcConn)

	err = listeners.ListenIfNotAlready(clientHello.ConnectionSecret, "tcp", fmt.Sprintf(":%d", serverPort))
	if err != nil {
		log.Errorf("Failed to listen for nnats: %v", err)
		return
	}
}
