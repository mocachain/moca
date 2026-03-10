ERC20 (Foundry)

1) Local test

- Start a local node in one terminal:
  - Run: `anvil`
- In another terminal:
  - Run tests: `forge test -vv` (no external deps required)
  - Deploy locally:
    - Export a private key from anvil output: `export PRIVATE_KEY=0x...`
    - `chmod +x script/deploy_local.sh && ./script/deploy_local.sh`

2) Deploy to Moca testnet (optional)

- Example command:
```
RPC_URL=https://testnet-rpc.mocachain.org \
PRIVATE_KEY=0xYOUR_PK \
INITIAL_SUPPLY_WEI=1000000000000000000000000 \
./script/deploy_local.sh
```

Notes

- The contract mints the entire initial supply to the deployer.
- Adjust `INITIAL_SUPPLY_WEI` as needed (18 decimals).


