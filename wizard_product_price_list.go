// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package product

import (
	"github.com/gleke/hexya/src/actions"
	"github.com/gleke/hexya/src/models"
	"github.com/gleke/pool/h"
	"github.com/gleke/pool/m"
)

func init() {

	h.ProductPriceListWizard().DeclareTransientModel()

	h.ProductPriceListWizard().AddFields(map[string]models.FieldDefinition{
		"PriceList": models.Many2OneField{RelationModel: h.ProductPricelist(), Required: true},
		"Qty1":      models.IntegerField{String: "Quantity-1", Default: models.DefaultValue(1)},
		"Qty2":      models.IntegerField{String: "Quantity-2", Default: models.DefaultValue(5)},
		"Qty3":      models.IntegerField{String: "Quantity-3", Default: models.DefaultValue(10)},
		"Qty4":      models.IntegerField{String: "Quantity-4", Default: models.DefaultValue(0)},
		"Qty5":      models.IntegerField{String: "Quantity-5", Default: models.DefaultValue(0)},
	})

	h.ProductPriceListWizard().Methods().PrintReport().DeclareMethod(
		`PrintReport returns the report action from the data in this popup (not implemented)`,
		func(rs m.ProductPriceListWizardSet) *actions.Action {
			// TODO implement with reports
			return &actions.Action{
				Type: actions.ActionCloseWindow,
			}
		})

}
