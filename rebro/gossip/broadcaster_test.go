package gossip

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	"github.com/iykyk-syn/unison/rebro"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestBroadcaster(t *testing.T) {
	// logging.SetDebugLogging()
	// slog.SetDefault()
	const (
		nodeCount     = 10
		roundCount    = 10
		signThreshold = nodeCount/3*2 + 1
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	t.Cleanup(cancel)

	net, err := mocknet.FullMeshLinked(nodeCount)
	require.NoError(t, err)

	bros := make([]*Broadcaster, nodeCount)
	for i, h := range net.Hosts() {
		bros[i] = broadcasterGood(t, h)
	}

	connect(ctx, t, net)
	start(t, bros)

	for i := 1; i < roundCount+1; i++ {
		wg, _ := errgroup.WithContext(ctx)
		for _, bro := range bros {
			bro := bro
			wg.Go(func() error {
				msg := message(i, bro)
				quorum := newQuorum(nodeCount, signThreshold)
				err := bro.Broadcast(ctx, msg, quorum)
				if err != nil {
					return err
				}

				assert.GreaterOrEqual(t, len(quorum.List()), signThreshold)
				assert.GreaterOrEqual(t, len(quorum.List()[0].Signatures()), signThreshold)
				return nil
			})
		}

		err = wg.Wait()
		require.NoError(t, err)
	}
}

func broadcasterGood(t *testing.T, host host.Host) *Broadcaster {
	psub, err := pubsub.NewGossipSub(context.Background(), host, pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)
	bro := NewBroadcaster(testNetworkID, newTestSigner(), &testVerifier{}, &testHasher{}, unmarshalMessageID, psub)
	return bro
}

func connect(ctx context.Context, t *testing.T, net mocknet.Mocknet) {
	hs := net.Hosts()
	subs := make([]event.Subscription, len(hs))
	for i, h := range hs {
		subs[i], _ = h.EventBus().Subscribe(&event.EvtPeerIdentificationCompleted{})
	}

	err := net.ConnectAllButSelf()
	require.NoError(t, err)

	for _, sub := range subs {
		select {
		case <-sub.Out():
		case <-ctx.Done():
			require.Fail(t, "timeout waiting for peers to connect")
		}
	}
}

func start(t *testing.T, bros []*Broadcaster) {
	for _, bro := range bros {
		err := bro.Start()
		require.NoError(t, err)
	}
}

func message(round int, bro *Broadcaster) rebro.Message {
	data := make([]byte, 1024)
	rand.Read(data)

	hash := sha256.New()
	hash.Write(data)
	digest := hash.Sum(nil)

	msgId := &messageId{
		round:  uint64(round),
		signer: bro.signer.ID(),
		hash:   digest,
	}

	return rebro.Message{
		ID:   msgId,
		Data: data,
	}
}

var testNetworkID rebro.NetworkID = "test"

type testSigner struct {
	privkey ed25519.PrivateKey
}

func (t *testSigner) ID() []byte {
	return t.privkey.Public().(ed25519.PublicKey)
}

func newTestSigner() *testSigner {
	_, privkey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}

	return &testSigner{
		privkey: privkey,
	}
}

func (t *testSigner) Sign(bytes []byte) (rebro.Signature, error) {
	sig, err := t.privkey.Sign(rand.Reader, bytes, crypto.Hash(0))
	if err != nil {
		return rebro.Signature{}, nil
	}

	return rebro.Signature{
		Body:   sig,
		Signer: t.privkey.Public().(ed25519.PublicKey),
	}, nil
}

func (t *testSigner) Verify(bytes []byte, signature rebro.Signature) error {
	key := ed25519.PublicKey(signature.Signer)
	ok := ed25519.Verify(key, bytes, signature.Body)
	if !ok {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

type testHasher struct {
}

func (t *testHasher) Hash(msg rebro.Message) ([]byte, error) {
	hash := sha256.New()
	hash.Write(msg.Data)
	return hash.Sum(nil), nil
}

type testVerifier struct {
}

func (t testVerifier) Verify(ctx context.Context, message rebro.Message) error {
	// simply accept for now
	return nil
}

type messageId struct {
	round  uint64
	signer []byte
	hash   []byte
}

func (m *messageId) Round() uint64 {
	return m.round
}

func (m *messageId) Signer() []byte {
	return m.signer
}

func (m *messageId) Hash() []byte {
	return m.hash
}

func (m *messageId) String() string {
	return fmt.Sprintf("%X", m.hash)
}

func (m *messageId) New() *messageId {
	return &messageId{}
}

func (m *messageId) MarshalBinary() (buf []byte, err error) {
	buf = binary.LittleEndian.AppendUint64(buf, m.round)
	buf = append(buf, m.signer...)
	buf = append(buf, m.hash...)
	return buf, nil
}

func (m *messageId) UnmarshalBinary(bytes []byte) error {
	m.round = binary.LittleEndian.Uint64(bytes)
	m.signer = bytes[8 : 8+32]
	m.hash = bytes[8+32:]
	return nil
}

func (m *messageId) ValidateBasic() error {
	return nil
}

func unmarshalMessageID(bytes []byte) (rebro.MessageID, error) {
	var id messageId
	return &id, id.UnmarshalBinary(bytes)
}

// One node - one vote a.k.a multisigs
type quorum struct {
	Size      int
	Threshold int
	comms     map[string]*certificate
	finalized int
}

func newQuorum(size, threshold int) *quorum {
	return &quorum{
		Size:      size,
		Threshold: threshold,
		comms:     map[string]*certificate{},
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
	comms := make(map[string]*certificate)
	for _, comm := range q.comms {
		if len(comm.sigs) >= q.Threshold {
			comms[comm.Message().ID.String()] = comm
		}
	}

	if len(comms) < q.Threshold {
		return false, nil
	}

	q.comms = comms
	return true, nil
}

type certificate struct {
	q    *quorum
	msg  rebro.Message
	sigs []rebro.Signature
}

func (c *certificate) Message() rebro.Message {
	return c.msg
}

func (c *certificate) Signatures() []rebro.Signature {
	return c.sigs
}

func (c *certificate) AddSignature(sig rebro.Signature) (bool, error) {
	c.sigs = append(c.sigs, sig)
	return len(c.sigs) == c.q.Threshold, nil
}

func (c *certificate) Quorum() rebro.QuorumCertificate {
	return c.q
}
