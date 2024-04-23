package stake

import (
	"bytes"
	"errors"

	"github.com/iykyk-syn/unison/crypto"
	"github.com/iykyk-syn/unison/rebro"
)

var (
	faultParameter = 1 / 3
	// threshold is a finalization rule for either a single Certificate inside the Quorum
	// or the Quorum itself.
	stakeThreshold = 2*faultParameter + 1
)

type Certificate struct {
	msg rebro.Message

	signatures []crypto.Signature

	includersSet *Includers
	totalStake   int64

	quorum *Quorum
}

func (c *Certificate) Message() rebro.Message {
	return c.msg
}

func (c *Certificate) Signatures() []crypto.Signature {
	return c.signatures
}

func (c *Certificate) AddSignature(s crypto.Signature) (bool, error) {
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

	completed := c.totalStake >= c.includersSet.TotalStake()*int64(stakeThreshold)
	if completed {
		c.quorum.markAsCompleted(c.msg.ID.String())
	}
	return completed, nil
}
