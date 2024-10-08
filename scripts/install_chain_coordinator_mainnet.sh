#!/bin/sh
#set -o errexit -o nounset -o pipefail

# DEFAULT_HOMEP=${HOMEP:-~/.mirumd}
# HOMEP=${HOMEP:-/mnt/volume_fra1_02/terramirum}
HOMEP=${HOMEP:-~/.mirumd}
PASSWORD=${PASSWORD:-12345678}
STAKE=${STAKE_TOKEN:-MIRUM}
FEE=${FEE_TOKEN:-uMIRUM}
CHAIN_ID=${CHAIN_ID:-mirum-1}
MONIKER=${MONIKER:-main}
GENESIS=${GENESIS:-"$HOMEP"/config/genesis.json}
APPTOML=${APPTOML:-"$HOMEP"/config/app.toml}
CLIENTTOML=${CLIENTTOML:-"$HOMEP"/config/client.toml}
CONFIG=${CONFIG:-"$HOMEP"/config/config.toml}
IS_PROD=${IS_PROD:-true}


rm -rf "$HOMEP"

mirumd init --chain-id "$CHAIN_ID" "$MONIKER" --home "$HOMEP"
# staking/governance token is hardcoded in config, change this
sed -i "s/\"stake\"/\"$STAKE\"/" $GENESIS
# this is essential for sub-1s block times (or header times go crazy)
if grep -F "time_iota_ms" $GENESIS
then 
    sed -i 's/"time_iota_ms": "1000"/"time_iota_ms": "500"/' $GENESIS
fi

apt update
apt install -y jq
# Store the original permissions
ORIGINAL_PERMISSIONS=$(stat -c "%a" $GENESIS)
# read and write permissiion to the owner.
chmod 644 $GENESIS
# to enable the api server
sed -i '/\[api\]/,+3 s/enable = false/enable = true/' $APPTOML
# to change the voting_period
jq '.app_state.gov.voting_params.voting_period = "600s"' $GENESIS > temp.json && mv temp.json $GENESIS

# to change the inflation
jq '.app_state.mint.minter.inflation = "0.010000000000000000"' $GENESIS > temp.json && mv temp.json $GENESIS
jq '.app_state.mint.params.inflation_rate_change = "0.010000000000000000"' $GENESIS > temp.json && mv temp.json $GENESIS
jq '.app_state.mint.params.inflation_max = "0.015000000000000000"' $GENESIS > temp.json && mv temp.json $GENESIS
jq '.app_state.mint.params.inflation_min = "0.001000000000000000"' $GENESIS > temp.json && mv temp.json $GENESIS
jq '.app_state.mint.params.goal_bonded = "0.510000000000000000"' $GENESIS > temp.json && mv temp.json $GENESIS
jq '.app_state.mint.params.blocks_per_year = "10519200"' $GENESIS > temp.json && mv temp.json $GENESIS
jq '.app_state.provider.params.max_provider_consensus_validators = "260"' $GENESIS > temp.json && mv temp.json $GENESIS
jq '.app_state.staking.params.max_validators = "300"' $GENESIS > temp.json && mv temp.json $GENESIS
jq '.app_state.slashing.params.downtime_jail_duration = "6000s"' $GENESIS > temp.json && mv temp.json $GENESIS

# making 1 sec block time.
sed -i 's/timeout_commit = "5s"/timeout_commit = "3s"/' $CONFIG

sed -i 's/minimum-gas-prices = ""/minimum-gas-prices = "0.0000001mirum"/' $APPTOML

for file in "$CONFIG" "$APPTOML" "$CLIENTTOML"; do
    sed -i 's/localhost/0.0.0.0/' "$file"
    sed -i 's/127.0.0.1/0.0.0.0/' "$file"
done

if [ "$IS_PROD" = true ]; then
    sed -i 's/log_level = "info"/log_level = "*:error"/' $CONFIG 
fi

if ! mirumd keys show validator --home "$HOMEP"; then
   (echo "$PASSWORD"; echo "$PASSWORD") | mirumd keys add validator --home "$HOMEP"
fi
# hardcode the validator account for this instance
echo "$PASSWORD" | mirumd genesis add-genesis-account validator "100000000000000000$STAKE" --home "$HOMEP"

# submit a genesis validator tx
## Workraround for https://github.com/cosmos/cosmos-sdk/issues/8251
(echo "$PASSWORD"; echo "$PASSWORD"; echo "$PASSWORD") | mirumd genesis gentx validator "25000000000000$STAKE" --chain-id="$CHAIN_ID" --amount="25000000000000$STAKE" --home "$HOMEP"
## should be:
# (echo "$PASSWORD"; echo "$PASSWORD"; echo "$PASSWORD") | mirumd gentx validator "100000000000$STAKE" --chain-id="$CHAIN_ID"
mirumd genesis collect-gentxs --home "$HOMEP"
# Restore the original permissions
chmod $ORIGINAL_PERMISSIONS $GENESIS
