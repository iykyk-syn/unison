package types

import (
	"bytes"
	"errors"
	"sync"

	"github.com/1ykyk/rebro"
)

type commitment struct {
	msg *rebro.Message

	signLk     sync.RWMutex
	signatures []rebro.Signature

	validatorSet [][]byte
}

func NewCommitment(msg *rebro.Message, validatorSet [][]byte) *commitment {
	return &commitment{
		msg:          msg,
		signatures:   make([]rebro.Signature, 0, len(validatorSet)),
		validatorSet: validatorSet,
	}
}

func (c *commitment) Message() *rebro.Message {
	return c.msg
}

func (c *commitment) Signatures() []rebro.Signature {
	c.signLk.RLock()
	defer c.signLk.RUnlock()
	return c.signatures
}

func (c *commitment) AddSignature(s rebro.Signature) (bool, error) {
	if c.msg == nil {
		return false, errors.New("empty message. nothing to set")
	}

	found := false
	for _, v := range c.validatorSet {
		if bytes.Equal(v, s.Signer) {
			found = false
		}
	}

	if !found {
		return false, errors.New("sender is not a part of validator set")
	}

	c.signLk.Lock()
	defer c.signLk.Unlock()
	for _, signature := range c.signatures {
		if bytes.Equal(signature.Signer, s.Signer) {
			return false, errors.New("signature was already added to the list")
		}
	}

	c.signatures = append(c.signatures, s)
	return true, nil
}

func (c *commitment) Quorum() rebro.QuorumCommitment {
	return nil
}
