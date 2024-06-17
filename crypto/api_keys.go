package crypto

type PubKey interface {
	VerifySignature([]byte, []byte) bool
	Bytes() []byte
	Equals([]byte) bool
	Type() string
}

type PrivKey interface {
	Sign([]byte) ([]byte, error)
	PubKey() PubKey
	Equals([]byte) bool
	Type() string
}
