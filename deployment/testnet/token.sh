#!/bin/bash
mocad tx bank send v0 $1 1000000000000000000000000amoca --keyring-backend test --node https://devnet-lcd.mocachain.org:443 -y --fees 6000000000000amoca

mocad --node https://devnet-lcd.mocachain.org:443 q bank balances $1
