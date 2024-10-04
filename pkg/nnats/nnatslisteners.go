package nnats

import (
	"io"
	"net"
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/charlie0129/nnat/pkg/handshake"
)

type empty struct{}

type NNATSListeners struct {
	mu         *sync.RWMutex
	listeners  map[handshake.ConnectionSecretType]net.Listener
	nnatcConns *NNATCConnections
}

func NewNNATSListeners(nnatcConns *NNATCConnections) *NNATSListeners {
	return &NNATSListeners{
		mu:         &sync.RWMutex{},
		listeners:  make(map[handshake.ConnectionSecretType]net.Listener),
		nnatcConns: nnatcConns,
	}
}

func (n *NNATSListeners) ListenIfNotAlready(secret handshake.ConnectionSecretType, network, address string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if _, ok := n.listeners[secret]; ok {
		return nil
	}

	listener, err := net.Listen(network, address)
	if err != nil {
		return errors.Wrap(err, "failed to listen")
	}

	n.listeners[secret] = listener

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"src": listener.Addr(),
				}).Errorf("Failed to accept connection: %v", err)
				continue
			}

			go func() {
				err := n.handleConn(secret, conn)
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"src": conn.RemoteAddr(),
					}).Errorf("Failed to handle connection: %v", err)
				}
			}()
		}
	}()

	return nil
}

func (n *NNATSListeners) handleConn(secret handshake.ConnectionSecretType, conn net.Conn) error {
	defer conn.Close()

	nnatcConn := n.nnatcConns.GetConnection(secret)

	if nnatcConn == nil {
		return errors.New("no nnatc connection found")
	}

	defer nnatcConn.Close()

	logrus.Debugf("Copying data between %v and %v", nnatcConn.RemoteAddr(), conn.RemoteAddr())

	stopCh := make(chan empty, 2)

	go func() {
		defer func() {
			stopCh <- empty{}
		}()
		_, err := io.Copy(nnatcConn, conn)
		log := logrus.WithFields(logrus.Fields{
			"src": conn.RemoteAddr(),
			"dst": nnatcConn.RemoteAddr(),
		})
		if err != nil {
			log.Debugf("Copy stopped: %v", err)
			return
		}
		log.Debug("Copy stopped")
	}()

	go func() {
		defer func() {
			stopCh <- empty{}
		}()
		_, err := io.Copy(conn, nnatcConn)
		log := logrus.WithFields(logrus.Fields{
			"src": nnatcConn.RemoteAddr(),
			"dst": conn.RemoteAddr(),
		})
		if err != nil {
			log.Debugf("Copy stopped: %v", err)
			return
		}
		log.Debug("Copy stopped")
	}()

	<-stopCh

	return nil
}
