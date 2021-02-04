// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package product

import (
	"fmt"
	"github.com/gleke/hexya/src/models/fields"
	"log"
	"strings"

	"github.com/gleke/decimalPrecision"
	"github.com/gleke/hexya/src/models"
	"github.com/gleke/hexya/src/models/operator"
	"github.com/gleke/pool/h"
	"github.com/gleke/pool/m"
	"github.com/gleke/pool/q"
)

var fields_ProductAttribute = map[string]models.FieldDefinition{
	"Name": fields.Char{Required: true, Translate: true},
	"Values": fields.One2Many{RelationModel: h.ProductAttributeValue(), ReverseFK: "Attribute",
		JSON: "value_ids", Copy: true},
	"Sequence": fields.Integer{Help: "Determine the display order"},
	"AttributeLines": fields.One2Many{String: "Lines", RelationModel: h.ProductAttributeLine(),
		ReverseFK: "Attribute", JSON: "attribute_line_ids"},
	"CreateVariant": fields.Boolean{Default: models.DefaultValue(true),
		Help: "Check this if you want to create multiple variants for this attribute."},
}

//`ComputePriceExtra returns the price extra for this attribute for the product
//		template passed as 'active_id' in the context. Returns 0 if there is not 'active_id'.`,
func product_attribute_value_ComputePriceExtra(rs m.ProductAttributeValueSet) m.ProductAttributeValueData {
	var priceExtra float64
	if rs.Env().Context().HasKey("active_id") {
		productTmpl := h.ProductTemplate().Browse(rs.Env(), []int64{rs.Env().Context().GetInteger("active_id")})
		price := rs.Prices().Search(q.ProductAttributePrice().ProductTmpl().Equals(productTmpl))
		priceExtra = price.PriceExtra()
	}
	return h.ProductAttributeValue().NewData().SetPriceExtra(priceExtra)
}

//`InversePriceExtra sets the price extra based on the product
//		template passed as 'active_id'. Does nothing if there is not 'active_id'.`,
func product_attribute_value_InversePriceExtra(rs m.ProductAttributeValueSet, value float64) {
	if !rs.Env().Context().HasKey("active_id") {
		return
	}
	productTmpl := h.ProductTemplate().Browse(rs.Env(), []int64{rs.Env().Context().GetInteger("active_id")})
	prices := h.ProductAttributePrice().Search(rs.Env(),
		q.ProductAttributePrice().Value().In(rs).And().ProductTmpl().Equals(productTmpl))
	if !prices.IsEmpty() {
		prices.SetPriceExtra(value)
		return
	}
	updated := h.ProductAttributeValue().NewSet(rs.Env())
	for _, price := range prices.Records() {
		updated = updated.Union(price.Value())
	}
	for _, val := range rs.Subtract(updated).Records() {
		h.ProductAttributePrice().Create(rs.Env(), h.ProductAttributePrice().NewData().
			SetProductTmpl(productTmpl).
			SetValue(val).
			SetPriceExtra(value))
	}
}

func product_attribute_value_NameGet(rs m.ProductAttributeValueSet) string {
	if rs.Env().Context().HasKey("show_attribute") && !rs.Env().Context().GetBool("show_attribute") {
		return rs.Super().NameGet()
	}
	return fmt.Sprintf("%s: %s", rs.Attribute().Name(), rs.Name())
}

func product_attribute_value_Unlink(rs m.ProductAttributeValueSet) int64 {
	linkedProducts := h.ProductProduct().NewSet(rs.Env()).WithContext("active_test", false).Search(
		q.ProductProduct().AttributeValues().In(rs))
	if !linkedProducts.IsEmpty() {
		log.Panic(rs.T(`The operation cannot be completed:
You are trying to delete an attribute value with a reference on a product variant.`))
	}
	return rs.Super().Unlink()
}

//`VariantName returns a comma separated list of this product's
//		attributes values of the given variable attributes'`,
func product_attribute_value_VariantName(rs m.ProductAttributeValueSet, variableAttribute m.ProductAttributeSet) string {
	var names []string
	rSet := rs.Sorted(func(rs1, rs2 m.ProductAttributeValueSet) bool {
		return rs1.Attribute().Name() < rs2.Attribute().Name()
	})
	for _, attrValue := range rSet.Records() {
		if attrValue.Attribute().Intersect(variableAttribute).IsEmpty() {
			continue
		}
		names = append(names, attrValue.Name())
	}
	return strings.Join(names, ", ")
}

