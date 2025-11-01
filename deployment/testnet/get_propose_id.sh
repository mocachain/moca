#!/bin/bash
mocad q tx $1 --node "https://devnet-lcd.mocachain.org:443" --output json | jq '.logs[] | .events[] | select(.type == "submit_proposal") | .attributes[] | select(.key == "proposal_id") | .value | tonumber'
