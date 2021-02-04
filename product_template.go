// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package product

import (
	"github.com/gleke/hexya/src/models/fields"
	"log"

	"github.com/gleke/base"
	"github.com/gleke/decimalPrecision"
	"github.com/gleke/hexya/src/models"
	"github.com/gleke/hexya/src/models/operator"
	"github.com/gleke/hexya/src/models/security"
	"github.com/gleke/hexya/src/models/types"
	"github.com/gleke/hexya/src/models/types/dates"
	"github.com/gleke/hexya/src/tools/b64image"
	"github.com/gleke/pool/h"
	"github.com/gleke/pool/m"
	"github.com/gleke/pool/q"
)

var fields_ProductTemplate = map[string]models.FieldDefinition{
	"Name": fields.Char{Index: true, Required: true, Translate: true},
	"Sequence": fields.Integer{Default: models.DefaultValue(1),
		Help: "Gives the sequence order when displaying a product list"},
	"Description": fields.Char{Translate: true,
		Help: "A precise description of the Product, used only for internal information purposes."},
	"DescriptionPurchase": fields.Char{String: "Purchase Description", Translate: true,
		Help: `A description of the Product that you want to communicate to your vendors.
This description will be copied to every Purchase Order, Receipt and Vendor Bill/Refund.`},
	"DescriptionSale": fields.Char{String: "Sale Description", Translate: true,
		Help: `A description of the Product that you want to communicate to your customers.
This description will be copied to every Sale Order, Delivery Order and Customer Invoice/Refund`},
	"Type": fields.Char{String: "Product Type", Selection: types.Selection{
		"consu":   "Consumable",
		"service": "Service",
	}, Default: models.DefaultValue("consu"), Required: true,
		Help: `A stockable product is a product for which you manage stock. The "Inventory" app has to be installed.
- A consumable product on the other hand is a product for which stock is not managed.
- A service is a non-material product you provide.
- A digital content is a non-material product you sell online.
	The files attached to the products are the one that are sold on
	the e-commerce such as e-books, music, pictures,...
	The "Digital Product" module has to be installed.`},
	"Rental": fields.Boolean{String: "Can be Rent"},
	"Category": fields.Many2One{String: "Internal Category", RelationModel: h.ProductCategory(),
		Default: func(env models.Environment) interface{} {
			if env.Context().HasKey("category_id") {
				return h.ProductCategory().Browse(env, []int64{env.Context().GetInteger("category_id")})
			}
			if env.Context().HasKey("default_category_id") {
				return h.ProductCategory().Browse(env, []int64{env.Context().GetInteger("default_category_id")})
			}
			category := h.ProductCategory().Search(env, q.ProductCategory().HexyaExternalID().Equals("product_product_category_all"))
			if category.Type() != "normal" {
				return h.ProductCategory().NewSet(env)
			}
			return category
		}, Filter: q.ProductCategory().Type().Equals("normal"), Required: true,
		Help: "Select category for the current product"},
	"Currency": fields.Many2One{RelationModel: h.Currency(),
		Compute: h.ProductTemplate().Methods().ComputeCurrency()},
	"Price": fields.Float{Compute: h.ProductTemplate().Methods().ComputeTemplatePrice(),
		Inverse: h.ProductTemplate().Methods().InverseTemplatePrice(),
		Digits:  decimalPrecision.GetPrecision("Product Price")},
	"ListPrice": fields.Float{String: "Sale Price", Default: models.DefaultValue(1.0),
		Digits: decimalPrecision.GetPrecision("Product Price"),
		Help:   "Base price to compute the customer price. Sometimes called the catalog price."},
	"LstPrice": fields.Float{String: "Public Price", Related: "ListPrice",
		Digits: decimalPrecision.GetPrecision("Product Price")},
	"StandardPrice": fields.Float{String: "Cost",
		Compute: h.ProductTemplate().Methods().ComputeStandardPrice(),
		Depends: []string{"ProductVariants", "ProductVariants.StandardPrice"},
		Inverse: h.ProductTemplate().Methods().InverseStandardPrice(),
		Digits:  decimalPrecision.GetPrecision("Product Price"),
		InvisibleFunc: func(env models.Environment) (bool, models.Conditioner) {
			return !security.Registry.HasMembership(env.Uid(), base.GroupUser), nil
		},
		Help: "Cost of the product, in the default unit of measure of the product."},
	"Volume": models.FloatField{Compute: h.ProductTemplate().Methods().ComputeVolume(),
		Depends: []string{"ProductVariants", "ProductVariants.Volume"},
		Inverse: h.ProductTemplate().Methods().InverseVolume(), Help: "The volume in m3.", Stored: true},
	"Weight": fields.Float{Compute: h.ProductTemplate().Methods().ComputeWeight(),
		Depends: []string{"ProductVariants", "ProductVariants.Weight"},
		Inverse: h.ProductTemplate().Methods().InverseWeight(),
		Digits:  decimalPrecision.GetPrecision("Stock Weight"), Stored: true,
		Help: "The weight of the contents in Kg, not including any packaging, etc."},
	"Warranty": fields.Float{},
	"SaleOk": fields.Boolean{String: "Can be Sold", Default: models.DefaultValue(true),
		Help: "Specify if the product can be selected in a sales order line."},
	"PurchaseOk": fields.Boolean{String: "Can be Purchased", Default: models.DefaultValue(true)},
	"Pricelist": fields.Many2One{String: "Pricelist", RelationModel: h.ProductPricelist(),
		Stored: false, Help: "Technical field. Used for searching on pricelists, not stored in database."},
	"Uom": fields.Many2One{String: "Unit of Measure", RelationModel: h.ProductUom(),
		Default: func(env models.Environment) interface{} {
			return h.ProductUom().NewSet(env).SearchAll().Limit(1).OrderBy("ID")
		}, Required: true, Help: "Default Unit of Measure used for all stock operation.",
		Constraint: h.ProductTemplate().Methods().CheckUom(),
		OnChange:   h.ProductTemplate().Methods().OnchangeUom()},
	"UomPo": fields.Many2One{String: "Purchase Unit of Measure", RelationModel: h.ProductUom(),
		Default: func(env models.Environment) interface{} {
			return h.ProductUom().NewSet(env).SearchAll().Limit(1).OrderBy("ID")
		}, Required: true, Constraint: h.ProductTemplate().Methods().CheckUom(),
		Help: "Default Unit of Measure used for purchase orders. It must be in the same category than the default unit of measure."},
	"Company": fields.Many2One{String: "Company", RelationModel: h.Company(),
		Default: func(env models.Environment) interface{} {
			return h.ProductUom().NewSet(env).SearchAll().Limit(1).OrderBy("ID")
		}, Index: true},
	"Packagings": fields.One2Many{String: "Logistical Units", RelationModel: h.ProductPackaging(),
		ReverseFK: "ProductTmpl", JSON: "packaging_ids",
		Help: `Gives the different ways to package the same product. This has no impact on
	the picking order and is mainly used if you use the EDI module.`},
	"Sellers": fields.One2Many{String: "Vendors", RelationModel: h.ProductSupplierinfo(),
		ReverseFK: "ProductTmpl", JSON: "seller_ids"},
	"Active": fields.Boolean{Default: models.DefaultValue(true), Required: true,
		Help: "If unchecked, it will allow you to hide the product without removing it."},
	"Color": fields.Integer{String: "Color Index"},
	"AttributeLines": fields.One2Many{String: "Product Attributes",
		RelationModel: h.ProductAttributeLine(), ReverseFK: "ProductTmpl", JSON: "attribute_line_ids"},
	"ProductVariants": fields.One2Many{String: "Products", RelationModel: h.ProductProduct(),
		ReverseFK: "ProductTmpl", JSON: "product_variant_ids", Required: true},
	"ProductVariant": fields.Many2One{String: "Product", RelationModel: h.ProductProduct(),
		Compute: h.ProductTemplate().Methods().ComputeProductVariant(),
		Depends: []string{"ProductVariants"}},
	"ProductVariantCount": fields.Integer{String: "# Product Variants",
		Compute: h.ProductTemplate().Methods().ComputeProductVariantCount(),
		Depends: []string{"ProductVariants"}, GoType: new(int)},
	"Barcode": fields.Char{},
	"DefaultCode": fields.Char{String: "Internal Reference",
		Compute: h.ProductTemplate().Methods().ComputeDefaultCode(),
		Depends: []string{"ProductVariants", "ProductVariants.DefaultCode"},
		Inverse: h.ProductTemplate().Methods().InverseDefaultCode(), Stored: true},
	"Items": fields.One2Many{String: "Pricelist Items", RelationModel: h.ProductPricelistItem(),
		ReverseFK: "ProductTmpl", JSON: "item_ids"},
	"Image": fields.Binary{
		Help: "This field holds the image used as image for the product, limited to 1024x1024px."},
	"ImageMedium": fields.Binary{String: "Medium-sized image",
		Help: `Medium-sized image of the product. It is automatically
	resized as a 128x128px image, with aspect ratio preserved,
	only when the image exceeds one of those sizes.
	Use this field in form views or some kanban views.`},
	"ImageSmall": fields.Binary{String: "Small-sized image",
		Help: `Small-sized image of the product. It is automatically
	resized as a 64x64px image, with aspect ratio preserved.
	Use this field anywhere a small image is required.`},
}

