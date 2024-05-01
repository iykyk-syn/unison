# Unison PoC

This PoC runs a Unison network of predefined amount of peers. Peers connect through the bootstrapper which PEx his peer 
table to all non-bootstrapper nodes. Every peer automatically joins the quorum, gets assigned 1000 stake and starts
producing bogus data with configurable rate. In the end, we get a network of peer producing a DAG chain of blocks
with every contributing.

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
  -kickoff-timeout duration
        Timeout before starting block production (default 5s)
  -network-size int
        Expected network size to wait for before starting the network. SKips if 0

```

