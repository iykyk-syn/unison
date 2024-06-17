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
	"github.com/iykyk-syn/unison/dag/block"
	"github.com/iykyk-syn/unison/dag/quorum"
	"github.com/iykyk-syn/unison/rebro"
	"github.com/iykyk-syn/unison/rebro/gossip"
	bootstrap2 "github.com/iykyk-syn/unison/unison-poc/bootstrap"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	p2phost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

var networkID rebro.NetworkID = "poc"

var (
	isBootstrapper bool
	bootstrapper   string
	kickoffTimeout time.Duration
	batchSize      int
	batchTime      time.Duration
	networkSize    int
	listenPort     int
	keyPath        string
)

func init() {
	flag.BoolVar(&isBootstrapper, "is-bootstrapper", false, "To indicate node is bootstrapper")
	flag.StringVar(&bootstrapper, "bootstrapper", "", "Specifies network bootstrapper multiaddr")
	flag.DurationVar(&kickoffTimeout, "kickoff-timeout", time.Second*5, "Timeout before starting block production")
	flag.IntVar(&batchSize, "batch-size", 2000*125, "Batch size to be produced every 'batch-time' (bytes). 0 disables batch production")
	flag.DurationVar(&batchTime, "batch-time", time.Second, "Batch production time")
	flag.IntVar(&networkSize, "network-size", 0, "Expected network size to wait for before starting the network. Skips if 0")
	flag.IntVar(&listenPort, "listen-port", 10000, "Port to listen on for libp2p connections")
	flag.StringVar(&keyPath, "key-path", "", "Path to the p2p private key (relative to home directory)")
	flag.Parse()

	slog.SetLogLoggerLevel(slog.LevelDebug)
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	p2pKey, privKey, err := getIdentity(keyPath)
	if err != nil {
		return err
	}

	signer, err := local.NewSigner(privKey)
	if err != nil {
		return err
	}

	host, err := createLibp2pHost(p2pKey)
	if err != nil {
		return err
	}
	defer host.Close()

	printListeningAddresses(host)

	pubsub, err := pubsub.NewFloodSub(ctx, host)
	if err != nil {
		return err
	}

	bootstrap := bootstrap2.NewService(signer.ID(), host, networkSize)
	if err := startBootstrap(ctx, bootstrap); err != nil {
		return err
	}

	pool := bapl.NewMemPool()
	defer pool.Close()

	mcastPool := bapl.NewMulticastPool(pool, host, host.Network().Peers, signer, &batchVerifier{})
	mcastPool.Start()
	defer mcastPool.Stop()

	cert := dag.NewCertifier(mcastPool)
	hasher := dag.NewHasher()
	broadcaster := gossip.NewBroadcaster(networkID, signer, cert, hasher, block.UnmarshalBlockID, pubsub)

	if err := broadcaster.Start(); err != nil {
		return err
	}
	defer broadcaster.Stop(ctx)

	waitForKickoffOrContext(ctx)

	members, err := bootstrap.GetMembers(0)
	if err != nil {
		return err
	}

	dagger := createDAGChain(broadcaster, mcastPool, members, privKey)
	dagger.Start()
	defer dagger.Stop()

	if batchSize == 0 {
		<-ctx.Done()
		return ctx.Err()
	}

	RandomBatches(ctx, mcastPool, batchSize, batchTime)
	return nil
}

func getIdentity(keyPath string) (libp2pcrypto.PrivKey, crypto.PrivKey, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, err
	}

	if keyPath == "" {
		keyPath = "/.unison/key"
	}
	fullPath := home + keyPath

	dir := home + "/.unison"
	if err = os.MkdirAll(dir, os.ModePerm); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, nil, err
	}

	return loadOrCreateKey(fullPath)
}

func loadOrCreateKey(fullPath string) (libp2pcrypto.PrivKey, crypto.PrivKey, error) {
	var keyBytes []byte
	f, err := os.Open(fullPath)
	if err != nil {
		f, err = os.Create(fullPath)
		if err != nil {
			return nil, nil, err
		}
		defer f.Close()

		keyBytes, err = createNewKey(f)
		if err != nil {
			return nil, nil, err
		}
	} else {
		defer f.Close()
		keyBytes, err = io.ReadAll(f)
		if err != nil {
			return nil, nil, err
		}
	}

	p2pKey, err := libp2pcrypto.UnmarshalPrivateKey(keyBytes)
	if err != nil {
		return nil, nil, err
	}

	return getEd25519Key(p2pKey)
}

func createNewKey(f *os.File) ([]byte, error) {
	privKey, _, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, err
	}

	keyBytes, err := libp2pcrypto.MarshalPrivateKey(privKey)
	if err != nil {
		return nil, err
	}

	if _, err := f.Write(keyBytes); err != nil {
		return nil, err
	}

	if err := f.Sync(); err != nil {
		return nil, err
	}

	return keyBytes, nil
}

func getEd25519Key(p2pKey libp2pcrypto.PrivKey) (libp2pcrypto.PrivKey, crypto.PrivKey, error) {
	keyRaw, err := p2pKey.Raw()
	if err != nil {
		return nil, nil, err
	}
	key := ed25519.PrivateKey(keyRaw)

	slog.Info("identity", "key", hex.EncodeToString(key))
	return p2pKey, key, nil
}

func createLibp2pHost(p2pKey libp2pcrypto.PrivKey) (p2phost.Host, error) {
	listenAddrs := []string{
		fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic-v1", listenPort),
		fmt.Sprintf("/ip6/::/udp/%d/quic-v1", listenPort),
	}
	listenMAddrs := make([]multiaddr.Multiaddr, 0, len(listenAddrs))
	for _, s := range listenAddrs {
		addr, err := multiaddr.NewMultiaddr(s)
		if err != nil {
			return nil, err
		}
		listenMAddrs = append(listenMAddrs, addr)
	}

	return libp2p.New(libp2p.Identity(p2pKey), libp2p.ListenAddrs(listenMAddrs...), libp2p.ResourceManager(&network.NullResourceManager{}))
}

func printListeningAddresses(host p2phost.Host) {
	addrs, err := peer.AddrInfoToP2pAddrs(p2phost.InfoFromHost(host))
	if err != nil {
		fmt.Println("Error getting addresses:", err)
		return
	}

	fmt.Println("The p2p host is listening on:")
	for _, addr := range addrs {
		fmt.Println("* ", addr.String())
	}
	fmt.Println()
}

func startBootstrap(ctx context.Context, bootstrap *bootstrap2.Service) error {
	if isBootstrapper {
		return bootstrap.Serve(ctx)
	}

	maddr, err := multiaddr.NewMultiaddr(bootstrapper)
	if err != nil {
		return fmt.Errorf("wrong bootstrapper multiaddr: %w", err)
	}

	addrInfo, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return err
	}

	return bootstrap.Start(ctx, *addrInfo)
}

func waitForKickoffOrContext(ctx context.Context) {
	select {
	case <-time.After(kickoffTimeout):
	case <-ctx.Done():
	}
}

func createDAGChain(broadcaster *gossip.Broadcaster, mcastPool *bapl.MulticastPool, members *quorum.Includers, privKey crypto.PrivKey) *dag.Chain {
	return dag.NewChain(broadcaster, mcastPool, func(round uint64) (*quorum.Includers, error) {
		return members, nil
	}, privKey.PubKey())
}

type batchVerifier struct{}

func (b *batchVerifier) Verify(context.Context, *bapl.Batch) (bool, error) {
	return true, nil
}
