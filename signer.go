package rebro

import (
	"errors"

	"github.com/1ykyk/rebro/crypto"
	"github.com/1ykyk/rebro/crypto/ed25519"
)

type localSigner struct {
	privKey crypto.PrivKey
	pubKey  crypto.PubKey
}

func NewSigner(privKey crypto.PrivKey) (*localSigner, error) {
	pubKey := privKey.PubKey()
	if !privKey.PubKey().Equals(pubKey.Bytes()) {
		return nil, errors.New("invalid pubKey received")
	}

	return &localSigner{
		privKey: privKey,
		pubKey:  pubKey,
	}, nil
}

func (s *localSigner) ID() []byte {
	return s.pubKey.Bytes()
}

func (s *localSigner) Sign(msg []byte) (Signature, error) {
	signature, err := s.privKey.Sign(msg)
	if err != nil {
		return Signature{}, err
	}

	return Signature{
		Signer: s.ID(),
		Body:   signature,
	}, nil
}

func (s *localSigner) Verify(msg []byte, signature Signature) error {
	pubK := ed25519.PublicKey(signature.Signer)
	ok := pubK.VerifySignature(msg, signature.Body)
	if !ok {
		return errors.New("signature is invalid")
	}
	return nil
}