//`ComputeProductVariant returns the first variant of this template`,
func product_template_ComputeProductVariant(rs m.ProductTemplateSet) m.ProductTemplateData {
	return h.ProductTemplate().NewData().
		SetProductVariant(rs.ProductVariants().Records()[0])
}

//`ComputeCurrency computes the currency of this template`,
func product_template_ComputeCurrency(rs m.ProductTemplateSet) m.ProductTemplateData {
	mainCompany := h.Company().NewSet(rs.Env()).Sudo().Search(
		q.Company().HexyaExternalID().Equals("base_main_company"))
	if mainCompany.IsEmpty() {
		mainCompany = h.Company().NewSet(rs.Env()).Sudo().SearchAll().Limit(1).OrderBy("ID")
	}
	currency := mainCompany.Currency()
	if !rs.Company().Sudo().Currency().IsEmpty() {
		currency = rs.Company().Sudo().Currency()
	}
	return h.ProductTemplate().NewData().SetCurrency(currency)
}

//`ComputeTemplatePrice returns the price of this template depending on the context:
//
//		- 'partner' => int64 (id of the partner)
//		- 'pricelist' => int64 (id of the price list)
//		- 'quantity' => float64`,
func product_template_ComputeTemplatePrice(rs m.ProductTemplateSet) m.ProductTemplateData {
	if !rs.Env().Context().HasKey("pricelist") {
		return h.ProductTemplate().NewData()
	}
	priceListID := rs.Env().Context().GetInteger("pricelist")
	priceList := h.ProductPricelist().Browse(rs.Env(), []int64{priceListID})
	if priceList.IsEmpty() {
		return h.ProductTemplate().NewData()
	}
	partnerID := rs.Env().Context().GetInteger("partner")
	partner := h.Partner().Browse(rs.Env(), []int64{partnerID})
	quantity := rs.Env().Context().GetFloat("quantity")
	if quantity == 0 {
		quantity = 1
	}
	return h.ProductTemplate().NewData().
		SetPrice(priceList.GetProductPrice(rs.ProductVariant(), quantity, partner, dates.Today(), h.ProductUom().NewSet(rs.Env())))
}

