package handshake

const (
	ClientHelloSize = 16
)

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
