package adapters

import (
	"github.com/hcnet/go/amount"
	"github.com/hcnet/go/exp/lightaurora/common"
	"github.com/hcnet/go/protocols/aurora/base"
	"github.com/hcnet/go/protocols/aurora/operations"
	"github.com/hcnet/go/support/errors"
	"github.com/hcnet/go/xdr"
)

func populateChangeTrustOperation(op *common.Operation, baseOp operations.Base) (operations.ChangeTrust, error) {
	changeTrust := op.Get().Body.MustChangeTrustOp()

	var (
		assetType string
		code      string
		issuer    string

		liquidityPoolID string
	)

	switch changeTrust.Line.Type {
	case xdr.AssetTypeAssetTypeCreditAlphanum4, xdr.AssetTypeAssetTypeCreditAlphanum12:
		err := changeTrust.Line.ToAsset().Extract(&assetType, &code, &issuer)
		if err != nil {
			return operations.ChangeTrust{}, errors.Wrap(err, "xdr.Asset.Extract error")
		}
	case xdr.AssetTypeAssetTypePoolShare:
		assetType = "liquidity_pool_shares"

		if changeTrust.Line.LiquidityPool.Type != xdr.LiquidityPoolTypeLiquidityPoolConstantProduct {
			return operations.ChangeTrust{}, errors.Errorf("unkown liquidity pool type %d", changeTrust.Line.LiquidityPool.Type)
		}

		cp := changeTrust.Line.LiquidityPool.ConstantProduct
		poolID, err := xdr.NewPoolId(cp.AssetA, cp.AssetB, cp.Fee)
		if err != nil {
			return operations.ChangeTrust{}, errors.Wrap(err, "error generating pool id")
		}
		liquidityPoolID = xdr.Hash(poolID).HexString()
	default:
		return operations.ChangeTrust{}, errors.Errorf("unknown asset type %d", changeTrust.Line.Type)
	}

	return operations.ChangeTrust{
		Base: baseOp,
		LiquidityPoolOrAsset: base.LiquidityPoolOrAsset{
			Asset: base.Asset{
				Type:   assetType,
				Code:   code,
				Issuer: issuer,
			},
			LiquidityPoolID: liquidityPoolID,
		},
		Limit:   amount.String(changeTrust.Limit),
		Trustee: issuer,
		Trustor: op.SourceAccount().Address(),
		// TODO:
		TrustorMuxed:   "",
		TrustorMuxedID: 0,
	}, nil
}
