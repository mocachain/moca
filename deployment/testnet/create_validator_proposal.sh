#!/bin/bash

mocad tx staking create-validator ./proposal.json --keyring-backend test --chain-id "moca_5151-1" --from $1 --node "https://devnet-lcd.mocachain.org:443" -b sync --gas "200000000" --fees "100000000000000000000amoca" --yes
