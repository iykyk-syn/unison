package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/iykyk-syn/unison/crypto/ed25519"
	"github.com/iykyk-syn/unison/dag/quorum"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

var bootstrapProtocol protocol.ID = "/bootstrap"

type Service struct {
	host host.Host

	selfPublicKey ed25519.PublicKey

	log *slog.Logger
}

func NewService(localPublicKey []byte, host host.Host) *Service {
	key, err := ed25519.BytesToPubKey(localPublicKey)
	if err != nil {
		panic(err)
	}

	return &Service{
		host:          host,
		selfPublicKey: key,
		log:           slog.With("module", "bootstrap-svc"),
	}
}

// Start connects to bootstrapper and fetch its peers.
func (serv *Service) Start(ctx context.Context, bootstrapper peer.AddrInfo) error {
	err := serv.host.Connect(ctx, bootstrapper)
	if err != nil {
		return fmt.Errorf("connecting to bootstrapper: %w", err)
	}
	serv.log.DebugContext(ctx, "connected to bootstrapper")

	// this gives time for connections to settle on the bootstrapper and gets us all the peers
	select {
	case <-time.After(time.Second):
	case <-ctx.Done():
	}

	s, err := serv.host.NewStream(ctx, bootstrapper.ID, bootstrapProtocol)
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
			err := serv.host.Connect(ctx, p)
			if err != nil {
				serv.log.Error("connecting to peer", "err", err)
			}
		}()
	}

	serv.log.Debug("started")
	return nil
}

// Serve starts serving bootstrap requests.
func (serv *Service) Serve() {
	serv.host.SetStreamHandler(bootstrapProtocol, func(stream network.Stream) {
		store := serv.host.Peerstore()
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
func (serv *Service) GetMembers(uint64) (*quorum.Includers, error) {
	store := serv.host.Peerstore()
	peers := serv.host.Network().Peers()
	incls := make([]*quorum.Includer, 0, len(peers)+1)
	incls = append(incls, quorum.NewIncluder(serv.selfPublicKey, defaultStake))

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

		incls = append(incls, quorum.NewIncluder(key, defaultStake))
	}

	return quorum.NewIncludersSet(incls), nil
}
