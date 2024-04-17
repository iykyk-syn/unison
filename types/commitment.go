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

	includersSet     *validator.Includers
	totalVotingPower int64
}

func NewCommitment(msg rebro.Message, set *validator.Includers) (*commitment, error) {
	if err := msg.Validate(); err != nil {
		return nil, err
	}
	return &commitment{
		msg:          msg,
		signatures:   make([]rebro.Signature, 0, set.Len()),
		includersSet: set,
	}, nil
}

func (c *commitment) Message() rebro.Message {
	return c.msg
}

func (c *commitment) Signatures() []rebro.Signature {
	return c.signatures
}

func (c *commitment) AddSignature(s rebro.Signature) (bool, error) {
	validator := c.includersSet.GetByPubKey(s.Signer)
	if validator == nil {
		return false, errors.New("the signer is not a part of validator set")
	}

	for _, signature := range c.signatures {
		if bytes.Equal(signature.Signer, s.Signer) {
			return false, errors.New("duplicate signature from the signer")
		}
	}

	c.signatures = append(c.signatures, s)
	c.totalVotingPower += validator.Stake
	quorum := minRequiredStake(c.includersSet.TotalStake())
	return c.totalVotingPower >= quorum, nil
}

func minRequiredStake(stake int64) int64 {
	return stake*2/3 + 1
}
