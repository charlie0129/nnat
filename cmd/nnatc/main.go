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
	"flag"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/charlie0129/nnat/pkg/handshake"
	"github.com/charlie0129/nnat/pkg/version"
)

var (
	connPoolSize           = 10
	serverAddress          = "localhost:9253"
	destinationAddress     = "localhost:8080"
	connectionSecret       [16]byte
	connectionSecretString string
	log                    = logrus.WithField("component", "nnatc")
)

func main() {
	flag.IntVar(&connPoolSize, "conn-pool-size", connPoolSize, "Connection pool size")
	flag.StringVar(&serverAddress, "server-address", serverAddress, "Server address")
	flag.StringVar(&destinationAddress, "destination-address", destinationAddress, "Destination address")
	flag.StringVar(&connectionSecretString, "connection-secret", "", "Connection secret")
	flag.Parse()

	if connectionSecretString == "" {
		log.Fatalf("Connection secret is required")
	}

	// base64 decode connection secret
	connectionSecretBytes, err := base64.StdEncoding.DecodeString(connectionSecretString)
	if err != nil {
		log.Fatalf("Failed to decode connection secret: %v", err)
	}
	if len(connectionSecretBytes) != len(connectionSecret) {
		log.Fatalf("Invalid connection secret length: %d, must be %d", len(connectionSecretBytes), len(connectionSecret))
	}
	connectionSecret = [16]byte(connectionSecretBytes)

	log.Infof("nnatc version %s", version.Version)

	logrus.SetLevel(logrus.DebugLevel)

	pool := newConnectionPool()
	pool.maintain()
}

type connectionPool struct {
	cond               *sync.Cond
	currentConnections atomic.Int32
}

func newConnectionPool() *connectionPool {
	return &connectionPool{
		cond: sync.NewCond(&sync.Mutex{}),
	}
}

func (c *connectionPool) maintain() {
	for {
		if c.currentConnections.Load() < int32(connPoolSize) {
			log.WithFields(logrus.Fields{
				"current": c.currentConnections.Load(),
				"max":     connPoolSize,
			}).Infof("Creating new connection")

			c.cond.L.Lock()

			nnatsConn, err := net.Dial("tcp", serverAddress)
			if err != nil {
				c.cond.L.Unlock()
				log.Fatalf("Failed to connect to server: %v", err)
			}

			if !c.handshake(nnatsConn) {
				nnatsConn.Close()
				// wait for a bit before trying again
				time.Sleep(5 * time.Second)
				c.cond.L.Unlock()
				continue
			}

			c.currentConnections.Add(1)
			c.cond.L.Unlock()
			go c.newConnection(nnatsConn)
		} else {
			c.cond.L.Lock()
			c.cond.Wait()
			c.cond.L.Unlock()
		}
	}
}

func (c *connectionPool) handshake(nnatsConn net.Conn) bool {
	clientHello := handshake.ClientHello{
		ConnectionSecret: connectionSecret,
	}
	_, err := nnatsConn.Write(clientHello.Serialize())
	if err != nil {
		log.Errorf("Failed to write to server: %v", err)
		return false
	}

	serverHello := handshake.ServerHello{}
	buf := make([]byte, 1024)
	n, err := nnatsConn.Read(buf)
	if err != nil {
		log.Errorf("Failed to read from server: %v", err)
		return false
	}

	if n != handshake.ServerHelloSize {
		log.Errorf("Invalid server hello size: %d", n)
		return false
	}

	serverHello.Deserialize(buf[:n])

	log.Infof("Received server hello: %v", serverHello)

	if serverHello.Code != handshake.ServerHelloCodeOK {
		log.Errorf("Server rejected connection: %v", serverHello)
		return false
	}

	return true
}

func (c *connectionPool) newConnection(nnatsConn net.Conn) {
	defer nnatsConn.Close()

	var dstConn net.Conn

	buf := make([]byte, 10240)

	// wait for first message from server
	log.Debugf("Waiting for first message from server")
	n, err := nnatsConn.Read(buf)
	if errors.Is(err, io.EOF) {
		c.currentConnections.Add(-1)
		c.cond.Broadcast()
		log.Infof("Connection closed by server")
		return
	}
	if err != nil {
		c.currentConnections.Add(-1)
		c.cond.Broadcast()
		log.Errorf("Failed to read from server: %v", err)
		return
	}
	c.currentConnections.Add(-1)
	c.cond.Broadcast()

	dstConn, err = net.Dial("tcp", destinationAddress)
	if err != nil {
		log.Errorf("Failed to connect to destination: %v", err)
		return
	}
	defer dstConn.Close()
	_, err = dstConn.Write(buf[:n])
	if err != nil {
		log.Errorf("Failed to write to destination: %v", err)
		return
	}

	// start copying data between connections
	log.Debugf("Copying data between %v and %v", nnatsConn.RemoteAddr(), dstConn.RemoteAddr())
	stopCh := make(chan empty, 2)

	go func() {
		defer func() {
			stopCh <- empty{}
		}()
		_, err := io.Copy(dstConn, nnatsConn)
		log := log.WithFields(logrus.Fields{
			"src": nnatsConn.RemoteAddr(),
			"dst": dstConn.RemoteAddr(),
		})
		if err != nil {
			log.Debugf("Copy stopped: %v", err)
			return
		}
		log.Debugf("Copy stopped")
	}()

	go func() {
		defer func() {
			stopCh <- empty{}
		}()
		_, err := io.Copy(nnatsConn, dstConn)
		log := log.WithFields(logrus.Fields{
			"src": dstConn.RemoteAddr(),
			"dst": nnatsConn.RemoteAddr(),
		})
		if err != nil {
			log.Debugf("Copy stopped: %v", err)
			return
		}
		log.Debugf("Copy stopped")
	}()

	<-stopCh
}

type empty struct{}
