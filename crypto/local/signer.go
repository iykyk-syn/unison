package local

import (
	"errors"

	"github.com/iykyk-syn/unison/crypto"
	"github.com/iykyk-syn/unison/crypto/ed25519"
)

type Signer struct {
	privKey crypto.PrivKey
	pubKey  crypto.PubKey
}

func NewSigner(privKey crypto.PrivKey) (*Signer, error) {
	pubKey := privKey.PubKey()
	if !privKey.PubKey().Equals(pubKey.Bytes()) {
		return nil, errors.New("invalid pubKey received")
	}

	return &Signer{
		privKey: privKey,
		pubKey:  pubKey,
	}, nil
}

func (s *Signer) ID() []byte {
	return s.pubKey.Bytes()
}

func (s *Signer) Sign(msg []byte) (crypto.Signature, error) {
	signature, err := s.privKey.Sign(msg)
	if err != nil {
		return crypto.Signature{}, err
	}

	return crypto.Signature{
		Signer: s.ID(),
		Body:   signature,
	}, nil
}

func (s *Signer) Verify(msg []byte, signature crypto.Signature) error {
	pubK := ed25519.PublicKey(signature.Signer)
	ok := pubK.VerifySignature(msg, signature.Body)
	if !ok {
		return errors.New("signature is invalid")
	}
	return nil
}
