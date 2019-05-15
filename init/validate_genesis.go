package init

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tendermint/tendermint/types"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/server"

	gaia_app "github.com/cosmos/cosmos-sdk/cmd/gaia/app"
	gaia_init "github.com/cosmos/cosmos-sdk/cmd/gaia/init"

	"github.com/coinexchain/dex/app"
)

// Validate genesis command takes
func ValidateGenesisCmd(ctx *server.Context, cdc *codec.Codec) *cobra.Command {
	return &cobra.Command{
		Use:   "validate-genesis [file]",
		Args:  cobra.RangeArgs(0, 1),
		Short: "validates the genesis file at the default location or at the location passed as an arg",
		RunE: func(cmd *cobra.Command, args []string) (err error) {

			// Load default if passed no args, otherwise load passed file
			var genesis string
			if len(args) == 0 {
				genesis = ctx.Config.GenesisFile()
			} else {
				genesis = args[0]
			}

			//nolint
			fmt.Fprintf(os.Stderr, "validating genesis file at %s\n", genesis)

			var genDoc types.GenesisDoc
			if genDoc, err = gaia_init.LoadGenesisDoc(cdc, genesis); err != nil {
				return fmt.Errorf("Error loading genesis doc from %s: %s", genesis, err.Error())
			}

			var genstate gaia_app.GenesisState
			if err = cdc.UnmarshalJSON(genDoc.AppState, &genstate); err != nil {
				return fmt.Errorf("Error unmarshaling genesis doc %s: %s", genesis, err.Error())
			}

			if err = app.ValidateGenesisState(genstate); err != nil {
				return fmt.Errorf("Error validating genesis file %s: %s", genesis, err.Error())
			}

			fmt.Printf("File at %s is a valid genesis file for cetd\n", genesis)
			return nil
		},
	}
}
