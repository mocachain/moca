package storage

import (
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	evmtypes "github.com/evmos/evmos/v12/x/evm/types"
	"github.com/stretchr/testify/require"
)

// Test that the ABI contains the cancelUpdateObjectContent method and its event,
// and that arguments can be encoded/decoded as expected.
func TestCancelUpdateObjectContent_ABI_And_Args(t *testing.T) {
	// 1) Method exists
	method := GetAbiMethod(CancelUpdateObjectContentMethodName)
	require.NotEqual(t, abi.Method{}, method, "cancelUpdateObjectContent method should exist in ABI")
	require.Equal(t, CancelUpdateObjectContentMethodName, method.Name)

	// 2) Event exists
	event := GetAbiEvent("CancelUpdateObjectContent")
	require.NotEqual(t, abi.Event{}, event, "CancelUpdateObjectContent event should exist in ABI")
	require.Equal(t, "CancelUpdateObjectContent", event.Name)

	// 3) Pack args and then decode via ParseMethodArgs
	args := CancelUpdateObjectContentArgs{
		BucketName: "bucket-a",
		ObjectName: "object-a",
	}
	// ABI encodes the arguments (without the 4-byte method selector)
	encodedArgs, err := method.Inputs.Pack(args.BucketName, args.ObjectName)
	require.NoError(t, err)

	// For ParseMethodArgs we need to pass the full input (after the 4-byte selector).
	// In precompile, types.ParseMethodArgs is invoked with contract.Input[4:],
	// so we directly pass the packed args here.
	var decoded CancelUpdateObjectContentArgs
	err = evmtypes.ParseMethodArgs(method, &decoded, encodedArgs)
	require.NoError(t, err)
	require.Equal(t, args.BucketName, decoded.BucketName)
	require.Equal(t, args.ObjectName, decoded.ObjectName)
}


