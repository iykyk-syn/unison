package round

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/1ykyk/rebro"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestRound(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	quorum := newQuorum()
	r := NewRound(0, quorum)
	id := &messageId{id: "msgid"}

	// check all the basic crud operations work
	err := r.AddCommitment(ctx, rebro.Message{ID: id})
	require.NoError(t, err)
	comm, err := r.GetCommitment(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, comm)
	err = r.AddSignature(ctx, id, rebro.Signature{})
	require.NoError(t, err)
	err = r.DeleteCommitment(ctx, id)
	require.NoError(t, err)
	err = r.AddSignature(ctx, id, rebro.Signature{})
	require.Error(t, err)

	// terminate the round
	err = r.Stop(ctx)
	require.NoError(t, err)

	// ensure we get errors after stopping
	err = r.AddSignature(ctx, id, rebro.Signature{})
	require.Error(t, errClosedRound)
	err = r.Stop(ctx)
	require.Error(t, errClosedRound)
}

func TestRoundSubscription(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	quorum := newQuorum()
	r := NewRound(0, quorum)
	id := &messageId{id: "msgid"}

	// subscribe for commitment
	commCh, errCh := make(chan rebro.Commitment), make(chan error)
	go func() {
		comm, err := r.GetCommitment(ctx, id)
		commCh <- comm
		errCh <- err
	}()
	go func() {
		comm, err := r.GetCommitment(ctx, id)
		commCh <- comm
		errCh <- err
	}()

	// ensure subscription cancellation works
	getCtx, cancel := context.WithTimeout(ctx, time.Millisecond*10)
	comm, err := r.GetCommitment(getCtx, &messageId{id: "msgid2"})
	require.Error(t, err)
	require.Nil(t, comm)
	cancel()

	// ensure subscriptions work
	err = r.AddCommitment(ctx, rebro.Message{ID: id})
	require.NoError(t, err)
	comm, err = <-commCh, <-errCh
	require.NoError(t, err)
	require.NotNil(t, comm)
	comm, err = <-commCh, <-errCh
	require.NoError(t, err)
	require.NotNil(t, comm)

	// ensure all the subscriptions are cleaned
	assert.Len(t, r.getOpSubs, 0)
}

func TestRoundGracefulShutdown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	quorum := newQuorum()
	r := NewRound(0, quorum)

	for i := 0; i < 10; i++ {
		err := r.execOpAsync(ctx, &stateOp{
			msg:    &rebro.Message{ID: &messageId{id: strconv.Itoa(i)}},
			doneCh: make(chan any, 1),
		})
		if err != nil {
			t.Log(err)
		}
	}

	err := r.Stop(ctx)
	require.NoError(t, err)
	assert.Len(t, quorum.List(), 10)
}

func TestRoundConcurrentAccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	quorum := newQuorum()
	r := NewRound(0, quorum)

	wg := errgroup.Group{}
	for i := 0; i < 100; i++ {
		id := &messageId{id: strconv.Itoa(i)}
		wg.Go(func() error {
			commCh, errCh := make(chan rebro.Commitment), make(chan error)
			go func() {
				comm, err := r.GetCommitment(ctx, id)
				commCh <- comm
				errCh <- err
			}()

			err := r.AddCommitment(ctx, rebro.Message{ID: id})
			if err != nil {
				return err
			}

			err = r.AddSignature(ctx, id, rebro.Signature{})
			if err != nil {
				return err
			}

			_, err = r.GetCommitment(ctx, id)
			if err != nil {
				return err
			}

			_, err = <-commCh, <-errCh
			if err != nil {
				return err
			}

			err = r.DeleteCommitment(ctx, id)
			if err != nil {
				return err
			}

			return nil
		})
	}

	err := wg.Wait()
	assert.NoError(t, err)

	err = r.Stop(ctx)
	require.NoError(t, err)
	assert.Len(t, quorum.List(), 0)
	assert.Len(t, r.getOpSubs, 0)
}

func TestRoundFinalize(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	id := &messageId{id: "msgid"}
	quorum := newQuorum()
	r := NewRound(0, quorum)

	go func() {
		err := r.AddCommitment(ctx, rebro.Message{ID: id})
		if err != nil {
			t.Error(err)
			return
		}

		err = r.AddSignature(ctx, id, rebro.Signature{})
		if err != nil {
			t.Error(err)
			return
		}
	}()

	// must not finalize
	finCtx, cancel := context.WithTimeout(context.Background(), time.Millisecond*10)
	err := r.Finalize(finCtx)
	assert.Error(t, err)
	cancel()

	// but after one more singature
	err = r.AddSignature(ctx, id, rebro.Signature{})
	if err != nil {
		t.Error(err)
		return
	}
	// it should
	err = r.Finalize(ctx)
	assert.NoError(t, err)

	ok, err := quorum.Finalize()
	assert.NoError(t, err)
	assert.True(t, ok)
}

type messageId struct {
	round uint64
	id    string
}

func (m *messageId) Round() uint64 {
	return m.round
}

func (m *messageId) Signer() []byte {
	return nil
}

func (m *messageId) Hash() []byte {
	return nil
}

func (m *messageId) String() string {
	return m.id
}

func (m *messageId) New() rebro.MessageID {
	return &messageId{}
}

func (m *messageId) MarshalBinary() ([]byte, error) {
	// TODO implement me
	panic("implement me")
}

func (m *messageId) UnmarshalBinary(bytes []byte) error {
	// TODO implement me
	panic("implement me")
}

type quorum struct {
	comms map[string]*commitment
}

func newQuorum() *quorum {
	return &quorum{
		comms: map[string]*commitment{},
	}
}

func (q *quorum) Add(msg rebro.Message) error {
	q.comms[msg.ID.String()] = &commitment{
		q:   q,
		msg: msg,
	}
	return nil
}

func (q *quorum) Get(id rebro.MessageID) (rebro.Commitment, bool) {
	comm, ok := q.comms[id.String()]
	return comm, ok
}

func (q *quorum) Delete(id rebro.MessageID) bool {
	_, ok := q.comms[id.String()]
	delete(q.comms, id.String())
	return ok
}

func (q *quorum) List() []rebro.Commitment {
	list := make([]rebro.Commitment, 0, len(q.comms))
	for _, comm := range q.comms {
		list = append(list, comm)
	}

	return list
}

func (q *quorum) Finalize() (bool, error) {
	return len(q.comms) == 1, nil
}

type commitment struct {
	q    *quorum
	msg  rebro.Message
	sigs []rebro.Signature
}

func (c *commitment) Message() rebro.Message {
	return c.msg
}

func (c *commitment) Signatures() []rebro.Signature {
	return c.sigs
}

func (c *commitment) AddSignature(sig rebro.Signature) (bool, error) {
	c.sigs = append(c.sigs, sig)
	return len(c.sigs) == 1, nil
}

func (c *commitment) Quorum() rebro.QuorumCommitment {
	return c.q
}
