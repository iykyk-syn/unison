package bapl

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

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
	mcast := NewMulticastPool(mem, host, mocknet.Peers, &verifier{})
	mcast.Start()
	return mcast
}

type verifier struct{}

func (v verifier) Verify(context.Context, *Batch) (bool, error) {
	return true, nil
}

func randBatch() *Batch {
	b := &Batch{make([]byte, 256)}
	rand.Reader.Read(b.Data)
	return b
}
