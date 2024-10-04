package handshake

type ConnectionSecretType [16]byte

type ClientHello struct {
	ConnectionSecret ConnectionSecretType
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
