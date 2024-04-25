package poc

import (
	"context"
	"encoding/json"
	"io"
	"log"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

var bootstrapProtocol protocol.ID = "/bootstrap"

// Bootstrap ask bootstrapper for peers registered on it
func Bootstrap(ctx context.Context, host host.Host, bootstrapper peer.AddrInfo) error {
	err := host.Connect(ctx, bootstrapper)
	if err != nil {
		return err
	}

	s, err := host.NewStream(ctx, bootstrapper.ID, bootstrapProtocol)
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
			err := host.Connect(ctx, p)
			if err != nil {
				log.Println(err)
			}
		}()
	}

	return nil
}

func ServeBootstrap(host host.Host) {
	host.SetStreamHandler(bootstrapProtocol, func(stream network.Stream) {
		store := host.Peerstore()
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
