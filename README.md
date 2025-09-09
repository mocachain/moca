# Moca

Official Golang implementation of the Moca Blockchain. It uses [cometbft](https://github.com/cometbft/cometbft/)
for consensus and build on [cosmos-sdk](https://github.com/cosmos/cosmos-sdk).

Moca aims to facilitate the decentralized data economy by simplifying the process of storing and managing data
access, as well as linking data ownership with the massive DeFi context of the other EVM-compatible blockchains.

Moca operates through three core components, which distinguish it from existing centralized and decentralized
storage systems:

- It enables ethereum-compatible addresses to create and manage data and token assets seamlessly.
- It provides similar API primitives and performance as popular existing Web2 cloud storage systems.

These features offer a novel and innovative approach to decentralized data management and ownership in the DeFi space.
Overall, Moca presents a promising solution for bringing greater flexibility, control, and efficiency to users
in the decentralized data economy.

## Disclaimer

**The software and related documentation are under active development, all subject to potential future change without
notification and not ready for production use. The code and security audit have not been fully completed and not ready
for any bug bounty. We advise you to be careful and experiment on the network at your own risk. Stay safe out there.**

## Moca Core

The center of Moca are two layers:

1. A new storage-oriented blockchain, and
2. network composed of "storage providers".

This repo is the official implementation of Moca blockchain.

The blockchain of Moca serves a dual purpose of maintaining the ledger for users as well as the storage metadata
as common blockchain state data. The blockchain has its native token, MOCA,
and is utilized for gas and governance functionalities. Governance is further enabled through the staking logic that is
unique to the Moca blockchain.

The Moca blockchain has two categories of states that are stored on-chain:

1. The ledger of accounts and their MOCA balance.

2. The metadata of the object storage system and service providers, along with the metadata of the objects stored on the
storage system, permission and billing information pertaining to the storage system.

Transactions on the Moca blockchain have the ability to modify the aforementioned on-chain states. These states and
the transactions that affect them are at the core of the economic data on the Moca platform.

Users looking to create or access data on Moca may do so by engaging with the Moca Core Infrastructure
through decentralized applications known as Moca dApps. These dApps provide a user-friendly interface for
interacting with the platform, enabling users to create and manipulate data in a secure and decentralized environment.

## Documentation

Visit our official [documentation site](https://digitalpulse.larksuite.com/docx/Y1padLPYWop4wvxjgtbu4hEZsXd) for more info.

More advanced script and command line usage, please refer to the [Tutorial](https://digitalpulse.larksuite.com/docx/Y1padLPYWop4wvxjgtbu4hEZsXd#share-J07cdQjtZoN949x9Truu6z0RsQb).

## Key Modules

- `x/evm`: bridges Ethereum's smart contract capabilities with Cosmos' cross-chain functionality and governance systems.
- `x/challenge`: generate random data challenge events or accept user's data challenge requests.
- `x/payment`: handle the billing and payment of the storage module. User fees are paid through "Stream" on Moca,
with a constant rate of payment from users to Storage Providers (SP) with charges applied every second of usage.
- `x/sp`: manage the various storage providers within the network.
- `x/storage`: users can manage their storage data through this module, like create/delete bucket, create/delete storage object.
- `x/permission`: user can manage its resource permission through this module, like put/delete policy for storage object.

And the following modules are in cosmos-sdk:

- `x/crosschain`: manage the cross chain packages, like store/query/update the cross chain package, channels, sequences.
- `x/gashub`: provide a governable and predictable fee charge mechanism.
- `x/oracle`: provide a secure runtime for cross chain packages.
- `x/staking`:  based on the Proof-of-Stake logic. The elected validators are responsible for the security of the moca blockchain.
They get involved in the governance and staking of the blockchain.

Refer to the [docs](https://digitalpulse.larksuite.com/docx/Y1padLPYWop4wvxjgtbu4hEZsXd) to dive deep into these modules.

## Running node

- [Interacting with the Node](https://digitalpulse.larksuite.com/docx/Y1padLPYWop4wvxjgtbu4hEZsXd)
- [Run Local Network](https://digitalpulse.larksuite.com/docx/Y1padLPYWop4wvxjgtbu4hEZsXd#share-W4nedCNrBocF2PxExwyupmZ9s56)
- [Run Node](https://digitalpulse.larksuite.com/docx/Y1padLPYWop4wvxjgtbu4hEZsXd#share-MxWcdZ1ZioMhCixEjuYuDhl1sXb)
- [Become Validator](https://digitalpulse.larksuite.com/docx/Y1padLPYWop4wvxjgtbu4hEZsXd#share-Ff26dlv6ooXRG5x8bhPufs0TsRh)

## Related Projects

- [moca-Contract](https://github.com/zkMeLabs/moca-contracts): the cross chain contract for Moca that deployed on ethereum-compatible network.
- [moca-Storage-Provider](https://github.com/zkMeLabs/moca-storage-provider): the storage service infrastructures provided by either organizations or individuals.
- [moca-relayer](https://github.com/zkMeLabs/moca-relayer): the service that relay cross chain package to both chains.
- [moca-cmd](https://github.com/zkMeLabs/moca-cmd): the most powerful command line to interact with Moca system.
- [Awesome Cosmos](https://github.com/cosmos/awesome-cosmos): Collection of Cosmos related resources which also fits Moca.

## Contribution

Thank you for expressing your willingness to contribute to the Moca source code. We deeply appreciate any help, no
matter how small the fix. We welcome contributions from anyone on the internet, and we value your input.

If you're interested in contributing to Moca, please follow these steps:

1. Fork the project on GitHub.
2. Fix the issue.
3. Commit the changes.
4. Send a pull request for the maintainers to review and merge into the main codebase.

If you're planning to submit more complex changes, we kindly suggest that you reach out to the core developers first.
This could be done through a GitHub issue or our upcoming Discord channel. By doing so, you could ensure that your
changes are aligned with the project's general philosophy, and you can receive valuable feedback that will make your
efforts lighter as well as our review and merge procedures quick and simple.

Once again, thank you for your willingness to contribute to the Moca project. We look forward to working with you!

## License

The moca library (i.e. all code outside the `cmd` directory) is licensed under the
[GNU Lesser General Public License v3.0](https://www.gnu.org/licenses/lgpl-3.0.en.html),
also included in our repository in the `COPYING.LESSER` file.

The moca binaries (i.e. all code inside the `cmd` directory) is licensed under the
[GNU Affero General Public License v3.0](https://www.gnu.org/licenses/agpl-3.0.en.html), also
included in our repository in the `COPYING` file.

## Fork Information

This project is fork from:

+ [greenfield](https://github.com/bnb-chain/greenfield)
+ [evmos v12](https://github.com/evmos/evmos/tree/release/v12.x.x)
  
Significant changes have been made to adapt the project for specific use cases, but much of the core functionality comes from the original project.
