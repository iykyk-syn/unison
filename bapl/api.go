// Package bapl implements rudimentary implementation of BatchPool
package bapl

import (
	"context"
	"crypto/sha256"

	"github.com/iykyk-syn/unison/crypto"
)

type Batch struct {
	Data      []byte
	Signature crypto.Signature
	Included  bool
}

func (b *Batch) Hash() []byte {
	h := sha256.New()
	h.Write(b.Data)
	return h.Sum(nil)
}

type BatchPool interface {
	Push(context.Context, *Batch) error
	Pull(context.Context, []byte) (*Batch, error)
	ListBySigner(context.Context, []byte) ([]*Batch, error)
	Delete(context.Context, []byte) error
	Size(context.Context) (int, error)
}

type BatchVerifier interface {
	Verify(context.Context, *Batch) (bool, error)
}
