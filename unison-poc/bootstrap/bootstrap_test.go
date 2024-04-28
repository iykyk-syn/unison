package bootstrap

import (
	"context"
	"crypto/rand"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	bhost "github.com/libp2p/go-libp2p/p2p/host/blank"
	swarmt "github.com/libp2p/go-libp2p/p2p/net/swarm/testing"
	"github.com/libp2p/go-libp2p/p2p/protocol/identify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBootstrap(t *testing.T) {
	const (
		nodeCount = 10
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	t.Cleanup(cancel)

	hosts := make([]host.Host, nodeCount)
	keys := make([][]byte, nodeCount)
	for i := range nodeCount {
		randKey := make([]byte, 32)
		rand.Reader.Read(randKey)
		keys[i] = randKey
		hosts[i] = testHost(t, randKey)
	}

	bootstrapper := *host.InfoFromHost(hosts[0])

	svcs := make([]*Service, nodeCount)
	for i, h := range hosts {
		svcs[i] = NewService(keys[i], h)
	}

	var wg sync.WaitGroup
	svcs[0].Serve()
	for _, svc := range svcs[1:] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := svc.Start(ctx, bootstrapper)
			assert.NoError(t, err)
		}()
	}

	wg.Wait()
	time.Sleep(time.Second * 1)
	for _, h := range hosts {
		assert.Len(t, h.Network().Peers(), nodeCount-1)
	}

	for _, svc := range svcs[1:] {
		incls, err := svc.GetMembers(0)
		require.NoError(t, err)
		assert.Equal(t, incls.Len(), nodeCount)
		assert.EqualValues(t, incls.TotalStake(), nodeCount*defaultStake)
	}
}

func testHost(t *testing.T, key []byte) host.Host {
	netw := swarmt.GenSwarm(t)
	h := bhost.NewBlankHost(netw)
	id, err := identify.NewIDService(h, identify.UserAgent(string(key)))
	require.NoError(t, err)
	id.Start()
	return h
}
