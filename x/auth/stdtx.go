package auth

import (
	"encoding/json"
	"fmt"

	"github.com/orientwalt/tendermint/crypto"
	"github.com/orientwalt/tendermint/crypto/multisig"

	"github.com/orientwalt/htdf/codec"
	"github.com/orientwalt/htdf/params"
	"github.com/orientwalt/htdf/server/config"
	"github.com/orientwalt/htdf/types"
	sdk "github.com/orientwalt/htdf/types"
)

var (
	_ sdk.Tx = (*StdTx)(nil)

	maxGasWanted = uint64((1 << 63) - 1)
)

// StdTx is a standard way to wrap a Msg with Fee and Signatures.
// NOTE: the first signature is the fee payer (Signatures must not be nil).
type StdTx struct {
	Msgs       []sdk.Msg      `json:"msg"`
	Fee        StdFee         `json:"fee"`
	Signatures []StdSignature `json:"signatures"`
	Memo       string         `json:"memo"`
}

func NewStdTx(msgs []sdk.Msg, fee StdFee, sigs []StdSignature, memo string) StdTx {
	return StdTx{
		Msgs:       msgs,
		Fee:        fee,
		Signatures: sigs,
		Memo:       memo,
	}
}

// GetMsgs returns the all the transaction's messages.
func (tx StdTx) GetMsgs() []sdk.Msg { return tx.Msgs }

// ValidateBasic does a simple and lightweight validation check that doesn't
// require access to any other information.
func (tx StdTx) ValidateBasic() sdk.Error {
	stdSigs := tx.GetSignatures()

	if tx.Fee.Gas > maxGasWanted {
		return sdk.ErrGasOverflow(fmt.Sprintf("invalid gas supplied; %d > %d", tx.Fee.Gas, maxGasWanted))
	}
	if tx.Fee.Amount.IsAnyNegative() {
		return sdk.ErrInsufficientFee(fmt.Sprintf("invalid fee %s amount provided", tx.Fee.Amount))
	}

	// junying-todo, 2019-11-13
	// MinGasPrice Checking
	var gasprice = tx.Fee.GasPrice
	minGasPrices, err := types.ParseDecCoins(config.DefaultMinGasPrices)
	if err != nil {
		return sdk.ErrTxDecode("DefaultMinGasPrices decode error")
	}
	if !gasprice.IsAllGTE(minGasPrices) {
		return sdk.ErrInsufficientFee(fmt.Sprintf("gasprice must be greater than %s", config.DefaultMinGasPrices))
	}
	// junying-todo, 2019-11-13
	// Validate Msgs &
	// Check MinGas for staking txs
	var msgs = tx.Msgs
	if msgs == nil || len(msgs) == 0 {
		return sdk.ErrUnknownRequest("Tx.GetMsgs() must return at least one message in list")
	}
	for _, msg := range msgs {
		// Validate the Msg.
		err := msg.ValidateBasic()
		if err != nil {
			return err
		}
		// Checking minimum gasprice condition for staking transactions
		if msg.Route() != "htdfservice" {
			if tx.Fee.Gas < params.TxStakingDefaultGas {
				return sdk.ErrInternal(fmt.Sprintf("staking tx gas must be greater than %d", params.TxStakingDefaultGas))
			}
		}
	}

	// added & commented by junying, 2019-11-07

	if len(stdSigs) == 0 {
		return sdk.ErrNoSignatures("no signers")
	}
	if len(stdSigs) != len(tx.GetSigners()) {
		return sdk.ErrUnauthorized("wrong number of signers")
	}

	sigCount := 0
	for i := 0; i < len(stdSigs); i++ {
		sigCount += countSubKeys(stdSigs[i].PubKey)
		if uint64(sigCount) > DefaultTxSigLimit {
			return sdk.ErrTooManySignatures(
				fmt.Sprintf("signatures: %d, limit: %d", sigCount, DefaultTxSigLimit),
			)
		}
	}

	return nil
}

// countSubKeys counts the total number of keys for a multi-sig public key.
func countSubKeys(pub crypto.PubKey) int {
	v, ok := pub.(multisig.PubKeyMultisigThreshold)
	if !ok {
		return 1
	}

	numKeys := 0
	for _, subkey := range v.PubKeys {
		numKeys += countSubKeys(subkey)
	}

	return numKeys
}

// GetSigners returns the addresses that must sign the transaction.
// Addresses are returned in a deterministic order.
// They are accumulated from the GetSigners method for each Msg
// in the order they appear in tx.GetMsgs().
// Duplicate addresses will be omitted.
func (tx StdTx) GetSigners() []sdk.AccAddress {
	seen := map[string]bool{}
	var signers []sdk.AccAddress
	for _, msg := range tx.GetMsgs() {
		for _, addr := range msg.GetSigners() {
			if !seen[addr.String()] {
				signers = append(signers, addr)
				seen[addr.String()] = true
			}
		}
	}
	return signers
}

// GetMemo returns the memo
func (tx StdTx) GetMemo() string { return tx.Memo }

