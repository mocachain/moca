package storage

import (
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
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

	// 3) Pack args and then decode via the ABI unpack/copy path used by the precompile
	args := CancelUpdateObjectContentArgs{
		BucketName: "bucket-a",
		ObjectName: "object-a",
	}
	// ABI encodes the arguments (without the 4-byte method selector)
	encodedArgs, err := method.Inputs.Pack(args.BucketName, args.ObjectName)
	require.NoError(t, err)

	// The precompile decodes calldata (after the 4-byte selector) via
	// method.Inputs.Unpack followed by method.Inputs.Copy into the arg struct.
	unpacked, err := method.Inputs.Unpack(encodedArgs)
	require.NoError(t, err)
	var decoded CancelUpdateObjectContentArgs
	err = method.Inputs.Copy(&decoded, unpacked)
	require.NoError(t, err)
	require.Equal(t, args.BucketName, decoded.BucketName)
	require.Equal(t, args.ObjectName, decoded.ObjectName)
}
