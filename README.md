# Unison
> Consensus nodes performing in unison!

The Unison project is a collection of modular and reusable BFT consensus and broadcast software primitives bundled 
together as a protocol set for scalable and modular(lazy) blockchain networks. It's a consensus stack that 
cleanly separates concerns allowing them to scale reliably.

Philosophically, Unison stack draws inspiration from projects like Celestia, LLVM and libp2p. Practically, Unison 
implements protocols alike to [Narwhal](https://arxiv.org/pdf/2105.11827.pdf), [Bullshark](https://arxiv.org/pdf/2201.05677.pdf) 
with [Shoal](https://arxiv.org/pdf/2306.03058.pdf) and [Pilotfish](https://arxiv.org/abs/2401.16292).

## Packages
List of currently implemented packages with implementation status:

* 🔴 [unison-poc](./unison-poc) - Runnable Proof of Concept of Unison project achieving 12MB/s data throughput across 30 
geographically distributed nodes
* 🟡 [dag](./dag) - DAG chain implementation
* 🟢 [rebro](./rebro) - Reliable Broadcast
* 🟢 [bapl](./bapl) - Batch Pool with multicast and im-memory implementations
* 🟡 [crypto](./crypto) - crypto primitives

> 🟢 - Needs minor improvements
> 
> 🟡 - Needs major redesign/refactorings
> 
> 🔴 - To be removed

## Design Goals
> TODO: These are supposed to be extended later and categorized e.g. software design goals vs protocol design goals

* Decouple quorum and commitment data structures from the networking protocols and behaviors. 
  * Modular networking behavior
  * Customizable asymmetric cryptography schemes and hash function
* Clear separation between components. Each component should be individually scalable and, if necessary, deployed on 
separate machines, a.k.a node/internal sharding.
* Future proving. The network should be able to resync from the very beginning yet be able to change crypto primitives
in its lifespan. We use self-describing property to achieve that. All the crypto primitives will be commited together with 
certain unique identifier of the primitive, so that future network iterations can identify which primitive is that to
select correct verification algorithm. E.g. hash function or public-key cryptography scheme.
* Minimal dependencies
* Concurrency. Everything that can be concurrent is and in Golang that's automatically parallel.
* Fully code Symmetry. In fact a lot of flexibility and design goals were achieved by following this principle.
* Minimal golang footprint. The protocol should be PL agnostic
* User-owned serialization. The protocol only defines it for internal messages, while all public types are interfaces
for which users can define serialization formats themselves. 
* DAG-flavoured proposer-builder separation. The identity of the block owners can be decoupled from the block signers.
* Ability to define PoS logic with deterministic or randomized or anonymous quorum. Applications can employ arbitrary rules
around quorum and choose/decide as many validators as needed for them. 
* Heterogenity. A single host may participate in as many network it needs. Lazily on-demand join new p2p networks or
statically configured to follow a concrete set. Node operators don't necessarily have to run a node for each network.
To be more precise, network engineers could force operator to run fully independent host, but at that point it's their 
decision and not software limitation.
  * Yet, networks should be able to interoperate while having full sovereignty over their design and primitives choices
  And this is why we embrace self-describing data-types 
* Symmetry.
* Light Node/Client friendliness

## RoadMap

* [PoC](https://github.com/iykyk-syn/unison/issues/22)
* [Production ready Reliable Broadcast](https://github.com/iykyk-syn/unison/issues/10)
* [TxPool](https://github.com/iykyk-syn/unison/issues/47)
* Tendermint consensus running over Unison via CometBFT fork integrating ABCI++ chains
* Bullshark
* Shoal
* Pilotfish

## FAQ

### Why Golang?
The Golang is only for the start and rapid prototyping. The stack is aimed to have multiple implementations over the 
same spec. Once certain parts of the protocol leave out prototype/PoC status they will be immediately scheduled for 
specification unblocking other implementations. 

#### But why start with Golang and not Rust?
The most _CT likable_ way would be starting straight with Rust, but we are going with Golang for pragmatic reasons. Golang
has this simplicity in it that allows you to experiment and quickly iterate over designs. Rust on the other hand is 
cumbersome. Yes, it gives greater flexibility and more ways of solving a problem, but we believe it's a distraction 
initially. 

Besides, Golang is one of the best tools when it comes to networking and distributed systems with rich ecosystem and 
highly mature. It doesn't have all the great features and flexibility that Rust has to offer, but it sufficient-enough 
to build scalable and robust software with design goals we aim for. 

Another important reason is that current team has multi-year experience building p2p and distributed systems in Go.

### Why [Cap’n Proto](https://capnproto.org/) serialization?
Choosing serialization is crucial. It's a decision that is hard to rewind as project matures. Even our protocol design 
aims to be serialization agnostic by making protocol layers care about bytes with certain constraints, we don't expect 
things to be perfect from very beginning and expect serialization to be invasive across the stack. After careful 
consideration, we settle on Cap’n Proto with the following rationale. 

#### Performance
This is the main reason to use Cap’n Proto. ReBro aims for maximum throughput while allocations during serialization and 
deserialization are major bottlenecks. Cap'n Proto addresses that by providing arena allocators and random in-memory 
field access from allocated buffers, so there is effectively no serialization and deserialization happening.

#### Canonicalization
Cap'n Proto has a well-defined and efficient [canonical form](https://capnproto.org/encoding.html#canonicalization) 
that is necessary for cryptographic nature of ReBro.

#### Protobuf Successor
Cap’n Proto learns from multi-year accumulated protobuf experience and is authored by the same expert.

#### Maturity
Reached stable v1 release after going incrementally with minor releases from v0.1. This indicates maturity and willingness
for the long term support. Besides, it is used by Cloudflare extensively, which is another great signal.

#### RPC
Cap’n Proto provides RPC framework and a versatile IDL to describe those. We don't intend to use it initially, but it 
will be handy at later stages for interoperability across programming languages. Particularly, when network nodes evolve
to sets of microservices where each service is written in a different language.

### Why libp2p?

Libp2p has build multifaceted reputation. Some folks can find it amazing, while some will look at it with disgust.
Objectively, libp2p is a powerful toolbox with most of the _engineering_ problems solved when you tackle p2p networks. 
Most of the protocol designers neglect the complexity that networking and p2p networks bring to the table and this is 
where libp2p can shine by saving countless engineering hours. A simple way of thinking about libp2p would be as a 
collection of conditions and edge-cases accumulated from dozens of p2p networks that any other network will
inevitably face. If you don't want to discover and learn all the implementation details the hard way through trial and 
error - libp2p is your choice.

Another important argument against using libp2p is the overhead it brings. Literally, purely operating transports like
QUIC will be slightly more efficient without libp2p wrappings, e.g. for connection throughput. However, we consider 
this is an acceptable tradeoff at least initially. The overhead libp2p brings, as proven by other networks, will become
a bottleneck in a distant future. The strategy is to bootstraps our development with libp2p, then, once we observe a 
bottleneck, we contribute back by optimizing that portion. If that's not feasible, we get rid of libp2p in that portion 
of the stack in favour of highly optimized solution. The great thing about libp2p's library design is that it is not as 
invasive and together with rebro's system architecture swapping out pieces with optimization purposes won't be 
complicated.
