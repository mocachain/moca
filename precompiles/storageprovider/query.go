package storageprovider

import (
	"bytes"
	"encoding/hex"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/ethereum/go-ethereum/accounts/abi"

	sptypes "github.com/mocachain/moca/v2/x/sp/types"
)

const (
	// StorageProviderMethodName is the ABI name for the StorageProvider query.
	StorageProviderMethodName = "storageProvider"
	// StorageProvidersMethodName is the ABI name for the StorageProviders query.
	StorageProvidersMethodName = "storageProviders"
	// StorageProviderByOperatorAddressMethodName is the ABI name for the StorageProviderByOperatorAddress query.
	StorageProviderByOperatorAddressMethodName = "storageProviderByOperatorAddress"
	// StorageProviderPriceMethodName is the ABI name for the StorageProviderPrice query.
	StorageProviderPriceMethodName = "storageProviderPrice"
)

// StorageProvider queries a storage provider with specify id.
func (p Precompile) StorageProvider(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input StorageProviderArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.spQuerier.StorageProvider(ctx, &sptypes.QueryStorageProviderRequest{
		Id: input.ID,
	})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(outputStorageProviderInfo(res.StorageProvider))
}

// StorageProviders queries a list of GetStorageProviders items.
func (p Precompile) StorageProviders(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input StorageProvidersArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	if bytes.Equal(input.Pagination.Key, []byte{0}) {
		input.Pagination.Key = nil
	}
	res, err := p.spQuerier.StorageProviders(ctx, &sptypes.QueryStorageProvidersRequest{
		Pagination: &query.PageRequest{
			Key:        input.Pagination.Key,
			Offset:     input.Pagination.Offset,
			Limit:      input.Pagination.Limit,
			CountTotal: input.Pagination.CountTotal,
			Reverse:    input.Pagination.Reverse,
		},
	})
	if err != nil {
		return nil, err
	}

	sps := make([]StorageProvider, 0, len(res.Sps))
	for _, objectInfo := range res.Sps {
		sps = append(sps, *outputStorageProviderInfo(objectInfo))
	}

	var pageResponse PageResponse
	pageResponse.NextKey = res.Pagination.NextKey
	pageResponse.Total = res.Pagination.Total

	return method.Outputs.Pack(sps, pageResponse)
}

// StorageProviderByOperatorAddress queries a StorageProvider by specify operator address.
func (p Precompile) StorageProviderByOperatorAddress(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input StorageProviderByOperatorAddressArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.spQuerier.StorageProviderByOperatorAddress(ctx, &sptypes.QueryStorageProviderByOperatorAddressRequest{
		OperatorAddress: input.OperatorAddress.String(),
	})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(outputStorageProviderInfo(res.StorageProvider))
}

// QuerySpStoragePrice queries the latest storage price of a specific sp.
func (p Precompile) QuerySpStoragePrice(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input StorageProviderPriceArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.spQuerier.QuerySpStoragePrice(ctx, &sptypes.QuerySpStoragePriceRequest{
		SpAddr: input.OperatorAddress.String(),
	})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(outputStoragePrice(&res.SpStoragePrice))
}

func outputStorageProviderInfo(sp *sptypes.StorageProvider) *StorageProvider {
	n := &StorageProvider{
		Id:                 sp.Id,
		OperatorAddress:    sp.OperatorAddress,
		FundingAddress:     sp.FundingAddress,
		SealAddress:        sp.SealAddress,
		ApprovalAddress:    sp.ApprovalAddress,
		GcAddress:          sp.GcAddress,
		MaintenanceAddress: sp.MaintenanceAddress,
		TotalDeposit:       sp.TotalDeposit.BigInt(),
		Status:             uint8(sp.Status),
		Endpoint:           sp.Endpoint,
		Description: Description{
			Moniker:         sp.Description.Moniker,
			Identity:        sp.Description.Identity,
			Website:         sp.Description.Website,
			SecurityContact: sp.Description.SecurityContact,
			Details:         sp.Description.Details,
		},
		BlsKey: hex.EncodeToString(sp.BlsKey),
	}

	return n
}

func outputStoragePrice(sp *sptypes.SpStoragePrice) *SpStoragePrice {
	n := &SpStoragePrice{
		SpId:          sp.SpId,
		UpdateTimeSec: big.NewInt(sp.UpdateTimeSec),
		ReadPrice:     sp.ReadPrice.BigInt(),
		FreeReadQuota: sp.FreeReadQuota,
		StorePrice:    sp.StorePrice.BigInt(),
	}

	return n
}
