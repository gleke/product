// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package product

import (
	"github.com/gleke/hexya/src/models/fields"
	"log"

	"github.com/gleke/hexya/src/models"
	"github.com/gleke/hexya/src/models/types"
	"github.com/gleke/hexya/src/tools/nbutils"
	"github.com/gleke/pool/h"
	"github.com/gleke/pool/m"
)

var fields_ProductUomCategory = map[string]models.FieldDefinition{
	"Name": models.CharField{String: "Name", Required: true, Translate: true},
}

var fields_ProductUom = map[string]models.FieldDefinition{
	"Name": fields.Char{String: "Unit of Measure", Required: true, Translate: true},
	"Category": fields.Many2One{RelationModel: h.ProductUomCategory(), Required: true, OnDelete: models.Cascade,
		Help: `Conversion between Units of Measure can only occur if they belong to the same category.
	The conversion will be made based on the ratios.`},
	"Factor": fields.Float{String: "Ratio", Default: models.DefaultValue(1.0), Required: true,
		Help: `How much bigger or smaller this unit is compared to the reference Unit of Measure for this category:
	1 * (reference unit) = ratio * (this unit)`},
	"FactorInv": fields.Float{String: "Bigger Ratio", Compute: h.ProductUom().Methods().ComputeFactorInv(),
		Required: true,
		Help: `How many times this Unit of Measure is bigger than the reference Unit of Measure in this category:
	1 * (this unit) = ratio * (reference unit)`,
		Depends: []string{"Factor"}},
	"Rounding": fields.Float{String: "Rounding Precision", Default: models.DefaultValue(0.01),
		Required: true, Help: `The computed quantity will be a multiple of this value.
	Use 1.0 for a Unit of Measure that cannot be further split, such as a piece.`},
	"Active": fields.Boolean{Default: models.DefaultValue(true), Required: true,
		Help: "Uncheck the active field to disable a unit of measure without deleting it."},
	"UomType": fields.Selection{String: "Type", Selection: types.Selection{
		"bigger":    "Bigger than the reference Unit of Measure",
		"reference": "Reference Unit of Measure for this category",
		"smaller":   "Smaller than the reference Unit of Measure",
	}, Default: models.DefaultValue("reference"), Required: true,
		OnChange: h.ProductUom().Methods().OnchangeUomType()},
}

//`ComputeFactorInv computes the inverse factor`,
func product_uom_ComputeFactorInv(rs m.ProductUomSet) m.ProductUomData {
	var factorInv float64
	if rs.Factor() != 0 {
		factorInv = 1 / rs.Factor()
	}
	return h.ProductUom().NewData().SetFactorInv(factorInv)
}

//`OnchangeUomType updates factor when the UoM type is changed`,
func product_uom_OnchangeUomType(rs m.ProductUomSet) m.ProductUomData {
	res := h.ProductUom().NewData()
	if rs.UomType() == "reference" {
		res.SetFactor(1)
	}
	return res

}

func product_uom_Create(rs m.ProductUomSet, data m.ProductUomData) m.ProductUomSet {
	if data.FactorInv() != 0 {
		data.SetFactor(1 / data.FactorInv())
		data.SetFactorInv(0)
	}
	return rs.Super().Create(data)
}

func product_uom_Write(rs m.ProductUomSet, vals m.ProductUomData) bool {
	if vals.HasFactorInv() {
		var factor float64
		if vals.FactorInv() != 0 {
			factor = 1 / vals.FactorInv()
		}
		vals.SetFactor(factor)
		vals.SetFactorInv(0)
	}
	return rs.Super().Write(vals)
}

//`ComputeQuantity converts the given qty from this UoM to toUnit UoM. If round is true,
//		the result will be rounded to toUnit rounding.
//
//		It panics if both units are not from the same category`,
func product_uom_ComputeQuantity(rs m.ProductUomSet, qty float64, toUnit m.ProductUomSet, round bool) float64 {
	if rs.IsEmpty() {
		return qty
	}
	rs.EnsureOne()
	if !rs.Category().Equals(toUnit.Category()) {
		log.Panic(rs.T("Conversion from Product UoM %s to Default UoM %s is not possible as they both belong to different Category!.", rs.Name(), toUnit.Name()))
	}
	amount := qty / rs.Factor()
	if toUnit.IsEmpty() {
		return amount
	}
	amount = amount * toUnit.Factor()
	if round {
		amount = nbutils.Round(amount, toUnit.Rounding())
	}
	return amount
}

//`ComputePrice computes the price per 'toUnit' from the given price per this unit`,
func product_uom_ComputePrice(rs m.ProductUomSet, price float64, toUnit m.ProductUomSet) float64 {
	rs.EnsureOne()
	if price == 0 || toUnit.IsEmpty() || rs.Equals(toUnit) {
		return price
	}
	if !rs.Category().Equals(toUnit.Category()) {
		return price
	}
	amount := price * rs.Factor()
	return amount / toUnit.Factor()
}

func init() {

	models.NewModel("ProductUomCategory")
	h.ProductUomCategory().AddFields(fields_ProductUomCategory)

	models.NewModel("ProductUom")
	h.ProductUom().SetDefaultOrder("Name")

	h.ProductUom().AddFields(fields_ProductUom)

	h.ProductUom().AddSQLConstraint("FactorGtZero", "CHECK (factor!=0)", "The conversion ratio for a unit of measure cannot be 0!")
	h.ProductUom().AddSQLConstraint("RoundingGtZero", "CHECK (rounding>0)", "The rounding precision must be greater than 0!")

	h.ProductUom().NewMethod("ComputeFactorInv", product_uom_ComputeFactorInv)
	h.ProductUom().NewMethod("OnchangeUomType", product_uom_OnchangeUomType)
	h.ProductUom().NewMethod("ComputeQuantity", product_uom_ComputeQuantity)
	h.ProductUom().NewMethod("ComputePrice", product_uom_ComputePrice)

	h.ProductUom().Methods().Create().Extend(product_uom_Create)
	h.ProductUom().Methods().Write().Extend(product_uom_Write)

}
