package keys

type Address []byte

type PubKey interface {
	Address() (Address, error)
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
