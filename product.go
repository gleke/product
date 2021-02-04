// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package product

import (
	"fmt"
	"github.com/gleke/hexya/src/models/fields"
	"log"
	"regexp"
	"strings"

	"github.com/gleke/base"
	"github.com/gleke/decimalPrecision"
	"github.com/gleke/hexya/src/actions"
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

var fields_ProductCategory = map[string]models.FieldDefinition{
	"Name": fields.Char{Index: true, Required: true, Translate: true},
	"Parent": fields.Many2One{String: "Parent Category", RelationModel: h.ProductCategory(), Index: true,
		OnDelete: models.Cascade, Constraint: h.ProductCategory().Methods().CheckCategoryRecursion()},
	"Children": fields.One2Many{String: "Child Categories", RelationModel: h.ProductCategory(),
		ReverseFK: "Parent", JSON: "child_id"},
	"Type": fields.Selection{String: "Category Type", Selection: types.Selection{"view": "View", "normal": "Normal"},
		Default: models.DefaultValue("normal"), Help: "A category of the view type is a virtual category that can be used as the parent of another category to create a hierarchical structure."},
	"Products": fields.One2Many{RelationModel: h.ProductTemplate(), ReverseFK: "Category"},
	"ProductCount": fields.Integer{String: "# Products", Compute: h.ProductCategory().Methods().ComputeProductCount(),
		Help:    "The number of products under this category (Does not consider the children categories)",
		Depends: []string{"Products"}, GoType: new(int)},
}

//`CheckCategoryRecursion panics if there is a recursion in the category tree.`,
func product_category_CheckCategoryRecursion(rs m.ProductCategorySet) {
	if !rs.CheckRecursion() {
		panic(rs.T("Error ! You cannot create recursive categories."))
	}
}

func product_category_NameGet(rs m.ProductCategorySet) string {
	var names []string
	for current := rs; !current.IsEmpty(); current = current.Parent() {
		names = append([]string{current.Name()}, names...)
	}
	return strings.Join(names, " / ")
}

func product_category_SearchByName(rs m.ProductCategorySet, name string, op operator.Operator, additionalCond q.ProductCategoryCondition, limit int) m.ProductCategorySet {
	if name == "" {
		return rs.Super().SearchByName(name, op, additionalCond, limit)
	}
	// Be sure name_search is symetric to name_get
	categoryNames := strings.Split(name, " / ")
	child := categoryNames[len(categoryNames)-1]
	cond := q.ProductCategory().Name().AddOperator(op, child)
	var categories m.ProductCategorySet
	if len(categoryNames) > 1 {
		parents := rs.SearchByName(strings.Join(categoryNames[:len(categoryNames)-1], " / "), operator.IContains, additionalCond, limit)
		if op.IsNegative() {
			categories = h.ProductCategory().Search(rs.Env(), q.ProductCategory().ID().NotIn(parents.Ids()))
			cond = cond.Or().Parent().In(categories)
		} else {
			cond = cond.And().Parent().In(parents)
		}
		for i := 1; i < len(categoryNames); i++ {
			if op.IsNegative() {
				cond = cond.AndCond(q.ProductCategory().Name().AddOperator(op, strings.Join(categoryNames[len(categoryNames)-1-i:], " / ")))
			} else {
				cond = cond.OrCond(q.ProductCategory().Name().AddOperator(op, strings.Join(categoryNames[len(categoryNames)-1-i:], " / ")))
			}
		}
	}
	return h.ProductCategory().Search(rs.Env(), cond.AndCond(additionalCond))
}

var fields_ProductPriceHistory = map[string]models.FieldDefinition{
	"Company": fields.Many2One{RelationModel: h.Company(),
		Default: func(env models.Environment) interface{} {
			if env.Context().HasKey("force_company") {

				return h.Company().Browse(env, []int64{env.Context().GetInteger("force_company")})
			}
			currentUser := h.User().NewSet(env).CurrentUser()
			return currentUser.Company()
		}, Required: true},
	"Product": fields.Many2One{RelationModel: h.ProductProduct(), JSON: "product_id",
		OnDelete: models.Cascade, Required: true},
	"Datetime": fields.DateTime{String: "Date", Default: func(env models.Environment) interface{} {
		return dates.Now()
	}},
	"Cost": fields.Float{String: "Cost", Digits: decimalPrecision.GetPrecision("Product Price")},
}
var fields_ProductProduct = map[string]models.FieldDefinition{
	"Price": fields.Float{Compute: h.ProductProduct().Methods().ComputeProductPrice(),
		Digits:  decimalPrecision.GetPrecision("Product Price"),
		Inverse: h.ProductProduct().Methods().InverseProductPrice()},
	"PriceExtra": fields.Float{String: "Variant Price Extra",
		Compute: h.ProductProduct().Methods().ComputeProductPriceExtra(),
		Depends: []string{"AttributeValues", "AttributeValues.Prices", "AttributeValues.Prices.PriceExtra", "AttributeValues.Prices.ProductTmpl"},
		Digits:  decimalPrecision.GetPrecision("Product Price"),
		Help:    "This is the sum of the extra price of all attributes"},
	"LstPrice": fields.Float{String: "Sale Price",
		Compute: h.ProductProduct().Methods().ComputeProductLstPrice(),
		Depends: []string{"ListPrice", "PriceExtra"},
		Digits:  decimalPrecision.GetPrecision("Product Price"),
		Inverse: h.ProductProduct().Methods().InverseProductLstPrice(),
		Help:    "The sale price is managed from the product template. Click on the 'Variant Prices' button to set the extra attribute prices."},
	"DefaultCode": fields.Char{String: "Internal Reference", Index: true},
	"Code": fields.Char{String: "Internal Reference",
		Compute: h.ProductProduct().Methods().ComputeProductCode(), Depends: []string{""}},
	"PartnerRef": fields.Char{String: "Customer Ref",
		Compute: h.ProductProduct().Methods().ComputePartnerRef(), Depends: []string{""}},
	"Active": fields.Boolean{String: "Active",
		Default: models.DefaultValue(true), Required: true,
		Help: "If unchecked, it will allow you to hide the product without removing it."},
	"ProductTmpl": fields.Many2One{String: "Product Template", RelationModel: h.ProductTemplate(),
		Index: true, OnDelete: models.Cascade, Required: true, Embed: true},
	"Barcode": fields.Char{String: "Barcode", NoCopy: true, /*Unique: true,*/
		Help: "International Article Number used for product identification."},
	"AttributeValues": fields.Many2Many{String: "Attributes", RelationModel: h.ProductAttributeValue(),
		JSON:       "attribute_value_ids", /*, OnDelete: models.Restrict*/
		Constraint: h.ProductProduct().Methods().CheckAttributeValueIds()},
	"ImageVariant": fields.Binary{String: "Variant Image",
		Help: "This field holds the image used as image for the product variant, limited to 1024x1024px."},
	"Image": fields.Binary{String: "Big-sized image",
		Compute: h.ProductProduct().Methods().ComputeImages(),
		Depends: []string{"ImageVariant", "ProductTmpl", "ProductTmpl.Image"},
		Inverse: h.ProductProduct().Methods().InverseImageValue(),
		Help: `Image of the product variant (Big-sized image of product template if false). It is automatically
resized as a 1024x1024px image, with aspect ratio preserved.`},
	"ImageSmall": fields.Binary{String: "Small-sized image",
		Compute: h.ProductProduct().Methods().ComputeImages(),
		Depends: []string{"ImageVariant", "ProductTmpl", "ProductTmpl.Image"},
		Inverse: h.ProductProduct().Methods().InverseImageValue(),
		Help:    "Image of the product variant (Small-sized image of product template if false)."},
	"ImageMedium": fields.Binary{String: "Medium-sized image",
		Compute: h.ProductProduct().Methods().ComputeImages(),
		Depends: []string{"ImageVariant", "ProductTmpl", "ProductTmpl.Image"},
		Inverse: h.ProductProduct().Methods().InverseImageValue(),
		Help:    "Image of the product variant (Medium-sized image of product template if false)."},
	"StandardPrice": fields.Float{String: "Cost", Contexts: base.CompanyDependent,
		Digits: decimalPrecision.GetPrecision("Product Price"),
		InvisibleFunc: func(env models.Environment) (bool, models.Conditioner) {
			return !security.Registry.HasMembership(env.Uid(), base.GroupUser), nil
		},
		Help: `Cost of the product template used for standard stock valuation in accounting and used as a
base price on purchase orders. Expressed in the default unit of measure of the product.`},
	"Volume": fields.Float{Help: "The volume in m3."},
	"Weight": fields.Float{Digits: decimalPrecision.GetPrecision("Stock Weight"),
		Help: "The weight of the contents in Kg, not including any packaging, etc."},
	"PricelistItems": fields.Many2Many{RelationModel: h.ProductPricelistItem(),
		JSON: "pricelist_item_ids", Compute: h.ProductProduct().Methods().GetPricelistItems()},
}

//`ComputeProductPrice computes the price of this product based on the given context keys:
//
//		- 'partner' => int64 (id of the partner)
//		- 'pricelist' => int64 (id of the price list)
//		- 'quantity' => float64`,
func product_product_ComputeProductPrice(rs m.ProductProductSet) m.ProductProductData {
	if !rs.Env().Context().HasKey("pricelist") {
		return h.ProductProduct().NewData()
	}
	priceListID := rs.Env().Context().GetInteger("pricelist")
	priceList := h.ProductPricelist().Browse(rs.Env(), []int64{priceListID})
	if priceList.IsEmpty() {
		return h.ProductProduct().NewData()
	}
	quantity := rs.Env().Context().GetFloat("quantity")
	if quantity == 0 {
		quantity = 1
	}
	partnerID := rs.Env().Context().GetInteger("partner")
	partner := h.Partner().Browse(rs.Env(), []int64{partnerID})
	return h.ProductProduct().NewData().SetPrice(
		priceList.GetProductPrice(rs, quantity, partner, dates.Date{}, h.ProductUom().NewSet(rs.Env())))
}

//`InverseProductPrice updates ListPrice from the given Price`,
func product_product_InverseProductPrice(rs m.ProductProductSet, price float64) {
	if rs.Env().Context().HasKey("uom") {
		price = h.ProductUom().Browse(rs.Env(), []int64{rs.Env().Context().GetInteger("uom")}).ComputePrice(price, rs.Uom())
	}
	price -= rs.PriceExtra()
	rs.SetListPrice(price)
}

//`InverseProductLstPrice updates ListPrice from the given LstPrice`,
func product_product_InverseProductLstPrice(rs m.ProductProductSet, price float64) {
	if rs.Env().Context().HasKey("uom") {
		price = h.ProductUom().Browse(rs.Env(), []int64{rs.Env().Context().GetInteger("uom")}).ComputePrice(price, rs.Uom())
	}
	price -= rs.PriceExtra()
	rs.SetListPrice(price)
}

//`ComputeProductPriceExtra computes the price extra of this product by suming the extras of each attribute`,
func product_product_ComputeProductPriceExtra(rs m.ProductProductSet) m.ProductProductData {
	var priceExtra float64
	for _, attributeValue := range rs.AttributeValues().Records() {
		for _, attributePrice := range attributeValue.Prices().Records() {
			if attributePrice.ProductTmpl().Equals(rs.ProductTmpl()) {
				priceExtra += attributePrice.PriceExtra()
			}
		}
	}
	return h.ProductProduct().NewData().SetPriceExtra(priceExtra)
}

//`ComputeProductLstPrice computes the LstPrice from the ListPrice and the extras`,
func product_product_ComputeProductLstPrice(rs m.ProductProductSet) m.ProductProductData {
	listPrice := rs.ListPrice()
	if rs.Env().Context().HasKey("uom") {
		toUoM := h.ProductUom().Browse(rs.Env(), []int64{rs.Env().Context().GetInteger("uom")})
		listPrice = rs.Uom().ComputePrice(listPrice, toUoM)
	}
	return h.ProductProduct().NewData().SetLstPrice(listPrice + rs.PriceExtra())
}

//`ComputeProductCode computes the product code based on the context:
//		- 'partner_id' => int64 (id of the considered partner)`,
func product_product_ComputeProductCode(rs m.ProductProductSet) m.ProductProductData {
	var code string
	for _, supplierInfo := range rs.Sellers().Records() {
		if supplierInfo.Name().ID() == rs.Env().Context().GetInteger("partner_id") {
			code = supplierInfo.ProductCode()
			break
		}
	}
	if code == "" {
		code = rs.DefaultCode()
	}
	return h.ProductProduct().NewData().SetCode(code)
}

//`ComputePartnerRef computes the product's reference (i.e. "[code] description") based on the context:
//		- 'partner_id' => int64 (id of the considered partner)`,
func product_product_ComputePartnerRef(rs m.ProductProductSet) m.ProductProductData {
	var code, productName string
	for _, supplierInfo := range rs.Sellers().Records() {
		if supplierInfo.Name().ID() == rs.Env().Context().GetInteger("partner_id") {
			code = supplierInfo.ProductCode()
			productName = supplierInfo.ProductName()
			break
		}
	}
	if code == "" {
		code = rs.DefaultCode()
	}
	if productName == "" {
		productName = rs.Name()
	}
	return h.ProductProduct().NewData().SetPartnerRef(rs.NameFormat(productName, code))
}

//`ComputeImages computes the images in different sizes.`,
func product_product_ComputeImages(rs m.ProductProductSet) m.ProductProductData {
	var imageMedium, imageSmall, image string
	if rs.Env().Context().GetBool("bin_size") {
		imageMedium = rs.ImageVariant()
		imageSmall = rs.ImageVariant()
		image = rs.ImageVariant()
	} else {
		imageMedium = b64image.Resize(rs.ImageVariant(), 128, 128, true)
		imageSmall = b64image.Resize(rs.ImageVariant(), 64, 64, false)
		image = b64image.Resize(rs.ImageVariant(), 1024, 1024, true)
	}
	if imageMedium == "" {
		imageMedium = rs.ProductTmpl().ImageMedium()
	}
	if imageSmall == "" {
		imageSmall = rs.ProductTmpl().ImageSmall()
	}
	if image == "" {
		image = rs.ProductTmpl().Image()
	}
	return h.ProductProduct().NewData().
		SetImageSmall(imageSmall).
		SetImageMedium(imageMedium).
		SetImage(image)
}

//`InverseImageValue sets all images from the given image`,
func product_product_InverseImageValue(rs m.ProductProductSet, image string) {
	image = b64image.Resize(image, 1024, 1024, true)
	if rs.ProductTmpl().Image() == "" {
		rs.ProductTmpl().SetImage(image)
		return
	}
	rs.SetImageVariant(image)
}

//`GetPricelistItems returns all price list items for this product`,
func product_product_GetPricelistItems(rs m.ProductProductSet) m.ProductProductData {
	rs.EnsureOne()
	priceListItems := h.ProductPricelistItem().Search(rs.Env(),
		q.ProductPricelistItem().Product().Equals(rs).Or().ProductTmpl().Equals(rs.ProductTmpl()))
	return h.ProductProduct().NewData().SetPricelistItems(priceListItems)
}

//`CheckAttributeValueIds checks that we do not have more than one value per attribute.`,
func product_product_CheckAttributeValueIds(rs m.ProductProductSet) {
	attributes := h.ProductAttribute().NewSet(rs.Env())
	for _, value := range rs.AttributeValues().Records() {
		if !value.Attribute().Intersect(attributes).IsEmpty() {
			log.Panic(rs.T("Error! It is not allowed to choose more than one value for a given attribute."))
		}
		attributes = attributes.Union(value.Attribute())
	}
}

//`OnchangeUom process UI triggers when changing th UoM`,
func product_product_OnchangeUom(rs m.ProductProductSet) m.ProductProductData {
	if !rs.Uom().IsEmpty() && !rs.UomPo().IsEmpty() && !rs.Uom().Category().Equals(rs.UomPo().Category()) {
		return h.ProductProduct().NewData().SetUomPo(rs.Uom())
	}
	return h.ProductProduct().NewData()
}

func product_product_Create(rs m.ProductProductSet, data m.ProductProductData) m.ProductProductSet {
	product := rs.WithContext("create_product_product", true).Super().Create(data)
	// When a unique variant is created from tmpl then the standard price is set by DefineStandardPrice
	if !rs.Env().Context().HasKey("create_from_tmpl") && product.ProductTmpl().ProductVariants().Len() == 1 {
		product.DefineStandardPrice(data.StandardPrice())
	}
	return product
}

func product_product_Write(rs m.ProductProductSet, data m.ProductProductData) bool {
	// Store the standard price change in order to be able to retrieve the cost of a product for a given date
	res := rs.Super().Write(data)
	if data.HasStandardPrice() {
		rs.DefineStandardPrice(data.StandardPrice())
	}
	return res
}

//h.ProductProduct().Methods().Unlink().Extend("",
func product_product_Unlink(rs m.ProductProductSet) int64 {
	unlinkProducts := h.ProductProduct().NewSet(rs.Env())
	unlinkTemplates := h.ProductTemplate().NewSet(rs.Env())
	for _, product := range rs.Records() {
		// Check if the product is last product of this template
		otherProducts := h.ProductProduct().Search(rs.Env(),
			q.ProductProduct().ProductTmpl().Equals(product.ProductTmpl()).And().ID().NotEquals(product.ID()))
		if otherProducts.IsEmpty() {
			unlinkTemplates = unlinkTemplates.Union(product.ProductTmpl())
		}
		unlinkProducts = unlinkProducts.Union(product)
	}
	res := unlinkProducts.Super().Unlink()
	// delete templates after calling super, as deleting template could lead to deleting
	// products due to ondelete='cascade'
	unlinkTemplates.Unlink()
	return res
}

//`UnlinkOrDeactivate tries to unlink this product. If it fails, it simply deactivate it.`,
func product_product_UnlinkOrDeactivate(rs m.ProductProductSet) {
	defer func() {
		if r := recover(); r != nil {
			rs.SetActive(false)
		}
	}()
	rs.Unlink()
}

func product_product_Copy(rs m.ProductProductSet, overrides m.ProductProductData) m.ProductProductSet {
	switch {
	case rs.Env().Context().HasKey("variant"):
		// if we copy a variant or create one, we keep the same template
		overrides.SetProductTmpl(rs.ProductTmpl())
	case !overrides.HasName():
		overrides.SetName(rs.Name())
	}
	return rs.Super().Copy(overrides)
}

func product_product_Search(rs m.ProductProductSet, cond q.ProductProductCondition) m.ProductProductSet {
	// FIXME: strange...
	if categID := rs.Env().Context().GetInteger("search_default_category_id"); categID != 0 {
		categ := h.ProductCategory().Browse(rs.Env(), []int64{categID})
		cond = cond.AndCond(q.ProductProduct().Category().ChildOf(categ))
	}
	return rs.Super().Search(cond)
}

//`NameFormat formats a product name string from the given arguments`,
func product_product_NameFormat(rs m.ProductProductSet, name, code string) string {
	if code == "" ||
		(rs.Env().Context().HasKey("display_default_code") && !rs.Env().Context().GetBool("display_default_code")) {
		return name
	}
	return fmt.Sprintf("[%s] %s", code, name)
}

func product_product_NameGet(rs m.ProductProductSet) string {
	/*
	   def _name_get(d):
	       name = d.get('name', '')
	       code = self._context.get('display_default_code', True) and d.get('default_code', False) or False
	       if code:
	           name = '[%s] %s' % (code,name)
	       return (d['id'], name)

	   partner_id = self._context.get('partner_id')
	   if partner_id:
	       partner_ids = [partner_id, self.env['res.partner'].browse(partner_id).commercial_partner_id.id]
	   else:
	       partner_ids = []

	   # all user don't have access to seller and partner
	   # check access and use superuser
	   self.check_access_rights("read")
	   self.check_access_rule("read")

	   result = []
	   for product in self.sudo():
	       # display only the attributes with multiple possible values on the template
	       variable_attributes = product.attribute_line_ids.filtered(lambda l: len(l.value_ids) > 1).mapped('attribute_id')
	       variant = product.attribute_value_ids._variant_name(variable_attributes)

	       name = variant and "%s (%s)" % (product.name, variant) or product.name
	       sellers = []
	       if partner_ids:
	           sellers = [x for x in product.seller_ids if (x.name.id in partner_ids) and (x.product_id == product)]
	           if not sellers:
	               sellers = [x for x in product.seller_ids if (x.name.id in partner_ids) and not x.product_id]
	       if sellers:
	           for s in sellers:
	               seller_variant = s.product_name and (
	                   variant and "%s (%s)" % (s.product_name, variant) or s.product_name
	                   ) or False
	               mydict = {
	                         'id': product.id,
	                         'name': seller_variant or name,
	                         'default_code': s.product_code or product.default_code,
	                         }
	               temp = _name_get(mydict)
	               if temp not in result:
	                   result.append(temp)
	       else:
	           mydict = {
	                     'id': product.id,
	                     'name': name,
	                     'default_code': product.default_code,
	                     }
	           result.append(_name_get(mydict))
	   return result
	*/
	// display only the attributes with multiple possible values on the template
	variableAttributes := h.ProductAttribute().NewSet(rs.Env())
	for _, attrLine := range rs.AttributeLines().Records() {
		if attrLine.Values().Len() > 1 {
			variableAttributes = variableAttributes.Union(attrLine.Attribute())
		}
	}
	variant := rs.AttributeValues().VariantName(variableAttributes)
	if variant != "" {
		return fmt.Sprintf("%s (%s)", rs.PartnerRef(), variant)
	}
	return rs.PartnerRef()
}

func product_product_SearchByName(rs m.ProductProductSet, name string, op operator.Operator, additionalCond q.ProductProductCondition, limit int) m.ProductProductSet {
	if name == "" {
		return rs.Super().SearchByName(name, op, additionalCond, limit)
	}
	products := h.ProductProduct().NewSet(rs.Env())
	if op.IsPositive() {
		products = rs.Search(q.ProductProduct().DefaultCode().Equals(name).AndCond(additionalCond)).Limit(limit)
		if products.IsEmpty() {
			products = rs.Search(q.ProductProduct().Barcode().Equals(name).AndCond(additionalCond)).Limit(limit)
		}
	}
	switch {
	case products.IsEmpty() && !op.IsNegative():
		// Do not merge the 2 next lines into one single search, SQL search performance would be abysmal
		// on a database with thousands of matching products, due to the huge merge+unique needed for the
		// OR operator (and given the fact that the 'name' lookup results come from the ir.translation table
		// Performing a quick memory merge of ids in Python will give much better performance
		products = h.ProductProduct().Search(rs.Env(), q.ProductProduct().DefaultCode().AddOperator(op, name)).Limit(limit)
		if limit == 0 || products.Len() < limit {
			// we may underrun the limit because of dupes in the results, that's fine
			limit2 := limit - products.Len()
			if limit2 < 0 {
				limit2 = 0
			}
			products = products.Union(h.ProductProduct().Search(rs.Env(),
				q.ProductProduct().Name().AddOperator(op, name).And().ID().NotIn(products.Ids()))).Limit(limit2)
		}
	case products.IsEmpty() && op.IsNegative():
		products = h.ProductProduct().Search(rs.Env(),
			q.ProductProduct().DefaultCode().AddOperator(op, name).And().Name().AddOperator(op, name).AndCond(additionalCond))
	}
	if products.IsEmpty() && op.IsPositive() {
		ptrn, _ := regexp.Compile(`(\[(.*?)\])`)
		res := ptrn.FindAllString(name, -1)
		if len(res) > 1 {
			products = h.ProductProduct().Search(rs.Env(),
				q.ProductProduct().DefaultCode().Equals(res[1]).AndCond(additionalCond))
		}
	}
	// still no results, partner in context: search on supplier info as last hope to find something
	if products.IsEmpty() && rs.Env().Context().HasKey("partner_id") {
		partner := h.Partner().Browse(rs.Env(), []int64{rs.Env().Context().GetInteger("partner_id")})
		suppliers := h.ProductSupplierinfo().Search(rs.Env(),
			q.ProductSupplierinfo().Name().Equals(partner).
				AndCond(q.ProductSupplierinfo().ProductCode().AddOperator(op, name).Or().ProductName().AddOperator(op, name)))
		if !suppliers.IsEmpty() {
			products = h.ProductProduct().Search(rs.Env(),
				q.ProductProduct().ProductTmplFilteredOn(q.ProductTemplate().Sellers().In(suppliers)))
		}
	}
	return products
}

//`OpenProductTemplate is a utility method used to add an "Open Template" button in product views`,
func product_product_OpenProductTemplate(rs m.ProductProductSet) *actions.Action {
	rs.EnsureOne()
	return &actions.Action{
		Type:     actions.ActionActWindow,
		Model:    "ProductTemplate",
		ViewMode: "form",
		ResID:    rs.ProductTmpl().ID(),
		Target:   "new",
	}
}

//`SelectSeller returns the ProductSupplierInfo to use for the given partner, quantity, date and UoM.
//		If any of the parameters are their Go zero value, then they are not used for filtering.`,
func product_product_SelectSeller(rs m.ProductProductSet, partner m.PartnerSet, quantity float64, date dates.Date, uom m.ProductUomSet) m.ProductSupplierinfoSet {
	rs.EnsureOne()
	if date.IsZero() {
		date = dates.Today()
	}
	res := h.ProductSupplierinfo().NewSet(rs.Env())
	for _, seller := range rs.Sellers().Records() {
		quantityUomSeller := quantity
		if quantityUomSeller != 0 && !uom.IsEmpty() && !uom.Equals(seller.ProductUom()) {
			quantityUomSeller = uom.ComputeQuantity(quantityUomSeller, seller.ProductUom(), true)
		}
		if !seller.DateStart().IsZero() && seller.DateStart().Greater(date) {
			continue
		}
		if !seller.DateEnd().IsZero() && seller.DateEnd().Lower(date) {
			continue
		}
		if !partner.IsEmpty() && seller.Name().Intersect(partner.Union(partner.Parent())).IsEmpty() {
			continue
		}
		if quantityUomSeller < seller.MinQty() {
			continue
		}
		if !seller.Product().IsEmpty() && !seller.Product().Equals(rs) {
			continue
		}
		res = res.Union(seller)
		break
	}
	return res
}

//`PriceCompute returns the price field defined by priceType in the given uom and currency
//		for the given company.`,
func product_product_PriceCompute(rs m.ProductProductSet, priceType models.FieldNamer, uom m.ProductUomSet, currency m.CurrencySet, company m.CompanySet) float64 {
	rs.EnsureOne()
	// FIXME: delegate to template or not ? fields are reencoded here ...
	// compatibility about context keys used a bit everywhere in the code
	if uom.IsEmpty() && rs.Env().Context().HasKey("uom") {
		uom = h.ProductUom().NewSet(rs.Env()).Browse([]int64{rs.Env().Context().GetInteger("uom")})
	}
	if currency.IsEmpty() && rs.Env().Context().HasKey("currency") {
		currency = h.Currency().NewSet(rs.Env()).Browse([]int64{rs.Env().Context().GetInteger("currency")})
	}

	product := rs
	if priceType == q.ProductProduct().StandardPrice() {
		// StandardPrice field can only be seen by users in base.group_user
		// Thus, in order to compute the sale price from the cost for users not in this group
		// We fetch the standard price as the superuser
		if company.IsEmpty() {
			company = h.User().NewSet(rs.Env()).CurrentUser().Company()
			if rs.Env().Context().HasKey("force_company") {
				company = h.Company().Browse(rs.Env(), []int64{rs.Env().Context().GetInteger("force_company")})
			}
		}
		product = rs.WithContext("force_company", company.ID()).Sudo()
	}

	price := product.Get(priceType.String()).(float64)
	if priceType == q.ProductProduct().ListPrice() {
		price += product.PriceExtra()
	}

	if !uom.IsEmpty() {
		price = product.Uom().ComputePrice(price, uom)
	}
	// Convert from current user company currency to asked one
	// This is right cause a field cannot be in more than one currency
	if !currency.IsEmpty() {
		price = product.Currency().Compute(price, currency, true)
	}
	return price
}

//`DefineStandardPrice stores the standard price change in order to be able to retrieve the cost of a product for
//		a given date`,
func product_product_DefineStandardPrice(rs m.ProductProductSet, value float64) {
	company := h.User().NewSet(rs.Env()).CurrentUser().Company()
	if rs.Env().Context().HasKey("force_company") {
		company = h.Company().Browse(rs.Env(), []int64{rs.Env().Context().GetInteger("force_company")})
	}
	for _, product := range rs.Records() {
		h.ProductPriceHistory().Create(rs.Env(), h.ProductPriceHistory().NewData().
			SetProduct(product).
			SetCost(value).
			SetCompany(company))
	}
}

//`GetHistoryPrice returns the standard price of this product for the given company at the given date`,
func product_product_GetHistoryPrice(rs m.ProductProductSet, company m.CompanySet, date dates.DateTime) float64 {
	if date.IsZero() {
		date = dates.Now()
	}
	history := h.ProductPriceHistory().Search(rs.Env(),
		q.ProductPriceHistory().Company().Equals(company).
			And().Product().In(rs).
			And().Datetime().LowerOrEqual(date)).Limit(1)
	return history.Cost()
})