//`InverseTemplatePrice sets the template's price`,
func product_template_InverseTemplatePrice(rs m.ProductTemplateSet, price float64) {
	if rs.Env().Context().HasKey("uom") {
		uom := h.ProductUom().Browse(rs.Env(), []int64{rs.Env().Context().GetInteger("uom")})
		value := uom.ComputePrice(price, rs.Uom())
		rs.SetListPrice(value)
		return
	}
	rs.SetListPrice(price)
}

//`ComputeStandardPrice returns the standard price for this template`,
func product_template_ComputeStandardPrice(rs m.ProductTemplateSet) m.ProductTemplateData {
	if rs.ProductVariants().Len() == 1 {
		return h.ProductTemplate().NewData().
			SetStandardPrice(rs.ProductVariant().StandardPrice())
	}
	return h.ProductTemplate().NewData()
}

//`InverseStandardPrice sets this template's standard price`,
func product_template_InverseStandardPrice(rs m.ProductTemplateSet, price float64) {
	if rs.ProductVariants().Len() == 1 {
		rs.ProductVariant().SetStandardPrice(price)
	}
}

//`ComputeVolume compute the volume of this template`,
func product_template_ComputeVolume(rs m.ProductTemplateSet) m.ProductTemplateData {
	if rs.ProductVariants().Len() == 1 {
		return h.ProductTemplate().NewData().
			SetVolume(rs.ProductVariant().Volume())
	}
	return h.ProductTemplate().NewData()
}

