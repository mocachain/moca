# Docker multi-validator (`deployment/dockerup`)

This directory is mounted into the `init` service in the repository root `docker-compose.yml`.
The container runs `localup.sh` to initialize validators, generate genesis, and export
`validator.json` / `sp.json` for other components.

## Primary workflow

From the `moca` repository root (with Docker network `moca-network` created as needed):

- See `docker-compose.yml` for the exact `init` command (`localup.sh init`, `generate`,
  `copy_genesis`, `persistent_peers`, `export_validator`, `export_sps`).

## Shell scripts

- `localup.sh` – genesis and validator/SP setup (Docker-oriented paths and peer hostnames).
- `create_validator_proposal.sh` – helper for staking proposals; uses
  `create_validator_proposal.json` as a `sed` template (not Node.js).
- `create_sp.json` – example governance proposal JSON for `MsgCreateStorageProvider` (edit before
  use).
- `start-validator.sh`, `backup.sh`, `utils.sh`, `log-manager.sh` – supporting utilities.

## Removed legacy tooling

The previous Node.js `dev.js` and npm-based commands (`npm start`, etc.) have been removed.
Use `localup.sh` and `docker-compose` only.
