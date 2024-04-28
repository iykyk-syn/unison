# DAG 

`dag` package holds the core DAG-chain production logic. The necessary components to ensure ever-growing DAG of (compact) 
blocks with availability certification. The DAG doesn't interpret data(batches) anyhow and only ensure they are 
available locally(through Certifier). 

`dag/block` holds block and block id structure with respective serialization. The block mainly consists of hashes to
parent blocks(thus DAG) and batch hashes(thus compact).

`dag/quorum` contains stake-weighted quorum and actual certificates implementation. Quorum defaults to 2f+(where f is 1/3)
fault tolerance. It guarantees every round(height) produces blocks with at least 2f+1 power(by summing stakes of each 
block producer) and that every block gets at least 2f+1 signatures.
