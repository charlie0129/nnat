package nnats

import (
	"net"
	"sync"
)

type connectionPool struct {
	mu    *sync.RWMutex
	conns []net.Conn
}

func newConnectionPool() *connectionPool {
	return &connectionPool{
		mu: &sync.RWMutex{},
	}
}

func (c *connectionPool) add(conn net.Conn) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.conns = append(c.conns, conn)
}

func (c *connectionPool) get() net.Conn {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.conns) == 0 {
		return nil
	}
	conn := c.conns[0]
	c.conns = c.conns[1:]
	return conn
}
