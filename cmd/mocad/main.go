package main

import (
	"fmt"
	"os"

	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/mocachain/moca/v2/app"
	cmdcfg "github.com/mocachain/moca/v2/cmd/config"
	mocad "github.com/mocachain/moca/v2/cmd/mocad/cmd"
)

func main() {
	setupConfig()
	cmdcfg.RegisterDenoms()

	rootCmd, _ := mocad.NewRootCmd()

	if err := svrcmd.Execute(rootCmd, "mocad", app.DefaultNodeHome); err != nil {
		fmt.Fprintln(rootCmd.OutOrStderr(), err)
		os.Exit(1)
	}
}

func setupConfig() {
	// set the address prefixes
	config := sdk.GetConfig()
	cmdcfg.SetBech32Prefixes(config)
	// TODO fix
	// if err := cmdcfg.EnableObservability(); err != nil {
	// 	panic(err)
	// }
	cmdcfg.SetBip44CoinType(config)
	config.Seal()
}
