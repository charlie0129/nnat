package nnats

import (
	"sync"

	"github.com/charlie0129/nnat/pkg/handshake"
)

type SecretPortStorage struct {
	mu *sync.RWMutex
	m  map[handshake.ConnectionSecretType]uint16
}

func NewSecretPortStorage() *SecretPortStorage {
	return &SecretPortStorage{
		mu: &sync.RWMutex{},
		m:  make(map[handshake.ConnectionSecretType]uint16),
	}
}

func (s *SecretPortStorage) Get(secret handshake.ConnectionSecretType) (uint16, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	port, ok := s.m[secret]
	return port, ok
}

func (s *SecretPortStorage) Set(secret handshake.ConnectionSecretType, port uint16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[secret] = port
}

func (s *SecretPortStorage) Delete(secret handshake.ConnectionSecretType) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, secret)
}
