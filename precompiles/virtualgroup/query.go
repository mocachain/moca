package virtualgroup

import (
	"bytes"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	virtualgrouptypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
)

const (
	// GlobalVirtualGroupFamiliesMethodName is the ABI name for the globalVirtualGroupFamilies query.
	GlobalVirtualGroupFamiliesMethodName = "globalVirtualGroupFamilies"
	// GlobalVirtualGroupFamilyMethodName is the ABI name for the globalVirtualGroupFamily query.
	GlobalVirtualGroupFamilyMethodName = "globalVirtualGroupFamily"
)

// GlobalVirtualGroupFamilies queries all the global virtual group family.
func (p Precompile) GlobalVirtualGroupFamilies(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input GlobalVirtualGroupFamiliesArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	key := input.Pagination.Key
	if bytes.Equal(key, []byte{0}) {
		key = nil
	}

	msg := &virtualgrouptypes.QueryGlobalVirtualGroupFamiliesRequest{
		Pagination: &query.PageRequest{
			Key:        key,
			Offset:     input.Pagination.Offset,
			Limit:      input.Pagination.Limit,
			CountTotal: input.Pagination.CountTotal,
			Reverse:    input.Pagination.Reverse,
		},
	}

	res, err := p.virtualGroupKeeper.GlobalVirtualGroupFamilies(ctx, msg)
	if err != nil {
		return nil, err
	}

	gvgFamilies := make([]GlobalVirtualGroupFamily, 0, len(res.GvgFamilies))
	for _, gvgFamily := range res.GvgFamilies {
		gvgFamilies = append(gvgFamilies, GlobalVirtualGroupFamily{
			Id:                    gvgFamily.Id,
			PrimarySpId:           gvgFamily.PrimarySpId,
			GlobalVirtualGroupIds: gvgFamily.GlobalVirtualGroupIds,
			VirtualPaymentAddress: common.HexToAddress(gvgFamily.VirtualPaymentAddress),
		})
	}

	pageResponse := PageResponse{
		NextKey: res.Pagination.NextKey,
		Total:   res.Pagination.Total,
	}

	return method.Outputs.Pack(gvgFamilies, pageResponse)
}

// GlobalVirtualGroupFamily queries the global virtual group family by family id.
func (p Precompile) GlobalVirtualGroupFamily(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input GlobalVirtualGroupFamilyArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &virtualgrouptypes.QueryGlobalVirtualGroupFamilyRequest{
		FamilyId: input.FamilyID,
	}

	res, err := p.virtualGroupKeeper.GlobalVirtualGroupFamily(ctx, msg)
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(GlobalVirtualGroupFamily{
		Id:                    res.GlobalVirtualGroupFamily.Id,
		PrimarySpId:           res.GlobalVirtualGroupFamily.PrimarySpId,
		GlobalVirtualGroupIds: res.GlobalVirtualGroupFamily.GlobalVirtualGroupIds,
		VirtualPaymentAddress: common.HexToAddress(res.GlobalVirtualGroupFamily.VirtualPaymentAddress),
	})
}
