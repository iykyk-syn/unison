package types

import (
	"errors"

	"github.com/1ykyk/rebro"
	"github.com/1ykyk/rebro/types/keys"
	"github.com/1ykyk/rebro/types/keys/ed25519"
)

type localSigner struct {
	privKey keys.PrivKey
	pubKey  keys.PubKey
}

func NewSigner(privKey keys.PrivKey) (*localSigner, error) {
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

func (s *localSigner) Sign(msg []byte) (rebro.Signature, error) {
	signature, err := s.privKey.Sign(msg)
	if err != nil {
		return rebro.Signature{}, err
	}

	return rebro.Signature{
		Signer: s.ID(),
		Body:   signature,
	}, nil
}

func (s *localSigner) Verify(msg []byte, signature rebro.Signature) error {
	pubK := ed25519.PublicKey(signature.Signer)
	ok := pubK.VerifySignature(msg, signature.Body)
	if !ok {
		return errors.New("signature is invalid")
	}
	return nil
}