//`InverseVolume sets this template's volume`,
func product_template_InverseVolume(rs m.ProductTemplateSet, volume float64) {
	if rs.ProductVariants().Len() == 1 {
		rs.ProductVariant().SetVolume(volume)
	}
}

//`ComputeWeight compute the weight of this template`,
func product_template_ComputeWeight(rs m.ProductTemplateSet) m.ProductTemplateData {
	if rs.ProductVariants().Len() == 1 {
		return h.ProductTemplate().NewData().
			SetWeight(rs.ProductVariant().Weight())
	}
	return h.ProductTemplate().NewData()
}

//`InverseWeightsets this template's weight`,
func product_template_InverseWeight(rs m.ProductTemplateSet, weight float64) {
	if rs.ProductVariants().Len() == 1 {
		rs.ProductVariant().SetWeight(weight)
	}
}

//`ComputeProductVariantCount returns the number of variants for this template`,
func product_template_ComputeProductVariantCount(rs m.ProductTemplateSet) m.ProductTemplateData {
	return h.ProductTemplate().NewData().
		SetProductVariantCount(rs.ProductVariants().Len())
}

//`ComputeDefaultCode returns the default code for this template`,
func product_template_ComputeDefaultCode(rs m.ProductTemplateSet) m.ProductTemplateData {
	res := h.ProductTemplate().NewData()
	if rs.ProductVariants().Len() == 1 {
		res.SetDefaultCode(rs.ProductVariant().DefaultCode())
	}
	return res
}

//`InverseDefaultCode sets the default code of this template`,
func product_template_InverseDefaultCode(rs m.ProductTemplateSet, code string) {
	if rs.ProductVariants().Len() == 1 {
		rs.ProductVariant().SetDefaultCode(code)
	}
}

//`CheckUom checks that this template's uom is of the same category as the purchase uom`,
func product_template_CheckUom(rs m.ProductTemplateSet) {
	if rs.Uom().IsNotEmpty() && rs.UomPo().IsNotEmpty() && !rs.Uom().Category().Equals(rs.UomPo().Category()) {
		log.Panic(rs.T("Error: The default Unit of Measure and the purchase Unit of Measure must be in the same category."))
	}
}

//`OnchangeUom updates UomPo when uom is changed`,
func product_template_OnchangeUom(rs m.ProductTemplateSet) m.ProductTemplateData {
	res := h.ProductTemplate().NewData()
	if !rs.Uom().IsEmpty() {
		res.SetUomPo(rs.Uom())
	}
	return res
}

