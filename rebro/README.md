# ReBro - Reliable Broadcast
Rebro - is an asynchronous byzantine reliable quorum broadcast

Package rebro defines interfaces enabling:
- High throughput censorship resistant data certification(e.g. for availability)
- Static, dynamic or randomized quorums
- Customization of hashing functions and signing schemes, including aggregatable signatures.
- Customization of broadcasting algorithms and networking stacks
- Customizable quorum fault parameters and sizes.

For more details on the interfaces see code comments.

## Use Cases
Besides using it for data availability certification, on can use it for sequencing.

### Variant 1
A trivial sequencing scheme to implement here would be to:
* Require full quorum as finalization condition
* Order blocks by public keys of quorum participants lexicographically

This scheme pertains censorship resistant and has multiple data proposers within a round.

### Variant 2
A round-robin without a stake:
* A QuorumCertificate is initialized with a list of sequencer public keys in the order. 
* On each round, the QuorumCertificate only accepts messages from the predefined rotating sequencer and signatures over 
those from all other sequencers.

Then, on top of that, more sophisticated schemas can be applied, like the addition of stakes, etc.

## Gossip Based implementation
`gossip` is the current only rebro implementation uses gossiping as its backbone, which tradeoffs propagation speed for 
scalability. 

### Why GossipSub?
[As we already covered libp2p](../README.md#why-libp2p), the GossipSub question becomes half covered. Still, we don't 
want to pull every single protocol that libp2p ecosystem brings and deliberately select protocols that solve relevant 
problems. The GossipSub version of reliable broadcast has one important property - peer/node number scale. The GossipSub
can scale to dozens thousands nodes network as demonstrated by Ethereum's beacon chain. Initial implementation of 
reliable broadcast uses GossipSub to enable full and (potentially) light clients to follow the reliable broadcast 
network along, allowing networks to scale to thousands of simultaneous validators/proposers.
