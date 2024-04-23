package stake

import (
	"context"
	"errors"
	"fmt"

	"github.com/iykyk-syn/unison/crypto"
	"github.com/iykyk-syn/unison/rebro"
)

type Quorum struct {
	ctx context.Context

	includers *Includers
	round     uint64

	certificates map[string]rebro.Certificate
	completed    []rebro.Certificate
}

func NewQuorum(ctx context.Context, round uint64, includers *Includers) *Quorum {
	return &Quorum{
		ctx:          ctx,
		includers:    includers,
		round:        round,
		certificates: make(map[string]rebro.Certificate),
		completed:    make([]rebro.Certificate, 0),
	}
}

func (q *Quorum) Add(msg rebro.Message) error {
	if msg.ID.Round() != q.round {
		return errors.New("getting message from another round")
	}

	signer := q.includers.GetByPubKey(msg.ID.Signer())
	if signer == nil {
		return errors.New("signer is not a part of the includers set")
	}

	_, ok := q.certificates[msg.ID.String()]
	if ok {
		return errors.New("Certificate exists")
	}

	cert, err := q.newCertificate(msg)
	if err != nil {
		return err
	}
	q.certificates[msg.ID.String()] = cert
	return err
}

func (q *Quorum) Get(id rebro.MessageID) (rebro.Certificate, bool) {
	com, ok := q.certificates[id.String()]
	return com, ok
}

func (q *Quorum) Delete(id rebro.MessageID) bool {
	if _, ok := q.certificates[id.String()]; !ok {
		return false
	}
	delete(q.certificates, id.String())
	return true
}

func (q *Quorum) List() []rebro.Certificate {
	comms := make([]rebro.Certificate, 0, len(q.certificates))
	for _, comm := range q.certificates {
		comms = append(comms, comm)
	}
	return comms
}

func (q *Quorum) Finalize() (bool, error) {
	totalQuorumPower := int64(0)
	for _, comm := range q.completed {
		// no need to check for nil at this point as we can be sure that signer exists
		signer := q.includers.GetByPubKey(comm.Message().ID.Signer())
		totalQuorumPower = safeAddClip(totalQuorumPower, signer.Stake)
		if totalQuorumPower > MaxStake {
			panic(fmt.Sprintf(
				"Total stake exceeds MaxStake: %v; got: %v",
				MaxStake,
				totalQuorumPower))
		}
	}
	return totalQuorumPower >= q.includers.TotalStake()*int64(stakeThreshold), nil
}

func (q *Quorum) markAsCompleted(id string) bool {
	comm, ok := q.certificates[id]
	if !ok {
		return false
	}
	q.completed = append(q.completed, comm)
	delete(q.certificates, id)
	return true
}

func (q *Quorum) newCertificate(msg rebro.Message) (*Certificate, error) {
	return &Certificate{
		msg:          msg,
		signatures:   make([]crypto.Signature, 0, q.includers.Len()),
		includersSet: q.includers,
		quorum:       q,
	}, nil
}
