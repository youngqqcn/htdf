package upgrade

import (
	sdk "github.com/orientwalt/htdf/types"
	"github.com/orientwalt/htdf/version"
)

// GenesisState - all upgrade state that must be provided at genesis
type GenesisState struct {
	GenesisVersion VersionInfo `json:genesis_version`
}

// InitGenesis - build the genesis version For first Version
func InitGenesis(ctx sdk.Context, k Keeper, data GenesisState) {
	genesisVersion := data.GenesisVersion

	k.AddNewVersionInfo(ctx, genesisVersion)
	k.protocolKeeper.ClearUpgradeConfig(ctx)
	k.protocolKeeper.SetCurrentVersion(ctx, genesisVersion.UpgradeInfo.Protocol.Version)
}

// WriteGenesis - output genesis parameters
func ExportGenesis(ctx sdk.Context) GenesisState {
	return GenesisState{
		NewVersionInfo(sdk.DefaultUpgradeConfig("https://github.com/orientwalt/htdf/releases/tag/v"+version.Version), true),
	}
}

// get raw genesis raw message for testing
func DefaultGenesisState() GenesisState {
	return GenesisState{
		NewVersionInfo(sdk.DefaultUpgradeConfig("https://github.com/orientwalt/htdf/releases/tag/v"+version.Version), true),
	}
}

// get raw genesis raw message for testing
func DefaultGenesisStateForTest() GenesisState {
	return GenesisState{
		NewVersionInfo(sdk.DefaultUpgradeConfig("https://github.com/orientwalt/htdf/releases/tag/v"+version.Version), true),
	}
}
