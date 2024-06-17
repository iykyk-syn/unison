# Unison PoC

This PoC is aims to show the throughput capabilities of Unison. It achieves 12MB/s throughput of certified data with
30 geographically distributed nodes deployed [Vultr](https://www.vultr.com) using ansible.
> NOTE: The deployment scripts are not part of this repo.

The PoC application runs a Unison network of predefined amount of peers. Peers connect through the bootstrapper which 
PEx his peer table to all non-bootstrapper nodes. Every peer automatically joins the quorum, gets assigned 1000 stake 
and starts producing bogus data with configurable rate. In the end, we get a network of peer producing a DAG chain of 
blocks with every node contributing to the throughput.

### Usage
```shell
git clone https://github.com/iykyk-syn/unison
cd unison
go build -o ./unison ./unison-poc
```

```text
Usage of ./unison:
  -batch-size int
    	Batch size to be produced every 'batch-time' (bytes). 0 disables batch production (default 250000)
  -batch-time duration
    	Batch production time (default 1s)
  -bootstrapper string
    	Specifies network bootstrapper multiaddr
  -is-bootstrapper
    	To indicate node is bootstrapper
  -key-path string
    	Path to the p2p private key (default "/.unison/key")
  -kickoff-timeout duration
    	Timeout before starting block production (default 5s)
  -listen-port int
    	Port to listen on for libp2p connections (default 10000)
  -network-size int
    	Expected network size to wait for before starting the network. Skips if 0
```

