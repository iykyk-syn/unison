package round

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/iykyk-syn/unison/crypto"
	"github.com/iykyk-syn/unison/rebro"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestRound(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*100)
	defer cancel()

	quorum := newQuorum()
	r := NewRound(0, quorum)
	id := &messageID{id: "msgid"}

	// check all the basic crud operations work
	err := r.AddCertificate(ctx, rebro.Message{ID: id})
	require.NoError(t, err)
	comm, err := r.GetCertificate(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, comm)
	err = r.AddSignature(ctx, id, crypto.Signature{})
	require.NoError(t, err)
	err = r.DeleteCertificate(ctx, id)
	require.NoError(t, err)
	err = r.AddSignature(ctx, id, crypto.Signature{})
	require.Error(t, err)

	// terminate the round
	err = r.Stop(ctx)
	require.NoError(t, err)

	// ensure we get errors after stopping
	err = r.AddSignature(ctx, id, crypto.Signature{})

	require.ErrorAs(t, err, ErrClosedRound)
	err = r.Stop(ctx)
	require.ErrorAs(t, err, ErrClosedRound)
}

func TestRoundSubscription(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	quorum := newQuorum()
	r := NewRound(0, quorum)
	id := &messageID{id: "msgid"}

	// subscribe for certificate
	commCh, errCh := make(chan rebro.Certificate), make(chan error)
	go func() {
		comm, err := r.GetCertificate(ctx, id)
		commCh <- comm
		errCh <- err
	}()
	go func() {
		comm, err := r.GetCertificate(ctx, id)
		commCh <- comm
		errCh <- err
	}()

	// ensure subscription cancellation works
	getCtx, cancel := context.WithTimeout(ctx, time.Millisecond*10)
	comm, err := r.GetCertificate(getCtx, &messageID{id: "msgid2"})
	require.Error(t, err)
	require.Nil(t, comm)
	cancel()

	// ensure subscriptions work
	err = r.AddCertificate(ctx, rebro.Message{ID: id})
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
			msg:    &rebro.Message{ID: &messageID{id: strconv.Itoa(i)}},
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
		id := &messageID{id: strconv.Itoa(i)}
		wg.Go(func() error {
			commCh, errCh := make(chan rebro.Certificate), make(chan error)
			go func() {
				comm, err := r.GetCertificate(ctx, id)
				commCh <- comm
				errCh <- err
			}()

			err := r.AddCertificate(ctx, rebro.Message{ID: id})
			if err != nil {
				return err
			}

			err = r.AddSignature(ctx, id, crypto.Signature{})
			if err != nil {
				return err
			}

			_, err = r.GetCertificate(ctx, id)
			if err != nil {
				return err
			}

			_, err = <-commCh, <-errCh
			if err != nil {
				return err
			}

			err = r.DeleteCertificate(ctx, id)
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

	id := &messageID{id: "msgid"}
	quorum := newQuorum()
	r := NewRound(0, quorum)

	go func() {
		err := r.AddCertificate(ctx, rebro.Message{ID: id})
		if err != nil {
			t.Error(err)
			return
		}

		err = r.AddSignature(ctx, id, crypto.Signature{})
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

	// but after one more signature
	err = r.AddSignature(ctx, id, crypto.Signature{})
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

type messageID struct {
	round uint64
	id    string
}

func (m *messageID) Round() uint64 {
	return m.round
}

func (m *messageID) Signer() []byte {
	return nil
}

func (m *messageID) Hash() []byte {
	return nil
}

func (m *messageID) String() string {
	return m.id
}

func (m *messageID) New() rebro.MessageID {
	return &messageID{}
}

func (m *messageID) Validate() error { return nil }

func (m *messageID) MarshalBinary() ([]byte, error) {
	// TODO implement me
	panic("implement me")
}

func (m *messageID) UnmarshalBinary(bytes []byte) error {
	// TODO implement me
	panic("implement me")
}

type quorum struct {
	comms map[string]*certificate
}

func newQuorum() *quorum {
	return &quorum{
		comms: map[string]*certificate{},
	}
}

func (q *quorum) Add(msg rebro.Message) error {
	q.comms[msg.ID.String()] = &certificate{
		q:   q,
		msg: msg,
	}
	return nil
}

func (q *quorum) Get(id rebro.MessageID) (rebro.Certificate, bool) {
	comm, ok := q.comms[id.String()]
	return comm, ok
}

func (q *quorum) Delete(id rebro.MessageID) bool {
	_, ok := q.comms[id.String()]
	delete(q.comms, id.String())
	return ok
}

func (q *quorum) List() []rebro.Certificate {
	list := make([]rebro.Certificate, 0, len(q.comms))
	for _, comm := range q.comms {
		list = append(list, comm)
	}

	return list
}

func (q *quorum) Finalize() (bool, error) {
	return len(q.comms) == 1, nil
}

type certificate struct {
	q    *quorum
	msg  rebro.Message
	sigs []crypto.Signature
}

func (c *certificate) Message() rebro.Message {
	return c.msg
}

func (c *certificate) Signatures() []crypto.Signature {
	return c.sigs
}

func (c *certificate) AddSignature(sig crypto.Signature) (bool, error) {
	c.sigs = append(c.sigs, sig)
	return len(c.sigs) == 2, nil
}

func (c *certificate) Quorum() rebro.QuorumCertificate {
	return c.q
}