//`NeedProcurement`,
func product_product_NeedProcurement(rs m.ProductProductSet) bool {
	// When sale/product is installed alone, there is no need to create procurements. Only
	// sale_stock and sale_service need procurements
	return false
}

var fields_ProductPackaging = map[string]models.FieldDefinition{
	"Name": fields.Char{String: "Packaging Type", Required: true},
	"Sequence": fields.Integer{Default: models.DefaultValue(1),
		Help: "The first in the sequence is the default one."},
	"ProductTmpl": fields.Many2One{String: "Product", RelationModel: h.ProductTemplate()},
	"Qty": fields.Float{String: "Quantity per Package",
		Help: "The total number of products you can have per pallet or box."},
}

var fields_ProductSupplierinfo = map[string]models.FieldDefinition{
	"Name": fields.Many2One{String: "Vendor", RelationModel: h.Partner(), JSON: "name",
		Filter: q.Partner().Supplier().Equals(true), OnDelete: models.Cascade, Required: true,
		Help: "Vendor of this product"},
	"ProductName": fields.Char{String: "Vendor Product Name",
		Help: `This vendor's product name will be used when printing a request for quotation.
	Keep empty to use the internal one.`},
	"ProductCode": fields.Char{String: "Vendor Product Code",
		Help: `This vendor's product code will be used when printing a request for quotation.
	Keep empty to use the internal one.`},
	"Sequence": fields.Integer{Default: models.DefaultValue(1),
		Help: "Assigns the priority to the list of product vendor."},
	"ProductUom": fields.Many2One{String: "Vendor Unit of Measure", RelationModel: h.ProductUom(),
		ReadOnly: true, Related: "ProductTmpl.UomPo", Help: "This comes from the product form."},
	"MinQty": fields.Float{String: "Minimal Quantity", Default: models.DefaultValue(0), Required: true,
		Help: `The minimal quantity to purchase from this vendor, expressed in the vendor Product Unit of Measure if any,
	or in the default unit of measure of the product otherwise.`},
	"Price": fields.Float{Default: models.DefaultValue(0), Digits: decimalPrecision.GetPrecision("Product Price"),
		Required: true, Help: "The price to purchase a product"},
	"Company": fields.Many2One{RelationModel: h.Company(), Default: func(env models.Environment) interface{} {
		return h.User().NewSet(env).CurrentUser().Company()
	}, Index: true},
	"Currency": fields.Many2One{RelationModel: h.Currency(), Default: func(env models.Environment) interface{} {
		return h.User().NewSet(env).CurrentUser().Company().Currency()
	}, Required: true},
	"DateStart": fields.Date{String: "Start Date", Help: "Start date for this vendor price"},
	"DateEnd":   fields.Date{String: "End Date", Help: "End date for this vendor price"},
	"Product": fields.Many2One{String: "Product Variant", RelationModel: h.ProductProduct(),
		Help: "When this field is filled in, the vendor data will only apply to the variant."},
	"ProductTmpl": fields.Many2One{String: "Product Template", RelationModel: h.ProductTemplate(),
		Index: true, OnDelete: models.Cascade},
	"Delay": fields.Integer{String: "Delivery Lead Time", Default: models.DefaultValue(1), Required: true,
		Help: `Lead time in days between the confirmation of the purchase order and the receipt of the
	products in your warehouse. Used by the scheduler for automatic computation of the purchase order planning.`},
}

