package rebro

import (
	"context"
	"errors"
)

type quorum struct {
	ctx context.Context

	includers *Includers
	round     uint64

	commitments map[string]Commitment
	completed   []Commitment
}

func NewQuorum(ctx context.Context, round uint64, includers *Includers) *quorum {
	return &quorum{
		ctx:         ctx,
		includers:   includers,
		round:       round,
		commitments: make(map[string]Commitment),
		completed:   make([]Commitment, 0),
	}
}

func (q *quorum) Add(msg Message) error {
	if msg.ID.Round() != q.round {
		return errors.New("getting message from another round")
	}

	signer := q.includers.GetByPubKey(msg.ID.Signer())
	if signer == nil {
		return errors.New("signer is not a part of the includers set")
	}

	_, ok := q.commitments[msg.ID.String()]
	if ok {
		return errors.New("commitment exists")
	}

	com, err := NewCommitment(msg, q.includers, q)
	if err != nil {
		return err
	}
	q.commitments[msg.ID.String()] = com
	return err
}

func (q *quorum) Get(id MessageID) (Commitment, bool) {
	com, ok := q.commitments[id.String()]
	return com, ok
}

func (q *quorum) Delete(id MessageID) bool {
	if _, ok := q.commitments[id.String()]; !ok {
		return false
	}
	delete(q.commitments, id.String())
	return true
}

func (q *quorum) List() []Commitment {
	comms := make([]Commitment, 0, len(q.commitments))
	for _, comm := range q.commitments {
		comms = append(comms, comm)
	}
	return comms
}

func (q *quorum) Finalize() (bool, error) {
	return len(q.completed) >= q.includers.Len()*threshold, nil
}

func (q *quorum) markAsCompleted(id string) bool {
	comm, ok := q.commitments[id]
	if !ok {
		return false
	}
	q.completed = append(q.completed, comm)
	delete(q.commitments, id)
	return true
}
