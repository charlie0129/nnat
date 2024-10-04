package handshake

const (
	ServerHelloCodeOK = iota
	ServerHelloCodeInvalidSecret
)

const (
	ServerHelloSize = 3
)

type ServerHello struct {
	Code       uint8
	ServerPort uint16
}

func (s *ServerHello) Serialize() []byte {
	return append([]byte{s.Code}, uint16ToBytes(s.ServerPort)...)
}

func (s *ServerHello) Deserialize(data []byte) {
	s.Code = data[0]
	s.ServerPort = bytesToUint16(data[1:])
}

func (s *ServerHello) Size() int {
	return 3
}

// This is an over-simplified version that only works for little-endian systems.

func uint16ToBytes(i uint16) []byte {
	return []byte{byte(i >> 8), byte(i)}
}

func bytesToUint16(b []byte) uint16 {
	return uint16(b[0])<<8 | uint16(b[1])
}