//`ResizeImageData returns the given data struct with images set for the different sizes.`,
func product_template_ResizeImageData(set m.ProductTemplateSet, data m.ProductTemplateData) {
	switch {
	case data.Image() != "":
		data.SetImage(b64image.Resize(data.Image(), 1024, 1024, true))
		data.SetImageMedium(b64image.Resize(data.Image(), 128, 128, false))
		data.SetImageSmall(b64image.Resize(data.Image(), 64, 64, false))
	case data.ImageMedium() != "":
		data.SetImage(b64image.Resize(data.ImageMedium(), 1024, 1024, true))
		data.SetImageMedium(b64image.Resize(data.ImageMedium(), 128, 128, true))
		data.SetImageSmall(b64image.Resize(data.ImageMedium(), 64, 64, false))
	case data.ImageSmall() != "":
		data.SetImage(b64image.Resize(data.ImageSmall(), 1024, 1024, true))
		data.SetImageMedium(b64image.Resize(data.ImageSmall(), 128, 128, true))
		data.SetImageSmall(b64image.Resize(data.ImageSmall(), 64, 64, true))
	}
}

func product_template_Create(rs m.ProductTemplateSet, data m.ProductTemplateData) m.ProductTemplateSet {
	rs.ResizeImageData(data)
	template := rs.Super().Create(data)
	if !rs.Env().Context().HasKey("create_product_product") {
		template.WithContext("create_from_tmpl", true).CreateVariants()
	}
	// This is needed to set given values to first variant after creation
	relatedVals := h.ProductTemplate().NewData()
	if data.HasBarcode() {
		relatedVals.SetBarcode(data.Barcode())
	}
	if data.HasDefaultCode() {
		relatedVals.SetDefaultCode(data.DefaultCode())
	}
	if data.HasStandardPrice() {
		relatedVals.SetStandardPrice(data.StandardPrice())
	}
	if data.HasVolume() {
		relatedVals.SetVolume(data.Volume())
	}
	if data.HasWeight() {
		relatedVals.SetWeight(data.Weight())
	}
	template.Write(relatedVals)
	return template
}

func product_template_Write(rs m.ProductTemplateSet, vals m.ProductTemplateData) bool {
	rs.ResizeImageData(vals)
	res := rs.Super().Write(vals)
	if vals.HasAttributeLines() || vals.Active() {
		rs.CreateVariants()
	}
	if vals.HasActive() && !vals.Active() {
		rs.WithContext("active_test", false).ProductVariants().SetActive(vals.Active())
	}
	return res
}

func product_template_Copy(rs m.ProductTemplateSet, overrides m.ProductTemplateData) m.ProductTemplateSet {
	rs.EnsureOne()
	if !overrides.HasName() {
		overrides.SetName(rs.T("%s (Copy)", rs.Name()))
	}
	return rs.Super().Copy(overrides)
}

func product_template_NameGet(rs m.ProductTemplateSet) string {
	return h.ProductProduct().NewSet(rs.Env()).NameFormat(rs.Name(), rs.DefaultCode())
}

func product_template_SearchByName(rs m.ProductTemplateSet, name string, op operator.Operator, additionalCond q.ProductTemplateCondition, limit int) m.ProductTemplateSet {
	// Only use the product.product heuristics if there is a search term and the domain
	// does not specify a match on `product.template` IDs.
	if name == "" {
		return rs.Super().SearchByName(name, op, additionalCond, limit)
	}
	if additionalCond.HasField(h.ProductTemplate().Fields().ID()) {
		return rs.Super().SearchByName(name, op, additionalCond, limit)
	}

	templates := h.ProductTemplate().NewSet(rs.Env())
	if limit == 0 {
		limit = 100
	}
	for templates.Len() > limit {
		var prodCond q.ProductProductCondition
		if !templates.IsEmpty() {
			prodCond = q.ProductProduct().ProductTmpl().In(templates)
		}
		products := h.ProductProduct().NewSet(rs.Env()).SearchByName(name, op,
			prodCond.And().ProductTmplFilteredOn(additionalCond), limit)
		for _, prod := range products.Records() {
			templates = templates.Union(prod.ProductTmpl())
		}
		if products.IsEmpty() {
			break
		}
	}
	return templates
}

