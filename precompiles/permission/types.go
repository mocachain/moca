package permission

import (
	"github.com/ethereum/go-ethereum/common"

	"github.com/mocachain/moca/v2/types"
)

var (
	permissionAddress = common.HexToAddress(types.PermissionAddress)
	permissionABI     = types.MustABIJson(IPermissionMetaData.ABI)
)

// GetAddress returns the permission precompile's fixed hex address.
func GetAddress() common.Address {
	return permissionAddress
}
