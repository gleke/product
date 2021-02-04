// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package product

import (
	"github.com/gleke/base"
	"github.com/gleke/hexya/src/models"
	"github.com/gleke/hexya/src/models/fields"
	"github.com/gleke/pool/h"
	"github.com/gleke/pool/m"
	"github.com/gleke/pool/q"
)

var fields_Partner = map[string]models.FieldDefinition{
	"PropertyProductPricelist": fields.Many2One{String: "Sale Pricelist", RelationModel: h.ProductPricelist(),
		Compute: h.Partner().Methods().ComputeProductPricelist(),
		Depends: []string{"Country"},
		Inverse: h.Partner().Methods().InverseProductPricelist(),
		Help:    "This pricelist will be used instead of the default one for sales to the current partner"},
	"ProductPricelist": fields.Many2One{String: "Stored Pricelist", RelationModel: h.ProductPricelist(),
		Contexts: base.CompanyDependent},
}

//`ComputeProductPricelist returns the price list applicable for this partner`,
func partner_ComputeProductPricelist(rs m.PartnerSet) m.PartnerData {
	if rs.ID() == 0 {
		// We are processing an Onchange
		return h.Partner().NewData()
	}
	company := h.User().NewSet(rs.Env()).CurrentUser().Company()
	return h.Partner().NewData().SetPropertyProductPricelist(
		h.ProductPricelist().NewSet(rs.Env()).GetPartnerPricelist(rs, company))
}

//`InverseProductPricelist sets the price list for this partner to the given list`,
func partner_InverseProductPricelist(rs m.PartnerSet, priceList m.ProductPricelistSet) {
	var defaultForCountry m.ProductPricelistSet
	if !rs.Country().IsEmpty() {
		defaultForCountry = h.ProductPricelist().Search(rs.Env(),
			q.ProductPricelist().CountryGroupsFilteredOn(
				q.CountryGroup().CountriesFilteredOn(
					q.Country().Code().Equals(rs.Country().Code())))).Limit(1)
	} else {
		defaultForCountry = h.ProductPricelist().Search(rs.Env(),
			q.ProductPricelist().CountryGroups().IsNull()).Limit(1)
	}
	actual := rs.PropertyProductPricelist()
	if !priceList.IsEmpty() || (!actual.IsEmpty() && !defaultForCountry.Equals(actual)) {
		if priceList.IsEmpty() {
			rs.SetProductPricelist(defaultForCountry)
			return
		}
		rs.SetProductPricelist(priceList)
	}
}

//`CommercialFields`,
func partner_CommercialFields(rs m.PartnerSet) []models.FieldNames {
	return append(rs.Super().CommercialFields(), q.Partner().PropertyProductPricelist())
}
func init() {

	h.Partner().AddFields(fields_Partner)
	h.Partner().NewMethod("ComputeProductPricelist", partner_ComputeProductPricelist)
	h.Partner().NewMethod("InverseProductPricelist", partner_InverseProductPricelist)
	h.Partner().Methods().CommercialFields().Extend(partner_CommercialFields)

}