var fields_ProductAttributeValue = map[string]models.FieldDefinition{
	"Name":     fields.Char{String: "Value", Required: true, Translate: true},
	"Sequence": fields.Integer{Help: "Determine the display order"},
	"Attribute": fields.Many2One{RelationModel: h.ProductAttribute(), OnDelete: models.Cascade,
		Required: true},
	"Products": fields.Many2One{String: "Variants", RelationModel: h.ProductProduct(),
		JSON: "product_ids"},
	"PriceExtra": fields.Float{String: "Attribute Price Extra",
		Compute: h.ProductAttributeValue().Methods().ComputePriceExtra(),
		Inverse: h.ProductAttributeValue().Methods().InversePriceExtra(),
		Default: models.DefaultValue(0), Digits: decimalPrecision.GetPrecision("Product Price"),
		Help: "Price Extra: Extra price for the variant with this attribute value on sale price. eg. 200 price extra, 1000 + 200 = 1200."},
	"Prices": fields.One2Many{String: "Attribute Prices", RelationModel: h.ProductAttributePrice(),
		ReverseFK: "Value", JSON: "price_ids", ReadOnly: true},
}
var fields_ProductAttributePrice = map[string]models.FieldDefinition{
	"ProductTmpl": fields.Many2One{String: "Product Template", RelationModel: h.ProductTemplate(),
		OnDelete: models.Cascade, Required: true},
	"Value": fields.Many2One{String: "Product Attribute Value", RelationModel: h.ProductAttributeValue(),
		OnDelete: models.Cascade, Required: true},
	"PriceExtra": fields.Float{String: "Price Extra", Digits: decimalPrecision.GetPrecision("Product Price")},
}
var fields_ProductAttributeLine = map[string]models.FieldDefinition{
	"ProductTmpl": fields.Many2One{String: "Product Template", RelationModel: h.ProductTemplate(),
		OnDelete: models.Cascade, Required: true},
	"Attribute": fields.Many2One{RelationModel: h.ProductAttribute(),
		OnDelete: models.Restrict, Required: true,
		Constraint: h.ProductAttributeLine().Methods().CheckValidAttribute()},
	"Values": fields.Many2One{String: "Attribute Values", RelationModel: h.ProductAttributeValue(),
		JSON: "value_ids", Constraint: h.ProductAttributeLine().Methods().CheckValidAttribute()},
	"Name": fields.Char{Compute: h.ProductAttributeLine().Methods().ComputeName(), Stored: true,
		Depends: []string{"Attribute", "Attribute.Name", "Values", "Values.Name"}},
}

//`Name returns a standard name with the attribute name and the values for searching`,
func product_attribute_line_ComputeName(rs m.ProductAttributeLineSet) m.ProductAttributeLineData {
	var values []string
	for _, value := range rs.Values().Records() {
		values = append(values, value.Name())
	}
	return h.ProductAttributeLine().NewData().
		SetName(rs.Attribute().Name() + ": " + strings.Join(values, ", "))
}

//`CheckValidAttribute check that attributes values are valid for the given attributes.`,
func product_attribute_line_CheckValidAttribute(rs m.ProductAttributeLineSet) {
	for _, line := range rs.Records() {
		if !line.Values().Subtract(line.Attribute().Values()).IsEmpty() {
			log.Panic(rs.T("Error ! You cannot use this attribute with the following value."))
		}
	}
}

func product_attribute_line_NameGet(rs m.ProductAttributeLineSet) string {
	return rs.Attribute().NameGet()
}

func product_attribute_line_SearchByName(rs m.ProductAttributeLineSet, name string, op operator.Operator, additionalCond q.ProductAttributeLineCondition, limit int) m.ProductAttributeLineSet {
	// TDE FIXME: currently overriding the domain; however as it includes a
	// search on a m2o and one on a m2m, probably this will quickly become
	// difficult to compute - check if performance optimization is required
	if name != "" && op.IsPositive() {
		additionalCond = q.ProductAttributeLine().Attribute().AddOperator(op, name).
			Or().Values().AddOperator(op, name)
	}
	return rs.Super().SearchByName(name, op, additionalCond, limit)
}
func init() {

	models.NewModel("ProductAttribute")
	h.ProductAttribute().SetDefaultOrder("Sequence", "Name")

	h.ProductAttribute().AddFields(fields_ProductAttribute)
	models.NewModel("ProductAttributeValue")
	h.ProductAttributeValue().SetDefaultOrder("Sequence")

	h.ProductAttributeValue().AddFields(fields_ProductAttributeValue)

	// TODO Convert to constrains method
	//h.ProductAttributeValue().AddSQLConstraint("ValueCompanyUniq", "unique (name,attribute_id)", "This attribute value already exists !")
	h.ProductAttributeValue().NewMethod("ComputePriceExtra", product_attribute_value_ComputePriceExtra)
	h.ProductAttributeValue().NewMethod("InversePriceExtra", product_attribute_value_InversePriceExtra)
	h.ProductAttributeValue().NewMethod("VariantName", product_attribute_value_VariantName)

	h.ProductAttributeValue().Methods().NameGet().Extend(product_attribute_value_NameGet)
	h.ProductAttributeValue().Methods().Unlink().Extend(product_attribute_value_Unlink)

	models.NewModel("ProductAttributePrice")
	h.ProductAttributePrice().AddFields(fields_ProductAttributePrice)

	models.NewModel("ProductAttributeLine")

	h.ProductAttributeLine().AddFields(fields_ProductAttributeLine)

	h.ProductAttributeLine().NewMethod("ComputeName", product_attribute_ine_ComputeName)
	h.ProductAttributeLine().NewMethod("CheckValidAttribute", product_attribute_line_CheckValidAttribute)
	h.ProductAttributeLine().NewMethod("VariantName", product_attribute_line_ComputeName)

	h.ProductAttributeLine().Methods().NameGet().Extend(product_attribute_line_NameGet)
	h.ProductAttributeLine().Methods().SearchByName().Extend(product_attribute_line_SearchByName)

}
