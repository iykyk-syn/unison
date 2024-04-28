package poc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/iykyk-syn/unison/crypto/ed25519"
	"github.com/iykyk-syn/unison/stake"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

var bootstrapProtocol protocol.ID = "/bootstrap"

type BootstrapSvc struct {
	host host.Host

	selfPublicKey ed25519.PublicKey

	log *slog.Logger
}

func NewBootstrapSvc(localPublicKey []byte, host host.Host) *BootstrapSvc {
	key, err := ed25519.BytesToPubKey(localPublicKey)
	if err != nil {
		panic(err)
	}

	return &BootstrapSvc{
		host:          host,
		selfPublicKey: key,
		log:           slog.With("module", "bootstrap-svc"),
	}
}

// Start connects to bootstrapper and fetch its peers.
func (b *BootstrapSvc) Start(ctx context.Context, bootstrapper peer.AddrInfo) error {
	err := b.host.Connect(ctx, bootstrapper)
	if err != nil {
		return fmt.Errorf("connecting to bootstrapper: %w", err)
	}

	// this gives time for connections to settle on the bootstrapper and gets us all the peers
	select {
	case <-time.After(time.Second):
	case <-ctx.Done():
	}

	s, err := b.host.NewStream(ctx, bootstrapper.ID, bootstrapProtocol)
	if err != nil {
		return err
	}

	bytes, err := io.ReadAll(s)
	if err != nil {
		return err
	}

	var peers []peer.AddrInfo
	err = json.Unmarshal(bytes, &peers)
	if err != nil {
		return err
	}

	err = s.Close()
	if err != nil {
		return err
	}

	for _, p := range peers {
		go func() {
			err := b.host.Connect(ctx, p)
			if err != nil {
				b.log.Error("connecting to peer", "err", err)
			}
		}()
	}

	b.log.Debug("started")
	return nil
}

// Serve starts serving bootstrap requests.
func (b *BootstrapSvc) Serve() {
	b.host.SetStreamHandler(bootstrapProtocol, func(stream network.Stream) {
		store := b.host.Peerstore()
		peerIDs := store.PeersWithAddrs()

		peers := make([]peer.AddrInfo, len(peerIDs))
		for i, p := range peerIDs {
			peers[i] = store.PeerInfo(p)
		}

		bytes, err := json.Marshal(peers)
		if err != nil {
			return
		}

		_, err = stream.Write(bytes)
		if err != nil {
			return
		}

		err = stream.CloseWrite()
		if err != nil {
			return
		}
	})
}

const defaultStake = 1000

// GetMembers construct includer/validator set out of network participants
func (b *BootstrapSvc) GetMembers(uint64) (*stake.Includers, error) {
	store := b.host.Peerstore()
	peers := b.host.Network().Peers()
	incls := make([]*stake.Includer, 0, len(peers)+1)
	incls = append(incls, stake.NewIncluder(b.selfPublicKey, defaultStake))

	for _, p := range peers {
		keyWrap := store.PubKey(p)
		keyBytes, err := keyWrap.Raw()
		if err != nil {
			return nil, err
		}

		key, err := ed25519.BytesToPubKey(keyBytes)
		if err != nil {
			return nil, err
		}

		incls = append(incls, stake.NewIncluder(key, defaultStake))
	}

	return stake.NewIncludersSet(incls), nil
}
