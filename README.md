# AnonPeerster
AnonPeerster is a **decentralized** messaging system with end-to-end encryption, anonymity, and spam prevention through proof-of-work. It was realized as a course project for the Decentralized Systems Engineering course at EPFL (academic year 2017/2018).

Its goal is to provide a messaging service similar to Whatsapp and Telegram, which are known to implement end-to-end encryption. The latter, however, are centralized in the sense that they are managed by a central authority. AnonPeerster is based on a fully decentralized infrastructure.

## How it works
AnonPeerster implements a distributed system where messages are not stored in a single server, but are scattered across multiple nodes that communicate through a secure [gossip protocol](https://en.wikipedia.org/wiki/Gossip_protocol). The protocol features end-to-end encryption, which means that only the two parties (and not intermediary nodes or any attacker) are able to decrypt the messages and see their contents. Additionally, the network is resistant to failures and malicious (i.e. Byzantine) nodes. 

With regard to end-to-end encryption, centralized messaging systems are based on a [Public-key infrastructure (PKI)](https://en.wikipedia.org/wiki/Public_key_infrastructure). This means that a central authority is responsible for storing a database of identities (e.g. telephone numbers) and their association with a public key, which is then used for encryption purposes.
AnonPeerster borrows some ideas from Tor and Bitcoin, and implements a decentralized name infrastructure. Users do not need to register for using the system, they just need to generate a public/private key pair. A unique username is then derived from the public key, similarly to Tor hidden service domains (e.g. `blockchainbdgpzk.onion`) or Bitcoin wallet addresses. These names are said to be **self-authenticating** because they can be easily verified without relying on a central authority. This renders the protocol secure against impersonation and man-in-the-middle attacks, which have been shown to be feasible (to some extent) attacks in some messaging systems.

AnonPeerster also implements a spam prevention mechanism based on proof-of-work. Each time a user generates an identity or sends a new message, they must solve a hard cryptographic problem that can be easily verified by nodes. Unlike Bitcoin and other blockchain architectures, AnonPeerster does not rely on [consensus](https://en.wikipedia.org/wiki/Consensus_(computer_science)). This comes at the cost of relaxing some assumptions (with no effect on privacy), but has the benefit of scalability and low latency, which are crucial aspects in a messaging service.

More technical details about this project (including security guarantees and potential weaknesses) can be found in the report.

## Dependencies
AnonPeerster is written in [Go](https://golang.org/).
This project also depends on **DeDiS Protobuf** and **go-sqlite3**, which can be installed by running:
```
go get github.com/dedis/protobuf
go get github.com/mattn/go-sqlite3
go install github.com/mattn/go-sqlite3
```
Note that installing **go-sqlite3** requires gcc (both on Linux and on Windows), since it is a cgo package.

## How to run
After compiling the package with `go build` and renaming the executable "Project" to "gossiper", you can run `gossiper -h` to print the list of command-line arguments.
##### Mandatory arguments
- `-dataDir=...` the directory for storing the SQLite3 database and RSA keypair. If the directory does not exist, it will be created (along with an empty database and a new keypair/identity).
- `-gossipAddr=...` address/port for the gossiper socket. You can specify a full IP address:port like `127.0.0.1:5000` to listen on a specific interface, or `:5000` to listen on all interfaces.
##### Optional arguments
- `-peers=...` peers separated by commas.
- `-UIPort=...` port for the HTTP client, which listens only on `localhost`.
- `-powDifficulty=...` proof-of-work difficulty (default: 18 leading zeros).
##### Example
```
gossiper -dataDir=_data/RingA -gossipAddr=:5005 -peers=127.0.0.1:5006,127.0.0.1:5008,127.0.0.1:5001 -UIPort=8080
```

## Test scripts
We have included a test script `ring_test.sh` and `ring_test.bat` (respectively for Linux and Windows). It creates ring network topology with 8 nodes, as shown in the figure below:
![Network topology](https://dariopavllo.github.io/decentralized/topology.png)

The keys and databases for these nodes are stored in the `_data` directory. Of course, you are free to delete these files for your tests. If you delete the `key.bin` file (which contains the RSA keypair), the application will generate a new identity. If you delete the SQLite3 database `messages.db`, it will create a new empty database and synchronize messages as usual. The SQLite database can be opened by any standard SQLite database explorer.

## User interface
You can access the user interface through `http://localhost:UIPort`.

![GUI](https://dariopavllo.github.io/decentralized/ui.png)

With the GUI, you can:
- Send a message to the public room (unencrypted, but signed).
- Open a private chat with one of the known nodes and send a private message (encrypted and signed).
- Show additional information about a message (e.g. its hash) by hovering over the **(i)** icon.
- Add/remove peers.

The console shows some informative messages, such as the proof-of-work status when sending a new message:
```
PUBLIC MESSAGE FROM CLIENT: test
INFO: starting a nonce computation with 16 leading zeros...
INFO: nonce computed in 0.54 seconds (218522 tries)
```

## License
The author of this work is Dario Pavllo. The project is made available under the MIT license.