//`PriceCompute returns the price field defined by priceType in the given uom and currency
//		for the given company.`,
func product_template_PriceCompute(rs m.ProductTemplateSet, priceType models.FieldNamer, uom m.ProductUomSet, currency m.CurrencySet, company m.CompanySet) float64 {
	rs.EnsureOne()
	template := rs
	if priceType == q.ProductTemplate().StandardPrice() {
		// StandardPrice field can only be seen by users in base.group_user
		// Thus, in order to compute the sale price from the cost for users not in this group
		// We fetch the standard price as the superuser
		if company.IsEmpty() {
			company = h.User().NewSet(rs.Env()).CurrentUser().Company()
			if rs.Env().Context().HasKey("force_company") {
				company = h.Company().Browse(rs.Env(), []int64{rs.Env().Context().GetInteger("force_company")})
			}
		}
		template = rs.WithContext("force_company", company.ID()).Sudo()
	}
	price := template.Get(priceType.String()).(float64)
	if !uom.IsEmpty() {
		price = template.Uom().ComputePrice(price, uom)
	}
	// Convert from current user company currency to asked one
	// This is right cause a field cannot be in more than one currency
	if !currency.IsEmpty() {
		price = template.Currency().Compute(price, currency, true)
	}
	return price
}

//`CreateVariants`,
func product_template_CreateVariants(rs m.ProductTemplateSet) {
	for _, tmpl := range rs.WithContext("active_test", false).Records() {
		// adding an attribute with only one value should not recreate product
		// write this attribute on every product to make sure we don't lose them
		variantAloneLines := tmpl.AttributeLines().Filtered(func(r m.ProductAttributeLineSet) bool {
			return r.Attribute().CreateVariant() && r.Values().Len() == 1
		})
		for _, v := range variantAloneLines.Records() {
			value := v.Values()
			updatedProducts := tmpl.ProductVariants().Filtered(func(r m.ProductProductSet) bool {
				prodAttrs := h.ProductAttribute().NewSet(rs.Env())
				for _, pa := range r.AttributeValues().Records() {
					prodAttrs = prodAttrs.Union(pa.Attribute())
				}
				return value.Attribute().Intersect(prodAttrs).IsEmpty()
			})
			for _, prod := range updatedProducts.Records() {
				prod.SetAttributeValues(prod.AttributeValues().Union(value))
			}
		}

		// list of values combination
		var existingVariants []m.ProductAttributeValueSet
		for _, prod := range tmpl.ProductVariants().Records() {
			prodVariant := h.ProductAttributeValue().NewSet(rs.Env())
			for _, attrVal := range prod.AttributeValues().Records() {
				if attrVal.Attribute().CreateVariant() {
					prodVariant = prodVariant.Union(attrVal)
				}
			}
			existingVariants = append(existingVariants, prodVariant)
		}
		var matrixValues []m.ProductAttributeValueSet
		for _, attrLine := range tmpl.AttributeLines().Records() {
			if !attrLine.Attribute().CreateVariant() {
				continue
			}
			matrixValues = append(matrixValues, attrLine.Values())
		}
		var variantMatrix []m.ProductAttributeValueSet
		if len(matrixValues) > 0 {
			variantMatrix = matrixValues[0].CartesianProduct(matrixValues[1:]...)
		} else {
			variantMatrix = []m.ProductAttributeValueSet{h.ProductAttributeValue().NewSet(rs.Env())}
		}

		var toCreateVariants []m.ProductAttributeValueSet
		for _, mVariant := range variantMatrix {
			var exists bool
			for _, eVariant := range existingVariants {
				if mVariant.Equals(eVariant) {
					exists = true
					break
				}
			}
			if !exists {
				toCreateVariants = append(toCreateVariants, mVariant)
			}
		}

		// check product
		variantsToActivate := h.ProductProduct().NewSet(rs.Env())
		variantsToUnlink := h.ProductProduct().NewSet(rs.Env())
		for _, product := range tmpl.ProductVariants().Records() {
			tcAttrs := h.ProductAttributeValue().NewSet(rs.Env())
			for _, attrVal := range product.AttributeValues().Records() {
				if !attrVal.Attribute().CreateVariant() {
					continue
				}
				tcAttrs = tcAttrs.Union(attrVal)
			}
			var inMatrix bool
			for _, mVariant := range variantMatrix {
				if tcAttrs.Equals(mVariant) {

					inMatrix = true
					break
				}
			}
			switch {
			case inMatrix && !product.Active():
				variantsToActivate = variantsToActivate.Union(product)
			case !inMatrix:
				variantsToUnlink = variantsToUnlink.Union(product)
			}
		}
		if !variantsToActivate.IsEmpty() {
			variantsToActivate.SetActive(true)
		}

		// create new product
		for _, variants := range toCreateVariants {
			h.ProductProduct().Create(rs.Env(), h.ProductProduct().NewData().
				SetProductTmpl(tmpl).
				SetAttributeValues(variants))
		}

		// unlink or inactive product
		if !variantsToUnlink.IsEmpty() {
			variantsToUnlink.UnlinkOrDeactivate()
		}

	}
}

