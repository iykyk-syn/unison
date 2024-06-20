package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	p2phost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"

	"github.com/iykyk-syn/unison/bapl"
	"github.com/iykyk-syn/unison/crypto"
	"github.com/iykyk-syn/unison/crypto/ed25519"
	"github.com/iykyk-syn/unison/crypto/local"
	"github.com/iykyk-syn/unison/dag"
	"github.com/iykyk-syn/unison/dag/block"
	"github.com/iykyk-syn/unison/dag/quorum"
	"github.com/iykyk-syn/unison/rebro"
	"github.com/iykyk-syn/unison/rebro/gossip"
	bootstrap2 "github.com/iykyk-syn/unison/unison-poc/bootstrap"
)

var networkID rebro.NetworkID = "poc"

var (
	isBootstrapper bool
	bootstrapper   string
	kickoffTimeout time.Duration
	batchSize      int
	batchTime      time.Duration
	networkSize    int
)

func init() {
	flag.BoolVar(&isBootstrapper, "is-bootstrapper", false,
		"To indicate node is bootstrapper",
	)
	flag.StringVar(&bootstrapper, "bootstrapper", "",
		"Specifies network bootstrapper multiaddr",
	)
	flag.DurationVar(&kickoffTimeout, "kickoff-timeout", time.Second*5,
		"Timeout before starting block production",
	)
	flag.IntVar(&batchSize, "batch-size", 2000*125,
		"Batch size to be produced every 'batch-time' (bytes). 0 disables batch production",
	)
	flag.DurationVar(&batchTime, "batch-time", time.Second, "Batch production time")
	flag.IntVar(&networkSize, "network-size", 0,
		"Expected network size to wait for before starting the network. SKips if 0",
	)
	flag.Parse()

	slog.SetLogLoggerLevel(slog.LevelDebug)
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	err := run(ctx)
	if err != nil {
		fmt.Println(err)
		defer os.Exit(1)
		return
	}
}

func run(ctx context.Context) error {
	p2pKey, privKey, err := getIdentity()
	if err != nil {
		return err
	}

	signer, err := local.NewSigner(privKey)
	if err != nil {
		return err
	}

	listenAddrs := []string{
		"/ip4/0.0.0.0/udp/10000/quic-v1",
		"/ip6/::/udp/10000/quic-v1",
	}
	listenMAddrs := make([]multiaddr.Multiaddr, 0, len(listenAddrs))
	for _, s := range listenAddrs {
		addr, err := multiaddr.NewMultiaddr(s)
		if err != nil {
			return err
		}
		listenMAddrs = append(listenMAddrs, addr)
	}

	host, err := libp2p.New(
		libp2p.Identity(p2pKey),
		libp2p.ListenAddrs(listenMAddrs...),
		libp2p.ResourceManager(&network.NullResourceManager{}),
	)
	if err != nil {
		return err
	}
	defer host.Close()

	addrs, err := peer.AddrInfoToP2pAddrs(p2phost.InfoFromHost(host))
	if err != nil {
		return err
	}

	fmt.Println("The p2p host is listening on:")
	for _, addr := range addrs {
		fmt.Println("* ", addr.String())
	}
	fmt.Println()

	pSub, err := pubsub.NewFloodSub(ctx, host)
	if err != nil {
		return err
	}

	bootstrap := bootstrap2.NewService(signer.ID(), host, networkSize)
	if isBootstrapper {
		err := bootstrap.Serve(ctx)
		if err != nil {
			return err
		}
	} else {
		maddr, err := multiaddr.NewMultiaddr(bootstrapper)
		if err != nil {
			return fmt.Errorf("wrong bootstrapper multiaddr: %w", err)
		}

		addrInfo, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			return err
		}

		err = bootstrap.Start(ctx, *addrInfo)
		if err != nil {
			return err
		}
	}

	pool := bapl.NewMemPool()
	defer pool.Close()
	mcastPool := bapl.NewMulticastPool(pool, host, host.Network().Peers, signer, &batchVerifier{})
	mcastPool.Start()
	defer mcastPool.Stop()

	cert := dag.NewCertifier(mcastPool)
	hasher := dag.NewHasher()
	broadcaster := gossip.NewBroadcaster(networkID, signer, cert, hasher, block.UnmarshalBlockID, pSub)

	err = broadcaster.Start()
	if err != nil {
		return err
	}
	defer broadcaster.Stop(ctx) //nolint: errcheck

	select {
	case <-time.After(kickoffTimeout):
	case <-ctx.Done():
		return ctx.Err()
	}

	memebers, err := bootstrap.GetMembers(0)
	if err != nil {
		return err
	}

	dagger := dag.NewChain(broadcaster, mcastPool, func(uint64) (*quorum.Includers, error) {
		return memebers, nil
	}, privKey.PubKey())
	dagger.Start()
	defer dagger.Stop()

	if batchSize == 0 {
		<-ctx.Done()
		return ctx.Err()
	}
	RandomBatches(ctx, mcastPool, batchSize, batchTime)
	return nil
}

const dir = "/.unison"

func getIdentity() (libp2pcrypto.PrivKey, crypto.PrivKey, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, err
	}

	dir := home + dir
	if err = os.Mkdir(dir, os.ModePerm); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, nil, err
	}

	var keyBytes []byte
	path := dir + "/key"
	f, err := os.Open(path)
	if err != nil {
		f, err = os.Create(path)
		if err != nil {
			return nil, nil, err
		}

		privKey, _, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
		if err != nil {
			defer f.Close()
			return nil, nil, err
		}

		keyBytes, err = libp2pcrypto.MarshalPrivateKey(privKey)
		if err != nil {
			defer f.Close()
			return nil, nil, err
		}

		_, err = f.Write(keyBytes)
		if err != nil {
			defer f.Close()
			return nil, nil, err
		}
		if err = f.Sync(); err != nil {
			return nil, nil, err
		}
	}
	defer f.Close()

	if keyBytes == nil {
		keyBytes, err = io.ReadAll(f)
		if err != nil {
			return nil, nil, err
		}
	}

	p2pKey, err := libp2pcrypto.UnmarshalPrivateKey(keyBytes)
	if err != nil {
		return nil, nil, err
	}

	keyRaw, err := p2pKey.Raw()
	if err != nil {
		return nil, nil, err
	}
	key := ed25519.PrivateKey(keyRaw)

	slog.Info("identity", "key", hex.EncodeToString(key))
	return p2pKey, key, nil
}

type batchVerifier struct{}

func (b *batchVerifier) Verify(context.Context, *bapl.Batch) (bool, error) {
	return true, nil
}