func init() {

	models.NewModel("ProductCategory")

	h.ProductCategory().AddFields(fields_ProductCategory)
	h.ProductCategory().SetDefaultOrder("Parent.Name")

	h.ProductCategory().NewMethod("CheckCategoryRecursion", product_category_CheckCategoryRecursion)

	h.ProductCategory().Methods().SearchByName().Extend(product_category_SearchByName)
	h.ProductCategory().Methods().NameGet().Extend(product_category_NameGet)

	models.NewModel("ProductPriceHistory")
	h.ProductPriceHistory().SetDefaultOrder("Datetime DESC")
	h.ProductPriceHistory().AddFields(fields_ProductPriceHistory)

	models.NewModel("ProductProduct")
	h.ProductProduct().SetDefaultOrder("DefaultCode", "Name", "ID")

	h.ProductProduct().AddFields(fields_ProductProduct)

	h.ProductProduct().NewMethod("ComputeProductCode", product_product_ComputeProductCode)
	h.ProductProduct().NewMethod("GetPricelistItems", product_product_GetPricelistItems)
	h.ProductProduct().NewMethod("UnlinkOrDeactivate", product_product_UnlinkOrDeactivate)
	h.ProductProduct().NewMethod("InverseImageValue", product_product_InverseImageValue)
	h.ProductProduct().NewMethod("ComputeProductPrice", product_product_ComputeProductPrice)
	h.ProductProduct().NewMethod("InverseProductPrice", product_product_InverseProductPrice)
	h.ProductProduct().NewMethod("InverseProductLstPrice", product_product_InverseProductLstPrice)
	h.ProductProduct().NewMethod("ComputeProductPriceExtra", product_product_ComputeProductPriceExtra)
	h.ProductProduct().NewMethod("ComputeProductLstPrice", product_product_ComputeProductLstPrice)
	h.ProductProduct().NewMethod("ComputePartnerRef", product_product_ComputePartnerRef)
	h.ProductProduct().NewMethod("ComputeImages", product_product_ComputeImages)
	h.ProductProduct().NewMethod("CheckAttributeValueIds", product_product_CheckAttributeValueIds)
	h.ProductProduct().NewMethod("OnchangeUom", product_product_OnchangeUom)
	h.ProductProduct().NewMethod("NameFormat", product_product_NameFormat)
	h.ProductProduct().NewMethod("OpenProductTemplate", product_product_OpenProductTemplate)
	h.ProductProduct().NewMethod("SelectSeller", product_product_SelectSeller)
	h.ProductProduct().NewMethod("PriceCompute", product_product_PriceCompute)
	h.ProductProduct().NewMethod("DefineStandardPrice", product_product_DefineStandardPrice)
	h.ProductProduct().NewMethod("GetHistoryPrice", product_product_GetHistoryPrice)
	h.ProductProduct().NewMethod("NeedProcurement", product_product_NeedProcurement)

	h.ProductProduct().Methods().Create().Extend(product_product_Create)
	h.ProductProduct().Methods().Write().Extend(product_product_Write)
	h.ProductProduct().Methods().Unlink().Extend(product_product_Unlink)
	h.ProductProduct().Methods().Copy().Extend(product_product_Copy)
	h.ProductProduct().Methods().Search().Extend(product_product_Search)
	h.ProductProduct().Methods().NameGet().Extend(product_product_NameGet)
	h.ProductProduct().Methods().SearchByName().Extend(product_product_SearchByName)

	models.NewModel("ProductPackaging")
	h.ProductPackaging().SetDefaultOrder("Sequence")
	h.ProductPackaging().AddFields(fields_ProductPackaging)

	models.NewModel("ProductSupplierinfo")
	h.ProductSupplierinfo().SetDefaultOrder("Sequence", "MinQty DESC", "Price")
	h.ProductSupplierinfo().AddFields(fields_ProductSupplierinfo)

}
