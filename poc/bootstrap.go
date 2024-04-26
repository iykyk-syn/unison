package poc

import (
	"context"
	"encoding/json"
	"io"
	"log"
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
	bootstrapper  peer.AddrInfo
	networkSize   int
}

func NewBootstrapSvc(localPublicKey []byte, host host.Host, bootstrapper peer.AddrInfo, networkSize int) *BootstrapSvc {
	key, err := ed25519.BytesToPubKey(localPublicKey)
	if err != nil {
		panic(err)
	}

	return &BootstrapSvc{
		host:          host,
		selfPublicKey: key,
		bootstrapper:  bootstrapper,
		networkSize:   networkSize,
	}
}

// Start connects to bootstrapper and fetch its peers.
func (b *BootstrapSvc) Start(ctx context.Context) error {
	err := b.host.Connect(ctx, b.bootstrapper)
	if err != nil {
		return err
	}

	// this gives time for connections to settle on the bootstrapper and gets us all the peers
	select {
	case <-time.After(time.Second):
	case <-ctx.Done():
	}

	s, err := b.host.NewStream(ctx, b.bootstrapper.ID, bootstrapProtocol)
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
				log.Println(err)
			}
		}()
	}

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
			log.Println(err)
			return
		}

		_, err = stream.Write(bytes)
		if err != nil {
			log.Println(err)
			return
		}

		err = stream.CloseWrite()
		if err != nil {
			log.Println(err)
			return
		}
	})
}

const defaultStake = 1000

// GetMembers construct includer/validator set out of network participants
func (b *BootstrapSvc) GetMembers() (*stake.Includers, error) {
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
