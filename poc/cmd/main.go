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

	"github.com/iykyk-syn/unison/bapl"
	"github.com/iykyk-syn/unison/crypto"
	"github.com/iykyk-syn/unison/crypto/ed25519"
	"github.com/iykyk-syn/unison/crypto/local"
	"github.com/iykyk-syn/unison/dag"
	block "github.com/iykyk-syn/unison/dag/block"
	"github.com/iykyk-syn/unison/poc"
	"github.com/iykyk-syn/unison/rebro"
	"github.com/iykyk-syn/unison/rebro/gossip"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-pubsub"
	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	p2phost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

var network rebro.NetworkID = "poc"

var (
	isBootstrapper bool
	bootstrapper   string
	kickoffTimeout time.Duration
	batchSize      int
	batchTime      time.Duration
	blockTimeout   time.Duration
)

func init() {
	flag.BoolVar(&isBootstrapper, "is-bootstrapper", false, "To indicate node is bootstrapper")
	flag.StringVar(&bootstrapper, "bootstrapper", "", "Specifies network bootstrapper multiaddr")
	flag.DurationVar(&kickoffTimeout, "kickoff-timeout", time.Second*5, "Timeout before starting block production")
	flag.IntVar(&batchSize, "batch-amount", 1000*125, "Batch size to be produced every 'batch-time' (bytes)")
	flag.DurationVar(&batchTime, "batch-time", time.Second, "Batch production time")
	flag.DurationVar(&blockTimeout, "block-time", 0, "Adds additional time before producing new block")
	flag.Parse()

	slog.SetLogLoggerLevel(slog.LevelDebug)
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	err := run(ctx)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
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

	host, err := libp2p.New(libp2p.Identity(p2pKey))
	if err != nil {
		return err
	}

	addrs, err := peer.AddrInfoToP2pAddrs(p2phost.InfoFromHost(host))
	if err != nil {
		return err
	}

	fmt.Println("The p2p host is listening on:")
	for _, addr := range addrs {
		fmt.Println("* ", addr.String())
	}
	fmt.Println()

	pubsub, err := pubsub.NewGossipSub(ctx, host)
	if err != nil {
		return err
	}

	bootstrap := poc.NewBootstrapSvc(signer.ID(), host)
	if isBootstrapper {
		bootstrap.Serve()
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

	select {
	case <-time.After(kickoffTimeout):
	case <-ctx.Done():
		return ctx.Err()
	}

	pool := bapl.NewMemPool()
	mcastPool := bapl.NewMulticastPool(pool, host, host.Network().Peers, signer, &batchVerifier{})
	mcastPool.Start()
	defer mcastPool.Stop()

	cert := dag.NewCertifier(mcastPool)
	hasher := dag.NewHasher()
	broadcaster := gossip.NewBroadcaster(network, signer, cert, hasher, block.UnmarshalBlockID, pubsub)

	err = broadcaster.Start()
	if err != nil {
		return err
	}
	defer broadcaster.Stop(ctx)

	members, err := bootstrap.GetMembers()
	if err != nil {
		return err
	}

	dagger := dag.NewDagger(broadcaster, mcastPool, members, privKey.PubKey(), blockTimeout)
	dagger.Start()
	defer dagger.Stop()

	poc.RandomBatches(ctx, mcastPool, batchSize, batchTime)

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
	key := ed25519.PrivateKey(keyRaw) // truncate the public key off

	slog.Default().Info("identity", "key", hex.EncodeToString(key))
	return p2pKey, key, nil
}

type batchVerifier struct{}

func (b *batchVerifier) Verify(context.Context, *bapl.Batch) (bool, error) {
	return true, nil
}
