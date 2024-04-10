package types

import (
	"errors"

	"github.com/1ykyk/rebro"
	"github.com/1ykyk/rebro/types/keys/ed25519"
)

type signer struct {
	privKey rebro.PrivKey
	pubKey  rebro.PubKey
}

func NewSigner(privKey rebro.PrivKey, pubKey rebro.PubKey) (rebro.Signer, error) {
	if !privKey.PubKey().Equals(pubKey.Bytes()) {
		return nil, errors.New("invalid pubKey received")
	}

	return &signer{
		privKey: privKey,
		pubKey:  pubKey,
	}, nil
}

func (s *signer) ID() []byte {
	return s.pubKey.Bytes()
}

func (s *signer) Sign(msg []byte) (rebro.Signature, error) {
	signature, err := s.privKey.Sign(msg)
	if err != nil {
		return rebro.Signature{}, err
	}

	return rebro.Signature{
		Signer: s.ID(),
		Body:   signature,
	}, nil
}

func (s *signer) Verify(msg []byte, signature rebro.Signature) error {
	// TODO: should avoid a concrete type here
	ok := ed25519.PubKey(signature.Signer).VerifySignature(msg, signature.Body)
	if !ok {
		return errors.New("signature is invalid")
	}
	return nil
}
