package bapl

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	crypto2 "github.com/iykyk-syn/unison/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"
)

func TestMulticastPool(t *testing.T) {
	const (
		nodeCount = 10
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	t.Cleanup(cancel)

	net, err := mocknet.FullMeshConnected(nodeCount)
	require.NoError(t, err)

	pools := make([]*MulticastPool, nodeCount)
	for i := range nodeCount {
		pools[i] = multicast(net.Hosts()[i], net)
	}

	batches := make([][]byte, nodeCount)
	for i, p := range pools {
		b := randBatch()
		batches[i] = b.Hash()

		err := p.Push(ctx, b)
		require.NoError(t, err)
	}

	for _, p := range pools {
		byKey, err := p.ListBySigner(ctx, p.signer.ID())
		require.NoError(t, err)
		require.Len(t, byKey, 1)

		for _, hash := range batches {
			b, err := p.Pull(ctx, hash)
			require.NoError(t, err)
			require.EqualValues(t, hash, b.Hash())

			err = p.Delete(ctx, hash)
			require.NoError(t, err)
		}

		size, err := p.Size(ctx)
		require.NoError(t, err)
		require.Zero(t, size)
	}
}

func multicast(host host.Host, mocknet mocknet.Mocknet) *MulticastPool {
	mem := NewMemPool()
	mcast := NewMulticastPool(mem, host, mocknet.Peers, newTestSigner(), &verifier{})
	mcast.Start()
	return mcast
}

type verifier struct{}

func (v verifier) Verify(context.Context, *Batch) (bool, error) {
	return true, nil
}

func randBatch() *Batch {
	b := &Batch{Data: make([]byte, 256)}
	rand.Reader.Read(b.Data) //nolint: errcheck
	return b
}

// TODO: Dedup with rebro's test signer

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

func (t *testSigner) Sign(bytes []byte) (crypto2.Signature, error) {
	sig, err := t.privkey.Sign(rand.Reader, bytes, crypto.Hash(0))
	if err != nil {
		return crypto2.Signature{}, err
	}

	return crypto2.Signature{
		Body:   sig,
		Signer: t.privkey.Public().(ed25519.PublicKey),
	}, nil
}

func (t *testSigner) Verify(bytes []byte, signature crypto2.Signature) error {
	key := ed25519.PublicKey(signature.Signer)
	ok := ed25519.Verify(key, bytes, signature.Body)
	if !ok {
		return fmt.Errorf("invalid signature")
	}

	return nil
}
