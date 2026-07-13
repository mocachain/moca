package client

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/mocachain/moca/v2/types"
)

// ValidateChainID wraps a cobra command with a RunE function with base 10 integer chain-id verification.
func ValidateChainID(baseCmd *cobra.Command) *cobra.Command {
	// Copy base run command to be used after chain verification
	baseRunE := baseCmd.RunE

	// Function to replace command's RunE function
	validateFn := func(cmd *cobra.Command, args []string) error {
		chainID, _ := cmd.Flags().GetString(flags.FlagChainID)

		if !types.IsValidChainID(chainID) {
			return fmt.Errorf("invalid chain-id format: %s", chainID)
		}

		return baseRunE(cmd, args)
	}

	baseCmd.RunE = validateFn
	return baseCmd
}
