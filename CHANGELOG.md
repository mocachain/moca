<!--
Guiding Principles:

Changelogs are for humans, not machines.
There should be an entry for every single version.
The same types of changes should be grouped.
Versions and sections should be linkable.
The latest version comes first.
The release date of each version is displayed.
Mention whether you follow Semantic Versioning.

Usage:

Change log entries are to be added to the Unreleased section under the
appropriate stanza (see below). Each entry should ideally include a tag and
the Github issue reference in the following format:

* (<tag>) \#<issue-number> message

The issue numbers will later be link-ified during the release process so you do
not have to worry about including a link manually, but you can if you wish.

Types of changes (Stanzas):

"Features" for new features.
"Improvements" for changes in existing functionality.
"Deprecated" for soon-to-be removed features.
"Bug Fixes" for any bug fixes.
"Client Breaking" for breaking CLI commands and REST routes used by end-users.
"API Breaking" for breaking exported APIs used by developers building on SDK.
"State Machine Breaking" for any changes that result in a different AppState given same genesisState and txList.

Ref: https://keepachangelog.com/en/1.0.0/
-->

# Changelog

## [v0.1.0] - 2024-03-22

### Features

- (chain) Set prefix to mc and denom to zkme chain name to moca
- (precompile) Add storage module precompile skeleton
- (storage) Add system contract for object NFT

### Improvement

- (chore) Fix test after remove recovery/incentives/revenue/vesting/inflation/claims module and remove upgrades.
- (dev) Add dev.js script for development and testing.
- (dev) Add four quick command and fix stop node bug.
- (deps) Bump btcd version to `v0.23.0`
- (dev) Fix the issue of dev.js script not working after replacing moca-cosmos-sdk.
- (deps) Bump moca-cosmos-sdk version to v0.1.0

### Bug Fixes

- (cli) Use empty string as default value in `chain-id` flag to use the chain id from the genesis file when not specified.
- (evm) Fix deploy the contract but cannot call the contract.

### State Machine Breaking

- (recovery) Remove `x/recovery` module.
- (incentives) Remove `x/incentives` module.
- (revenue) Remove `x/revenue` module.
- (vesting) Remove `x/vesting` module.
- (inflation) Remove `x/inflation` module.
- (claims) Remove `x/claims` module.
- (evm) Enable EIP 3855 for solidity push0 instruction.
- (deps) Bump Cosmos-SDK to v0.47.2 and ibc-go to v7.2.0.
- (evm) Implement EIP 6780.

### API Breaking

- (evm) Implement EIP-1153 transient storage.