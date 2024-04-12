package types

import (
	"bytes"
	"errors"

	"github.com/1ykyk/rebro"
	"github.com/1ykyk/rebro/types/validator"
)

type commitment struct {
	msg rebro.Message

	signatures []rebro.Signature

	validatorSet     *validator.ValidatorSet
	totalVotingPower int64
}

func NewCommitment(msg rebro.Message, valSet *validator.ValidatorSet) (rebro.Commitment, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	return &commitment{
		msg:          msg,
		signatures:   make([]rebro.Signature, 0, valSet.Len()),
		validatorSet: valSet,
	}, nil
}

func (c *commitment) Message() rebro.Message {
	return c.msg
}

func (c *commitment) Signatures() []rebro.Signature {
	return c.signatures
}

func (c *commitment) AddSignature(s rebro.Signature) (bool, error) {
	validator := c.validatorSet.GetByPubKey(s.Signer)
	if validator == nil {
		return false, errors.New("the signer is not a part of validator set")
	}

	for _, signature := range c.signatures {
		if bytes.Equal(signature.Signer, s.Signer) {
			//TBD: maybe we should not err here.
			return false, errors.New("duplicate signature from the signer")
		}
	}

	if !validator.PubKey.VerifySignature(c.msg.ID.Hash(), s.Body) {
		return false, errors.New("invalid signature")
	}

	c.signatures = append(c.signatures, s)
	c.totalVotingPower += validator.VotingPower
	quorum := c.validatorSet.TotalVotingPower()*2/3 + 1
	return c.totalVotingPower >= quorum, nil
}
