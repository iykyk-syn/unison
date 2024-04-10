package ed25519

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"errors"

	"github.com/1ykyk/rebro/types/keys"
)

const (
	KeyType       = "ed25519"
	TruncatedSize = 20
)

type PubKey []byte

func (pubKey PubKey) Address() (keys.Address, error) {
	if len(pubKey) != ed25519.PublicKeySize {
		return nil, errors.New("invalid key length")
	}
	return keys.Address(pubKey[:TruncatedSize]), nil
}

func (pubKey PubKey) VerifySignature(msg []byte, sig []byte) bool {
	if len(sig) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(pubKey), msg, sig)
}

func (pubKey PubKey) Equals(other []byte) bool {
	if len(other) != ed25519.PublicKeySize {
		return false
	}
	return ed25519.PublicKey(pubKey).Equal(other)
}

func (pubKey PubKey) Bytes() []byte {
	return pubKey
}

func (pubKey PubKey) Type() string {
	return KeyType
}

type PrivateKey []byte

func (privKey PrivateKey) Sign(msg []byte) ([]byte, error) {
	return ed25519.PrivateKey(privKey).Sign(rand.Reader, msg, crypto.SHA256)
}

func (privKey PrivateKey) PubKey() keys.PubKey {
	public := ed25519.PrivateKey(privKey).Public().(ed25519.PublicKey)
	key := make(PubKey, ed25519.PublicKeySize)
	copy(key, public)
	return key
}

func (privKey PrivateKey) Equals(other []byte) bool {
	if len(other) != ed25519.PrivateKeySize {
		return false
	}
	return ed25519.PrivateKey(privKey).Equal(other)
}

func (privKey PrivateKey) Type() string {
	return KeyType
}

func GenKeys() (PubKey, PrivateKey, error) {
	pubK, privK, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	public := make(PubKey, ed25519.PublicKeySize)
	copy(public, pubK)
	private := make(PrivateKey, ed25519.PrivateKeySize)
	copy(private, privK)

	return public, private, nil
}

func BytesToPubKey(b []byte) (PubKey, error) {
	if len(b) != ed25519.PublicKeySize {
		return nil, errors.New("invalid key length")
	}

	key := make(PubKey, ed25519.PublicKeySize)
	copy(key, b)
	return key, nil
}
