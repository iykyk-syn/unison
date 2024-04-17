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

	completeCh chan string
}

func NewQuorum(ctx context.Context, r uint64, includers *Includers) *quorum {
	return &quorum{
		ctx:         ctx,
		includers:   includers,
		round:       r,
		commitments: make(map[string]Commitment),
		completed:   make([]Commitment, 0),
		completeCh:  make(chan string, includers.Len()), // buffered channel to make non-blocking write into it
	}
}

func (q *quorum) Add(msg Message) error {
	if err := msg.Validate(); err != nil {
		return err
	}

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

	com, err := NewCommitment(msg, q.includers, q.completeCh)
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
	minAmount := int(minRequiredAmount(int64(q.includers.Len())))
	// TBD: lets wait 2/3+1 for now and then decide what should we do with other 1/3
	for i := 0; i < minAmount; i++ {
		select {
		case <-q.ctx.Done():
			return false, q.ctx.Err()
		case id := <-q.completeCh:
			if !q.markAsCompleted(id) {
				// got one more confirmation that commitment was completed
				i--
			}
		}
	}
	// TODO: aggregate signatures
	return true, nil
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
