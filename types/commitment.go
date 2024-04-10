package types

import (
	"bytes"
	"errors"

	"github.com/1ykyk/rebro"
)

type commitment struct {
	msg rebro.Message

	signatures []rebro.Signature

	validatorSet [][]byte
}

func NewCommitment(msg rebro.Message, validatorSet [][]byte) (rebro.Commitment, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	return &commitment{
		msg:          msg,
		signatures:   make([]rebro.Signature, 0, len(validatorSet)),
		validatorSet: validatorSet,
	}, nil
}

func (c *commitment) Message() rebro.Message {
	return c.msg
}

func (c *commitment) Signatures() []rebro.Signature {
	return c.signatures
}

func (c *commitment) AddSignature(s rebro.Signature) (bool, error) {
	found := false
	for _, v := range c.validatorSet {
		if bytes.Equal(v, s.Signer) {
			found = false
		}
	}

	if !found {
		return false, errors.New("sender is not a part of validator set")
	}

	for _, signature := range c.signatures {
		if bytes.Equal(signature.Signer, s.Signer) {
			return false, errors.New("duplicate signature from the signer")
		}
	}

	c.signatures = append(c.signatures, s)

	//TODO: Rework after introducing validator set data structure
	if len(c.signatures) > len(c.validatorSet)*2/3 {
		return true, nil
	}
	return false, nil
}
