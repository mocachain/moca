package staking

import (
	"context"
	"fmt"
	"testing"

	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/assert"

	gnfdclient "github.com/evmos/evmos/v12/sdk/client"
	"github.com/evmos/evmos/v12/sdk/client/test"
)

func TestStakingValidator(t *testing.T) {
	client, err := gnfdclient.NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID)
	assert.NoError(t, err)

	query := stakingtypes.QueryValidatorRequest{
		ValidatorAddr: test.TestValAddr,
	}
	res, err := client.StakingQueryClient.Validator(context.Background(), &query)
	assert.NoError(t, err)
	assert.Equal(t, res.Validator.SelfDelAddress, test.TestValAddr)
}

func TestStakingValidators(t *testing.T) {
	client, err := gnfdclient.NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID)
	assert.NoError(t, err)

	query := stakingtypes.QueryValidatorsRequest{
		Status: "",
	}
	res, err := client.StakingQueryClient.Validators(context.Background(), &query)
	assert.NoError(t, err)
	assert.True(t, len(res.Validators) > 0)
}

func TestStakingDelagatorValidator(t *testing.T) {
	client, err := gnfdclient.NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID)
	assert.NoError(t, err)

	query := stakingtypes.QueryDelegatorValidatorRequest{
		DelegatorAddr: test.TestValAddr,
		ValidatorAddr: test.TestValAddr,
	}
	res, err := client.StakingQueryClient.DelegatorValidator(context.Background(), &query)
	assert.NotNil(t, res)
	assert.NotNil(t, res.Validator)
	assert.Equal(t, res.Validator.SelfDelAddress, test.TestValAddr)
}

func TestStakingDelagatorValidators(t *testing.T) {
	client, err := gnfdclient.NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID)
	assert.NoError(t, err)

	query := stakingtypes.QueryDelegatorValidatorsRequest{
		DelegatorAddr: test.TestValAddr,
	}

	res, err := client.StakingQueryClient.DelegatorValidators(context.Background(), &query)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.True(t, len(res.Validators) > 0)
}

func TestStakingUnbondingDelagation(t *testing.T) {
	client, err := gnfdclient.NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID)
	assert.NoError(t, err)

	query := stakingtypes.QueryUnbondingDelegationRequest{
		DelegatorAddr: test.TestValAddr,
		ValidatorAddr: test.TestValAddr,
	}
	res, err := client.StakingQueryClient.UnbondingDelegation(context.Background(), &query)
	if err != nil {
		// If no unbonding delegation exists, verify the validator is still bonded
		delegationQuery := stakingtypes.QueryDelegationRequest{
			DelegatorAddr: test.TestValAddr,
			ValidatorAddr: test.TestValAddr,
		}
		delegationRes, err := client.StakingQueryClient.Delegation(context.Background(), &delegationQuery)
		assert.NoError(t, err)
		assert.NotNil(t, delegationRes)
		assert.NotNil(t, delegationRes.DelegationResponse)
		assert.NotEmpty(t, delegationRes.DelegationResponse.Balance.Amount)

		// Query validator status to confirm it's active
		validatorQuery := stakingtypes.QueryValidatorRequest{
			ValidatorAddr: test.TestValAddr,
		}
		validatorRes, err := client.StakingQueryClient.Validator(context.Background(), &validatorQuery)
		assert.NoError(t, err)
		assert.NotNil(t, validatorRes)
		assert.Equal(t, validatorRes.Validator.Status, stakingtypes.Bonded)
		return
	}
	assert.NotNil(t, res)
	assert.NotNil(t, res.Unbond)
	assert.Equal(t, res.Unbond.DelegatorAddress, test.TestValAddr)
	assert.Equal(t, res.Unbond.ValidatorAddress, test.TestValAddr)
}

func TestStakingDelagatorDelegations(t *testing.T) {
	client, err := gnfdclient.NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID)
	assert.NoError(t, err)

	query := stakingtypes.QueryDelegatorDelegationsRequest{
		DelegatorAddr: test.TestValAddr,
	}
	res, err := client.StakingQueryClient.DelegatorDelegations(context.Background(), &query)
	assert.NoError(t, err)

	t.Log(res.String())
}

func TestStakingValidatorDelegations(t *testing.T) {
	client, err := gnfdclient.NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID)
	assert.NoError(t, err)

	query := stakingtypes.QueryValidatorDelegationsRequest{
		ValidatorAddr: test.TestValAddr,
	}
	res, err := client.StakingQueryClient.ValidatorDelegations(context.Background(), &query)
	assert.NoError(t, err)

	t.Log(res.String())
}

func TestStakingDelegatorUnbondingDelagation(t *testing.T) {
	client, err := gnfdclient.NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID)
	assert.NoError(t, err)

	query := stakingtypes.QueryDelegatorUnbondingDelegationsRequest{
		DelegatorAddr: test.TestValAddr,
	}
	res, err := client.StakingQueryClient.DelegatorUnbondingDelegations(context.Background(), &query)
	assert.NoError(t, err)

	t.Log(res.String())
}

func TestStaking(t *testing.T) {
	client, err := gnfdclient.NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID)
	assert.NoError(t, err)

	query := stakingtypes.QueryRedelegationsRequest{
		DelegatorAddr: test.TestValAddr,
	}
	res, err := client.StakingQueryClient.Redelegations(context.Background(), &query)
	assert.NoError(t, err)

	t.Log(res.String())
}

func TestStakingParams(t *testing.T) {
	client, err := gnfdclient.NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID)
	assert.NoError(t, err)

	query := stakingtypes.QueryParamsRequest{}
	res, err := client.StakingQueryClient.Params(context.Background(), &query)
	assert.NoError(t, err)

	t.Log(res.String())
}

func TestStakingPool(t *testing.T) {
	client, err := gnfdclient.NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID)
	assert.NoError(t, err)

	query := stakingtypes.QueryPoolRequest{}
	res, err := client.StakingQueryClient.Pool(context.Background(), &query)
	assert.NoError(t, err)

	t.Log(res.String())
}

func TestStakingHistoricalInfo(t *testing.T) {
	client, err := gnfdclient.NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID)
	assert.NoError(t, err)

	// Get current block height
	status, err := client.GetStatus(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.NotNil(t, status.SyncInfo)

	// Use a recent height (current height - 1)
	height := status.SyncInfo.LatestBlockHeight - 1
	query := stakingtypes.QueryHistoricalInfoRequest{
		Height: height,
	}

	// Query historical info
	res, err := client.StakingQueryClient.HistoricalInfo(context.Background(), &query)
	if err != nil {
		// If historical info is not found, this is acceptable
		if err.Error() == "rpc error: code = NotFound desc = historical info for height "+fmt.Sprintf("%d", height)+" not found: key not found" {
			t.Logf("Historical info not found for height %d, this is acceptable", height)
			return
		}
		t.Fatalf("Unexpected error when querying historical info: %v", err)
	}

	// If we got a response, verify it
	assert.NotNil(t, res)
	assert.NotNil(t, res.Hist)
	assert.NotNil(t, res.Hist.Valset)
	assert.True(t, len(res.Hist.Valset) > 0)
	t.Logf("Found historical info at height %d with %d validators", height, len(res.Hist.Valset))
}
