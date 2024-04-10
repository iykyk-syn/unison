package types

import (
	"errors"

	"github.com/1ykyk/rebro"
	"github.com/1ykyk/rebro/types/keys"
)

type signer struct {
	privKey keys.PrivKey
	pubKey  keys.PubKey

	pubKeyBuilder func([]byte) (keys.PubKey, error)
}

func NewSigner(privKey keys.PrivKey, pubKey keys.PubKey, bytesToPubKeyFn func([]byte) (keys.PubKey, error)) (rebro.Signer, error) {
	if !privKey.PubKey().Equals(pubKey.Bytes()) {
		return nil, errors.New("invalid pubKey received")
	}
	if bytesToPubKeyFn == nil {
		return nil, errors.New("empty pubKey builder")
	}

	return &signer{
		privKey:       privKey,
		pubKey:        pubKey,
		pubKeyBuilder: bytesToPubKeyFn,
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
	pubK, err := s.pubKeyBuilder(signature.Signer)
	if err != nil {
		return nil
	}

	ok := pubK.VerifySignature(msg, signature.Body)
	if !ok {
		return errors.New("signature is invalid")
	}
	return nil
}
