package handshake

type ClientHello struct {
	ConnectionSecret [16]byte
}

func (c *ClientHello) Serialize() []byte {
	return c.ConnectionSecret[:]
}

func (c *ClientHello) Deserialize(data []byte) {
	copy(c.ConnectionSecret[:], data)
}

func (c *ClientHello) Size() int {
	return 16
}