func init() {

	models.NewModel("ProductTemplate")

	h.ProductTemplate().AddFields(fields_ProductTemplate)
	h.ProductTemplate().SetDefaultOrder("Name")

	h.ProductTemplate().NewMethod("ComputeProductVariant", product_template_ComputeProductVariant)
	h.ProductTemplate().NewMethod("ComputeCurrency", product_template_ComputeCurrency)
	h.ProductTemplate().NewMethod("ComputeTemplatePrice", product_template_ComputeTemplatePrice)
	h.ProductTemplate().NewMethod("InverseTemplatePrice", product_template_InverseTemplatePrice)
	h.ProductTemplate().NewMethod("ComputeStandardPrice", product_template_ComputeStandardPrice)
	h.ProductTemplate().NewMethod("InverseStandardPrice", product_template_InverseStandardPrice)
	h.ProductTemplate().NewMethod("ComputeVolume", product_template_ComputeVolume)

	h.ProductTemplate().NewMethod("InverseVolume", product_template_InverseVolume)
	h.ProductTemplate().NewMethod("ComputeWeight", product_template_ComputeWeight)
	h.ProductTemplate().NewMethod("InverseWeight", product_template_InverseWeight)

	h.ProductTemplate().NewMethod("ComputeProductVariantCount", product_template_ComputeProductVariantCount)

	h.ProductTemplate().NewMethod("ComputeDefaultCode", product_template_ComputeDefaultCode)
	h.ProductTemplate().NewMethod("InverseDefaultCode", product_template_InverseDefaultCode)
	h.ProductTemplate().NewMethod("CheckUom", product_template_CheckUom)
	h.ProductTemplate().NewMethod("OnchangeUom", product_template_OnchangeUom)
	h.ProductTemplate().NewMethod("ResizeImageData", product_template_ResizeImageData)

	h.ProductTemplate().Methods().Create().Extend(product_template_Create)
	h.ProductTemplate().Methods().Write().Extend(product_template_Write)
	h.ProductTemplate().Methods().Copy().Extend(product_template_Copy)
	h.ProductTemplate().Methods().SearchByName().Extend(product_template_SearchByName)
	h.ProductTemplate().Methods().NameGet().Extend(product_template_NameGet)

	h.ProductTemplate().NewMethod("PriceCompute", product_template_PriceCompute)

	h.ProductTemplate().NewMethod("CreateVariants", product_template_CreateVariants)

}
