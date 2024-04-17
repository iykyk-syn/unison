package rebro

import (
	"bytes"
	"errors"
)

type commitment struct {
	msg Message

	signatures []Signature

	includersSet *Includers
	totalStake   int64

	complete chan<- string
}

func NewCommitment(msg Message, set *Includers, complete chan<- string) (*commitment, error) {
	if err := msg.Validate(); err != nil {
		return nil, err
	}
	return &commitment{
		msg:          msg,
		signatures:   make([]Signature, 0, set.Len()),
		includersSet: set,
		complete:     complete,
	}, nil
}

func (c *commitment) Message() Message {
	return c.msg
}

func (c *commitment) Signatures() []Signature {
	return c.signatures
}

func (c *commitment) AddSignature(s Signature) (bool, error) {
	includer := c.includersSet.GetByPubKey(s.Signer)
	if includer == nil {
		return false, errors.New("the signer is not a part of includers set")
	}

	for _, signature := range c.signatures {
		if bytes.Equal(signature.Signer, s.Signer) {
			return false, errors.New("duplicate signature from the signer")
		}
	}

	c.signatures = append(c.signatures, s)
	c.totalStake += includer.Stake

	completed := c.totalStake >= minRequiredAmount(c.includersSet.TotalStake())
	if completed {
		c.complete <- c.msg.ID.String()
	}
	return completed, nil
}

func minRequiredAmount(amount int64) int64 {
	return amount*2/3 + 1
}
