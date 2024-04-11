package adapters

import (
	"github.com/hcnet/go/amount"
	"github.com/hcnet/go/exp/lightaurora/common"
	"github.com/hcnet/go/protocols/aurora/base"
	"github.com/hcnet/go/protocols/aurora/operations"
	"github.com/hcnet/go/support/errors"
)

func populateManageSellOfferOperation(op *common.Operation, baseOp operations.Base) (operations.ManageSellOffer, error) {
	manageSellOffer := op.Get().Body.MustManageSellOfferOp()

	var (
		buyingAssetType string
		buyingCode      string
		buyingIssuer    string
	)
	err := manageSellOffer.Buying.Extract(&buyingAssetType, &buyingCode, &buyingIssuer)
	if err != nil {
		return operations.ManageSellOffer{}, errors.Wrap(err, "xdr.Asset.Extract error")
	}

	var (
		sellingAssetType string
		sellingCode      string
		sellingIssuer    string
	)
	err = manageSellOffer.Selling.Extract(&sellingAssetType, &sellingCode, &sellingIssuer)
	if err != nil {
		return operations.ManageSellOffer{}, errors.Wrap(err, "xdr.Asset.Extract error")
	}

	return operations.ManageSellOffer{
		Offer: operations.Offer{
			Base:   baseOp,
			Amount: amount.String(manageSellOffer.Amount),
			Price:  manageSellOffer.Price.String(),
			PriceR: base.Price{
				N: int32(manageSellOffer.Price.N),
				D: int32(manageSellOffer.Price.D),
			},
			BuyingAssetType:    buyingAssetType,
			BuyingAssetCode:    buyingCode,
			BuyingAssetIssuer:  buyingIssuer,
			SellingAssetType:   sellingAssetType,
			SellingAssetCode:   sellingCode,
			SellingAssetIssuer: sellingIssuer,
		},
		OfferID: int64(manageSellOffer.OfferId),
	}, nil
}
