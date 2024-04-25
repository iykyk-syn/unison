package poc

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBootstrap(t *testing.T) {
	const (
		nodeCount = 10
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	t.Cleanup(cancel)

	net, err := mocknet.FullMeshConnected(nodeCount)
	require.NoError(t, err)
	hosts := net.Hosts()

	bootstrapper := *host.InfoFromHost(hosts[0])
	ServeBootstrap(hosts[0])

	var wg sync.WaitGroup
	for _, h := range hosts[1:] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := Bootstrap(ctx, h, bootstrapper)
			assert.NoError(t, err)
		}()
	}

	wg.Wait()
	for _, h := range hosts {
		require.Len(t, h.Network().Peers(), nodeCount-1)
	}
}
