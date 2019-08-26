package rest

import (
	"net/http"

	"github.com/orientwalt/htdf/client/context"
	"github.com/orientwalt/htdf/client/utils"
	"github.com/orientwalt/htdf/codec"
	"github.com/orientwalt/htdf/crypto/keys/keyerror"
	sdk "github.com/orientwalt/htdf/types"
	"github.com/orientwalt/htdf/types/rest"
	"github.com/orientwalt/htdf/x/auth"
	authtxb "github.com/orientwalt/htdf/x/auth/client/txbuilder"
	"github.com/orientwalt/htdf/accounts/keystore"
	hsign "github.com/orientwalt/htdf/accounts/signs"
	htdfservice "github.com/orientwalt/htdf/x/core"
	hscorecli "github.com/orientwalt/htdf/x/core/client/cli"
)

// SignBody defines the properties of a sign request's body.
type SignBody struct {
	Tx         auth.StdTx   `json:"tx"`
	BaseReq    rest.BaseReq `json:"base_req"`
	Passphrase string       `json:"passphrase"`
}

// nolint: unparam
// sign tx REST handler
func SignTxRequestHandlerFn(cdc *codec.Codec, cliCtx context.CLIContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req SignBody

		if !rest.ReadRESTReq(w, r, cdc, &req) {
			rest.WriteErrorResponse(w, http.StatusBadRequest, "failed to parse request")
			return
		}

		// Santize
		req.BaseReq = req.BaseReq.Sanitize()
		if !req.BaseReq.ValidateBasic(w) {
			rest.WriteErrorResponse(w, http.StatusBadRequest, "failed to parse request")
			return
		}

		// validate tx
		// discard error if it's CodeNoSignatures as the tx comes with no signatures
		if err := req.Tx.ValidateBasic(); err != nil && err.Code() != sdk.CodeNoSignatures {
			rest.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		// derive the from account address and name from the Keybase
		fromAddr, err := sdk.AccAddressFromBech32(req.BaseReq.From)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		priv, err := keystore.GetPrivKeyEx(fromAddr, req.Passphrase, "")
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		txBldr := authtxb.NewTxBuilder(
			utils.GetTxEncoder(cdc),
			req.BaseReq.AccountNumber,
			req.BaseReq.Sequence,
			req.Tx.Fee.Gas,
			1.0,
			false,
			req.BaseReq.ChainID,
			req.Tx.GetMemo(),
			req.Tx.Fee.Amount,
			nil,
		)

		var signedTx auth.StdTx
		signedTx, err = hsign.SignTx(txBldr, signedTx, priv)
		if keyerror.IsErrKeyNotFound(err) {
			rest.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		} else if keyerror.IsErrWrongPassword(err) {
			rest.WriteErrorResponse(w, http.StatusUnauthorized, err.Error())
			return
		} else if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		rest.PostProcessResponse(w, cdc, signedTx, cliCtx.Indent)
	}
}

// SignBody defines the properties of a sign request's body.
type SignRawBody struct {
	Tx         string `json:"tx"`
	Passphrase string `json:"passphrase"`
	Offline    bool   `json:"offline"`
	Encode     bool   `json:"encode"`
}

// sign tx REST handler
func SignTxRawRequestHandlerFn(cdc *codec.Codec, cliCtx context.CLIContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req SignRawBody

		if !rest.ReadRESTReq(w, r, cdc, &req) {
			rest.WriteErrorResponse(w, http.StatusBadRequest, "failed to parse request")
			return
		}

		// load sign tx from string
		stdTx, err := htdfservice.ReadStdTxFromRawData(cliCtx.Codec, req.Tx)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusBadRequest, "transaction decode failed")
			return
		}

		// derive the from account address and name from the Keybase
		if len(stdTx.GetSigners()) == 0 {
			rest.WriteErrorResponse(w, http.StatusBadRequest, "signer not found")
			return
		}

		fromAddr := stdTx.GetSigners()[0]
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		priv, err := keystore.GetPrivKeyEx(fromAddr, req.Passphrase, "")
		if err != nil {
			return
		}

		// sign
		res, err := hscorecli.SignTransaction(authtxb.NewTxBuilderFromCLI(), cliCtx, stdTx, priv, req.Offline)
		if err != nil {
			return
		}

		// response
		if !req.Encode {
			w.Write(res)
		} else {
			w.Write([]byte(htdfservice.Encode_Hex(res)))
		}
	}
}
