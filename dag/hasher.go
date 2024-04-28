package dag

import (
	"crypto/sha256"

	"github.com/iykyk-syn/unison/rebro"
)

type hasher struct{}

func NewHasher() rebro.Hasher {
	return &hasher{}
}

func (t *hasher) Hash(msg rebro.Message) ([]byte, error) {
	h := sha256.New()
	h.Write(msg.Data)
	return h.Sum(nil), nil
}
