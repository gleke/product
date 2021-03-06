// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package product

import (
	"github.com/gleke/hexya/src/actions"
	"github.com/gleke/hexya/src/models"
	"github.com/gleke/hexya/src/models/fields"
	"github.com/gleke/pool/h"
	"github.com/gleke/pool/m"
)

var fields_ProductPriceListWizard = map[string]models.FieldDefinition{
	"PriceList": fields.Many2One{RelationModel: h.ProductPricelist(), Required: true},
	"Qty1":      fields.Integer{String: "Quantity-1", Default: models.DefaultValue(1)},
	"Qty2":      fields.Integer{String: "Quantity-2", Default: models.DefaultValue(5)},
	"Qty3":      fields.Integer{String: "Quantity-3", Default: models.DefaultValue(10)},
	"Qty4":      fields.Integer{String: "Quantity-4", Default: models.DefaultValue(0)},
	"Qty5":      fields.Integer{String: "Quantity-5", Default: models.DefaultValue(0)},
}

//`PrintReport returns the report action from the data in this popup (not implemented)`,
func product_priceListWizard_PrintReport(rs m.ProductPriceListWizardSet) *actions.Action {
	// TODO implement with reports
	return &actions.Action{
		Type: actions.ActionCloseWindow,
	}
}
func init() {

	models.NewModel("ProductPriceListWizard")

	h.ProductPriceListWizard().AddFields(fields_ProductPriceListWizard)
	h.ProductPriceListWizard().NewMethod("PrintReport", product_priceListWizard_PrintReport)

}
