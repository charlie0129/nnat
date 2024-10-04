package nnats

import (
	"net"
	"sync"

	"github.com/charlie0129/nnat/pkg/handshake"
)

type NNATCConnections struct {
	mu         *sync.RWMutex
	nnatcConns map[handshake.ConnectionSecretType]*connectionPool
}

func NewNNATCConnections(secrets *SecretPortStorage) *NNATCConnections {
	return &NNATCConnections{
		mu:         &sync.RWMutex{},
		nnatcConns: make(map[handshake.ConnectionSecretType]*connectionPool),
	}
}

func (n *NNATCConnections) AddConnection(secret handshake.ConnectionSecretType, conn net.Conn) {
	n.mu.Lock()
	defer n.mu.Unlock()

	pool, ok := n.nnatcConns[secret]
	if !ok {
		pool = newConnectionPool()
		n.nnatcConns[secret] = pool
	}

	pool.add(conn)
}

func (n *NNATCConnections) GetConnection(secret handshake.ConnectionSecretType) net.Conn {
	n.mu.RLock()
	defer n.mu.RUnlock()

	pool, ok := n.nnatcConns[secret]
	if !ok {
		return nil
	}

	return pool.get()
}
