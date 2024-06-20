package ed25519

import (
	stdcrypto "crypto"
	"crypto/ed25519"
	"crypto/rand"
	"errors"

	"github.com/iykyk-syn/unison/crypto"
)

const (
	KeyType = "ed25519"
)

type PublicKey []byte

func (pubKey PublicKey) VerifySignature(msg, sig []byte) bool {
	if len(sig) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(pubKey), msg, sig)
}

func (pubKey PublicKey) Equals(other []byte) bool {
	if len(other) != ed25519.PublicKeySize {
		return false
	}
	return ed25519.PublicKey(pubKey).Equal(ed25519.PublicKey(other))
}

func (pubKey PublicKey) Bytes() []byte {
	return pubKey
}

func (pubKey PublicKey) Type() string {
	return KeyType
}

type PrivateKey []byte

func (privKey PrivateKey) Sign(msg []byte) ([]byte, error) {
	return ed25519.PrivateKey(privKey).Sign(rand.Reader, msg, stdcrypto.Hash(0))
}

func (privKey PrivateKey) PubKey() crypto.PubKey {
	public := ed25519.PrivateKey(privKey).Public().(ed25519.PublicKey)
	key := make(PublicKey, ed25519.PublicKeySize)
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

func GenKeys() (PublicKey, PrivateKey, error) {
	pubK, privK, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	public := make(PublicKey, ed25519.PublicKeySize)
	copy(public, pubK)
	private := make(PrivateKey, ed25519.PrivateKeySize)
	copy(private, privK)

	return public, private, nil
}

func BytesToPubKey(b []byte) (PublicKey, error) {
	if len(b) != ed25519.PublicKeySize {
		return nil, errors.New("invalid key length")
	}

	key := make(PublicKey, ed25519.PublicKeySize)
	copy(key, b)
	return key, nil
}
