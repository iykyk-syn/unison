package quorum

import (
	"github.com/iykyk-syn/unison/crypto"
	"github.com/iykyk-syn/unison/rebro"
)

type certificate struct {
	quorum *Quorum

	msg         rebro.Message
	signatures  []crypto.Signature
	activeStake int64
	completed   bool
}

func (c *certificate) Message() rebro.Message {
	return c.msg
}

func (c *certificate) Signatures() []crypto.Signature {
	return c.signatures
}

func (c *certificate) AddSignature(s crypto.Signature) (bool, error) {
	return c.quorum.addSignature(s, c)
}