// GetSignatures returns the signature of signers who signed the Msg.
// CONTRACT: Length returned is same as length of
// pubkeys returned from MsgKeySigners, and the order
// matches.
// CONTRACT: If the signature is missing (ie the Msg is
// invalid), then the corresponding signature is
// .Empty().
func (tx StdTx) GetSignatures() []StdSignature { return tx.Signatures }

//__________________________________________________________

// StdFee includes the amount of coins paid in fees and the maximum
// gas to be used by the transaction. The ratio yields an effective "gasprice",
// which must be above some miminum to be accepted into the mempool.
type StdFee struct {
	Amount   sdk.Coins    `json:"amount"`
	Gas      uint64       `json:"gas"`
	GasPrice sdk.DecCoins `json:"gasprice"`
}

// junying-todo, 2019-11-07
// fee = gas * gasprice
func CalcFees(gas uint64, gasprices sdk.DecCoins) sdk.Coins {
	Fees := make(sdk.Coins, len(gasprices))
	glDec := sdk.NewDec(int64(gas))
	for i, gp := range gasprices {
		fee := gp.Amount.Mul(glDec)
		Fees[i] = sdk.NewCoin(gp.Denom, fee.Ceil().RoundInt())
	}
	return Fees
}

// NewStdFee returns a new instance of StdFee
// func NewStdFee(gas uint64, amount sdk.Coins) StdFee {
// 	return StdFee{
// 		Amount: amount,
// 		Gas:    gas,
// 	}
// }
func NewStdFee(gas uint64, gasprice sdk.DecCoins) StdFee {
	return StdFee{
		Amount:   CalcFees(gas, gasprice),
		Gas:      gas,
		GasPrice: gasprice,
	}
}

// Bytes for signing later
func (fee StdFee) Bytes() []byte {
	// normalize. XXX
	// this is a sign of something ugly
	// (in the lcd_test, client side its null,
	// server side its [])
	if len(fee.Amount) == 0 {
		fee.Amount = sdk.NewCoins()
	}
	bz, err := msgCdc.MarshalJSON(fee) // TODO
	if err != nil {
		panic(err)
	}
	return bz
}

// GasPrices returns the gas prices for a StdFee.
//
// NOTE: The gas prices returned are not the true gas prices that were
// originally part of the submitted transaction because the fee is computed
// as fee = ceil(gasWanted * gasPrices).
func (fee StdFee) GasPrices() sdk.DecCoins {
	return sdk.NewDecCoins(fee.Amount).QuoDec(sdk.NewDec(int64(fee.Gas)))
}

// junying-todo, 2019-11-07
func (fee StdFee) GetAmount() sdk.DecCoins {
	return fee.GasPrice.MulDec(sdk.NewDec(int64(fee.Gas)))
}

//__________________________________________________________

// StdSignDoc is replay-prevention structure.
// It includes the result of msg.GetSignBytes(),
// as well as the ChainID (prevent cross chain replay)
// and the Sequence numbers for each signature (prevent
// inchain replay and enforce tx ordering per account).
type StdSignDoc struct {
	AccountNumber uint64            `json:"account_number"`
	ChainID       string            `json:"chain_id"`
	Fee           json.RawMessage   `json:"fee"`
	Memo          string            `json:"memo"`
	Msgs          []json.RawMessage `json:"msgs"`
	Sequence      uint64            `json:"sequence"`
}

// StdSignBytes returns the bytes to sign for a transaction.
func StdSignBytes(chainID string, accnum uint64, sequence uint64, fee StdFee, msgs []sdk.Msg, memo string) []byte {
	var msgsBytes []json.RawMessage
	for _, msg := range msgs {
		msgsBytes = append(msgsBytes, json.RawMessage(msg.GetSignBytes()))
	}
	bz, err := msgCdc.MarshalJSON(StdSignDoc{
		AccountNumber: accnum,
		ChainID:       chainID,
		Fee:           json.RawMessage(fee.Bytes()),
		Memo:          memo,
		Msgs:          msgsBytes,
		Sequence:      sequence,
	})
	if err != nil {
		panic(err)
	}
	return sdk.MustSortJSON(bz)
}

// StdSignature represents a sig
type StdSignature struct {
	crypto.PubKey `json:"pub_key"` // optional
	Signature     []byte           `json:"signature"`
}

// DefaultTxDecoder logic for standard transaction decoding
func DefaultTxDecoder(cdc *codec.Codec) sdk.TxDecoder {
	return func(txBytes []byte) (sdk.Tx, sdk.Error) {
		var tx = StdTx{}

		if len(txBytes) == 0 {
			return nil, sdk.ErrTxDecode("txBytes are empty")
		}

		// StdTx.Msg is an interface. The concrete types
		// are registered by MakeTxCodec
		err := cdc.UnmarshalBinaryLengthPrefixed(txBytes, &tx)
		if err != nil {
			return nil, sdk.ErrTxDecode("error decoding transaction").TraceSDK(err.Error())
		}
		// fmt.Println("DefaultTxDecoder:tx", tx)
		return tx, nil
	}
}

// DefaultTxEncoder logic for standard transaction encoding
func DefaultTxEncoder(cdc *codec.Codec) sdk.TxEncoder {
	return func(tx sdk.Tx) ([]byte, error) {
		return cdc.MarshalBinaryLengthPrefixed(tx)
	}
}
