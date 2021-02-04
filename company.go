// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package product

import (
	"github.com/gleke/hexya/src/models"
	"github.com/gleke/hexya/src/models/fields"
	"github.com/gleke/pool/h"
	"github.com/gleke/pool/m"
	"github.com/gleke/pool/q"
)

var fields_Company = map[string]models.FieldDefinition{
	"DefaultPriceList": fields.Many2One{RelationModel: h.ProductPricelist(),
		Help: "Default Price list for partners of this company"},
}

func company_Create(rs m.CompanySet, vals m.CompanyData) m.CompanySet {
	newCompany := rs.Super().Create(vals)
	priceList := h.ProductPricelist().Search(rs.Env(),
		q.ProductPricelist().Currency().Equals(newCompany.Currency()).And().Company().IsNull()).Limit(1)
	if priceList.IsEmpty() {
		priceList = h.ProductPricelist().Create(rs.Env(), h.ProductPricelist().NewData().
			SetName(newCompany.Name()).
			SetCurrency(newCompany.Currency()))
	}
	newCompany.SetDefaultPriceList(priceList)
	return newCompany
}

func company_Write(rs m.CompanySet, vals m.CompanyData) bool {
	// When we modify the currency of the company, we reflect the change on the list0 pricelist, if
	// that pricelist is not used by another company. Otherwise, we create a new pricelist for the
	// given currency.
	currency := vals.Currency()
	mainPricelist := h.ProductPricelist().Search(rs.Env(), q.ProductPricelist().HexyaExternalID().Equals("product_list0"))
	if currency.IsEmpty() || mainPricelist.IsEmpty() {
		return rs.Super().Write(vals)
	}
	nbCompanies := h.Company().NewSet(rs.Env()).SearchAll().SearchCount()
	for _, company := range rs.Records() {
		if mainPricelist.Company().Equals(company) || (mainPricelist.Company().IsEmpty() && nbCompanies == 1) {
			mainPricelist.SetCurrency(currency)
		} else {
			priceList := h.ProductPricelist().Create(rs.Env(), h.ProductPricelist().NewData().
				SetName(company.Name()).
				SetCurrency(currency))
			company.SetDefaultPriceList(priceList)
		}
	}
	return rs.Super().Write(vals)
}
func init() {

	h.Company().AddFields(fields_Company)
	h.Company().Methods().Create().Extend(company_Create)
	h.Company().Methods().Write().Extend(company_Write)
}
