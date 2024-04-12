package keys

type Address []byte

const (
	AddressSize = 20
)

type PubKey interface {
	Address() Address
	VerifySignature(msg []byte, sig []byte) bool
	Bytes() []byte
	Equals([]byte) bool
	Type() string
}

type PrivKey interface {
	Sign(msg []byte) ([]byte, error)
	PubKey() PubKey
	Equals([]byte) bool
	Type() string
}
