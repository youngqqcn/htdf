package state

import (
	"fmt"
	"github.com/orientwalt/htdf/store"
	sdk "github.com/orientwalt/htdf/types"
	"github.com/orientwalt/htdf/x/auth"
	"github.com/orientwalt/htdf/x/params"
	"github.com/magiconair/properties/assert"
	"github.com/stretchr/testify/require"
	abci "github.com/orientwalt/tendermint/abci/types"
	dbm "github.com/orientwalt/tendermint/libs/db"
	"github.com/orientwalt/tendermint/libs/log"
	"os"

	newevmtypes "github.com/orientwalt/htdf/evm/types"

	"github.com/orientwalt/htdf/codec"

	tmlog "github.com/orientwalt/tendermint/libs/log"

	"github.com/ethereum/go-ethereum/common"

	"math/big"
	"testing"
)

var (
	accKey     = sdk.NewKVStoreKey("acc")
	authCapKey = sdk.NewKVStoreKey("authCapKey")
	fckCapKey  = sdk.NewKVStoreKey("fckCapKey")
	keyParams  = sdk.NewKVStoreKey("params")
	tkeyParams = sdk.NewTransientStoreKey("transient_params")

	storageKey = sdk.NewKVStoreKey("storage")
	codeKey    = sdk.NewKVStoreKey("code")

	testHash    = common.StringToHash("zhoushx")
	fromAddress = common.StringToAddress("UserA")
	toAddress   = common.StringToAddress("UserB")
	amount      = big.NewInt(0)
	nonce       = uint64(0)
	gasLimit    = big.NewInt(100000)
	coinbase    = fromAddress

	logger = tmlog.NewNopLogger()
)

func newTestCodec1() *codec.Codec {
	cdc := codec.New()
	newevmtypes.RegisterCodec(cdc)
	auth.RegisterCodec(cdc)
	sdk.RegisterCodec(cdc)
	codec.RegisterCrypto(cdc)

	return cdc
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func cleanup(dataDir string) {
	fmt.Printf("cleaning up db dir|dataDir=%s\n", dataDir)
	os.RemoveAll(dataDir)
}

func TestStateDB(t *testing.T) {

	//---------------------stateDB test--------------------------------------
	dataPath := "/tmp/htdfStateDB"
	db := dbm.NewDB("state", dbm.LevelDBBackend, dataPath)

	cdc := newTestCodec1()
	cms := store.NewCommitMultiStore(db)

	cms.MountStoreWithDB(keyParams, sdk.StoreTypeIAVL, nil)
	cms.MountStoreWithDB(codeKey, sdk.StoreTypeIAVL, nil)
	cms.MountStoreWithDB(tkeyParams, sdk.StoreTypeTransient, nil)

	pk := params.NewKeeper(cdc, keyParams, tkeyParams)
	ak := auth.NewAccountKeeper(cdc, accKey, pk.Subspace(auth.DefaultParamspace), auth.ProtoBaseAccount)

	cms.MountStoreWithDB(accKey, sdk.StoreTypeIAVL, nil)
	cms.MountStoreWithDB(storageKey, sdk.StoreTypeIAVL, nil)

	cms.SetPruning(store.PruneNothing)

	err := cms.LoadLatestVersion()
	require.NoError(t, err)

	ms := cms.CacheMultiStore()
	ctx := sdk.NewContext(ms, abci.Header{ChainID: "test-chain-id"}, false, log.NewNopLogger())

	stateDB, err := NewCommitStateDB(ctx, &ak, storageKey, codeKey)
	must(err)

	fmt.Printf("addr=%s|testBalance=%v\n", fromAddress.String(), stateDB.GetBalance(fromAddress))
	stateDB.AddBalance(fromAddress, big.NewInt(1e18))
	fmt.Printf("addr=%s|testBalance=%v\n", fromAddress.String(), stateDB.GetBalance(fromAddress))

	assert.Equal(t, stateDB.GetBalance(fromAddress).String() == "1000000000000000000", true)

	//remove DB dir
	cleanup(dataPath)
}
