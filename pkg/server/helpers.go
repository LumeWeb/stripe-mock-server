package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/v2"
	"go.lumeweb.com/stripe-mock-server/pkg/generator"
	"go.lumeweb.com/stripe-mock-server/pkg/internal/gen/models/api"
)

// newKoanfFromMap creates a koanf instance from a map[string]any
func newKoanfFromMap(m map[string]any) *koanf.Koanf {
	k := koanf.New(".")
	_ = k.Load(confmap.Provider(m, "."), nil)
	return k
}

// GetString retrieves a string value from a map.
func GetString(m map[string]any, key string) string {
	k := newKoanfFromMap(m)
	return k.String(key)
}

// GetInt64 retrieves an int64 value from a map (handles int/int64/float64/string).
func GetInt64(m map[string]any, key string) int64 {
	k := newKoanfFromMap(m)
	return k.Int64(key)
}

// GetBool retrieves a boolean value from a map (handles bool/string).
func GetBool(m map[string]any, key string) bool {
	k := newKoanfFromMap(m)
	return k.Bool(key)
}

// GetStringMap retrieves a map[string]any value from a map.
func GetStringMap(m map[string]any, key string) map[string]any {
	k := newKoanfFromMap(m)
	if m := k.Get(key); m != nil {
		if result, ok := m.(map[string]any); ok {
			return result
		}
	}
	return nil
}

// GetMapSlice retrieves a slice of map[string]any from a map.
func GetMapSlice(m map[string]any, key string) []map[string]any {
	k := newKoanfFromMap(m)
	var result []map[string]any
	if slice := k.Get(key); slice != nil {
		// Handle []any (from JSON unmarshal)
		if s, ok := slice.([]any); ok {
			for _, item := range s {
				if m, ok := item.(map[string]any); ok {
					result = append(result, m)
				}
			}
		}
		// Handle []map[string]any (from Go literal)
		if s, ok := slice.([]map[string]any); ok {
			result = append(result, s...)
		}
	}
	return result
}

// GetStringSlice retrieves a slice of strings from a map.
func GetStringSlice(m map[string]any, key string) []string {
	k := newKoanfFromMap(m)
	return k.Strings(key)
}

// GetStringWithEmpty retrieves a string value from a map, treating empty string as missing.
func GetStringWithEmpty(m map[string]any, key string) (*string, bool) {
	k := newKoanfFromMap(m)
	s := k.String(key)
	if s != "" {
		return &s, true
	}
	return nil, false
}

// stringToUnion converts a string ID to a json.RawMessage union type
// Used for fields like Price_Product, Subscription_Customer that can be either ID or expanded object
func stringToUnion(s string) json.RawMessage {
	if s == "" {
		return nil
	}
	b, _ := json.Marshal(s)
	return b
}

// mapAnyToString converts map[string]any to map[string]string
// Used for metadata conversion from request to response types
func mapAnyToString(m map[string]any) map[string]string {
	if m == nil {
		return nil
	}
	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result
}

// mapStringToAny converts map[string]string to map[string]any
func mapStringToAny(m map[string]string) map[string]any {
	if m == nil {
		return nil
	}
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

// mergeMetadata merges new metadata into existing
func mergeMetadata(existing map[string]string, newMeta map[string]any) map[string]string {
	if existing == nil && newMeta == nil {
		return nil
	}
	if existing == nil {
		existing = make(map[string]string)
	}
	for k, v := range newMeta {
		existing[k] = fmt.Sprintf("%v", v)
	}
	return existing
}

// parseEnumValue converts a string value to the target enum type
func parseEnumValue[T ~string](s string) T {
	return T(s)
}

// parseEnumSlice converts a string slice to the target enum slice type
func parseEnumSlice[T ~string](strings []string) []T {
	if strings == nil {
		return nil
	}
	result := make([]T, len(strings))
	for i, s := range strings {
		result[i] = T(s)
	}
	return result
}

// parseCustomerUpdateAllowedUpdates converts string slice to PortalCustomerUpdateAllowedUpdates
func parseCustomerUpdateAllowedUpdates(vals []string) []api.PortalCustomerUpdateAllowedUpdates {
	var result []api.PortalCustomerUpdateAllowedUpdates
	for _, v := range vals {
		if v == "" {
			continue
		}
		result = append(result, api.PortalCustomerUpdateAllowedUpdates(v))
	}
	return result
}

// parseSubscriptionUpdateDefaultAllowedUpdates converts string slice to PortalSubscriptionUpdateDefaultAllowedUpdates
func parseSubscriptionUpdateDefaultAllowedUpdates(vals []string) []api.PortalSubscriptionUpdateDefaultAllowedUpdates {
	var result []api.PortalSubscriptionUpdateDefaultAllowedUpdates
	for _, v := range vals {
		if v == "" {
			continue
		}
		result = append(result, api.PortalSubscriptionUpdateDefaultAllowedUpdates(v))
	}
	return result
}

// parseSubscriptionCancelReasonOptions converts string slice to PortalSubscriptionCancellationReasonOptions
func parseSubscriptionCancelReasonOptions(vals []string) []api.PortalSubscriptionCancellationReasonOptions {
	var result []api.PortalSubscriptionCancellationReasonOptions
	for _, v := range vals {
		if v == "" {
			continue
		}
		result = append(result, api.PortalSubscriptionCancellationReasonOptions(v))
	}
	return result
}

// mapStringToBool converts string "true"/"false" to boolean
func mapStringToBool(s string) bool {
	return strings.ToLower(s) == "true"
}

// buildCustomer creates a Customer from input data with all supported fields
func buildCustomer(id string, data map[string]any) *api.Customer {
	c := &api.Customer{
		Id:       id,
		Object:   api.CustomerObjectEnumCustomer,
		Livemode: false,
	}

	if email := GetString(data, "email"); email != "" {
		c.Email = &email
	}

	if name := GetString(data, "name"); name != "" {
		c.Name = &name
	}

	if description := GetString(data, "description"); description != "" {
		c.Description = &description
	}

	if phone := GetString(data, "phone"); phone != "" {
		c.Phone = &phone
	}

	if currency := GetString(data, "currency"); currency != "" {
		c.Currency = &currency
	}

	if balance := GetInt64(data, "balance"); balance != 0 {
		b := int(balance)
		c.Balance = &b
	}

	if address := GetStringMap(data, "address"); address != nil {
		c.Address = buildCustomerAddress(address)
	}

	if shipping := GetStringMap(data, "shipping"); shipping != nil {
		c.Shipping = buildCustomerShipping(shipping)
	}

	if metadata := GetStringMap(data, "metadata"); len(metadata) > 0 {
		m := mapAnyToString(metadata)
		c.Metadata = &m
	}

	return c
}

// buildCustomerAddress creates a Customer_Address from input data
func buildCustomerAddress(data map[string]any) *api.Customer_Address {
	address := &api.Customer_Address{}

	addr := api.Address{}
	if line1 := GetString(data, "line1"); line1 != "" {
		addr.Line1 = &line1
	}
	if line2 := GetString(data, "line2"); line2 != "" {
		addr.Line2 = &line2
	}
	if city := GetString(data, "city"); city != "" {
		addr.City = &city
	}
	if state := GetString(data, "state"); state != "" {
		addr.State = &state
	}
	if postalCode := GetString(data, "postal_code"); postalCode != "" {
		addr.PostalCode = &postalCode
	}
	if country := GetString(data, "country"); country != "" {
		addr.Country = &country
	}

	_ = address.FromAddress(addr)

	return address
}

// buildAddress creates an Address from input data (non-union version)
func buildAddress(data map[string]any) *api.Address {
	addr := &api.Address{}
	if line1 := GetString(data, "line1"); line1 != "" {
		addr.Line1 = &line1
	}
	if line2 := GetString(data, "line2"); line2 != "" {
		addr.Line2 = &line2
	}
	if city := GetString(data, "city"); city != "" {
		addr.City = &city
	}
	if state := GetString(data, "state"); state != "" {
		addr.State = &state
	}
	if postalCode := GetString(data, "postal_code"); postalCode != "" {
		addr.PostalCode = &postalCode
	}
	if country := GetString(data, "country"); country != "" {
		addr.Country = &country
	}
	return addr
}

// buildCustomerShipping creates a Customer_Shipping from input data
func buildCustomerShipping(data map[string]any) *api.Customer_Shipping {
	shipping := &api.Customer_Shipping{}

	ship := api.Shipping{}
	if name := GetString(data, "name"); name != "" {
		ship.Name = &name
	}
	if phone := GetString(data, "phone"); phone != "" {
		ship.Phone = &phone
	}
	if carrier := GetString(data, "carrier"); carrier != "" {
		ship.Carrier = &carrier
	}
	if trackingNumber := GetString(data, "tracking_number"); trackingNumber != "" {
		ship.TrackingNumber = &trackingNumber
	}

	if address := GetStringMap(data, "address"); address != nil {
		ship.Address = buildAddress(address)
	}

	_ = shipping.FromShipping(ship)

	return shipping
}

// buildProduct creates a Product from input data with all supported fields
func buildProduct(id string, data map[string]any) *api.Product {
	now := int(time.Now().Unix())

	p := &api.Product{
		Id:                id,
		Object:            api.ProductObjectEnumProduct,
		Livemode:          false,
		Active:            true,
		Created:           now,
		Updated:           now,
		Images:            []string{},
		MarketingFeatures: []api.ProductMarketingFeature{},
		Metadata:          map[string]string{},
	}

	if name := GetString(data, "name"); name != "" {
		p.Name = name
	}

	if description := GetString(data, "description"); description != "" {
		p.Description = &description
	}

	if _, ok := GetStringWithEmpty(data, "active"); ok {
		p.Active = GetBool(data, "active")
	}

	if images := GetStringSlice(data, "images"); len(images) > 0 {
		p.Images = images
	}

	if url := GetString(data, "url"); url != "" {
		p.Url = &url
	}

	if unitLabel := GetString(data, "unit_label"); unitLabel != "" {
		p.UnitLabel = &unitLabel
	}

	if statementDescriptor := GetString(data, "statement_descriptor"); statementDescriptor != "" {
		p.StatementDescriptor = &statementDescriptor
	}

	if metadata := GetStringMap(data, "metadata"); len(metadata) > 0 {
		p.Metadata = mapAnyToString(metadata)
	}

	if _, ok := GetStringWithEmpty(data, "shippable"); ok {
		// Parse the string to bool
		shippable := GetBool(data, "shippable")
		p.Shippable = &shippable
	}

	if defaultPrice := GetString(data, "default_price"); defaultPrice != "" {
		var dp api.Product_DefaultPrice
		_ = dp.FromProductDefaultPrice0(defaultPrice)
		p.DefaultPrice = &dp
	}

	return p
}

// buildPrice creates a Price from input data with all supported fields
func buildPrice(id string, data map[string]any) *api.Price {
	now := int(time.Now().Unix())

	p := &api.Price{
		Id:       id,
		Object:   api.PriceObjectEnumPrice,
		Livemode: false,
		Active:   true,
		Created:  now,
		Metadata: map[string]string{},
		Type:     api.PriceTypeEnumOneTime,
	}

	if currency := GetString(data, "currency"); currency != "" {
		p.Currency = currency
	}

	if amount := GetInt64(data, "unit_amount"); amount != 0 {
		a := int(amount)
		p.UnitAmount = &a
	}

	if unitAmountDecimal := GetString(data, "unit_amount_decimal"); unitAmountDecimal != "" {
		p.UnitAmountDecimal = &unitAmountDecimal
	}

	if product := GetString(data, "product"); product != "" {
		var pp api.Price_Product
		_ = pp.FromPriceProduct0(product)
		p.Product = pp
	}

	if nickname := GetString(data, "nickname"); nickname != "" {
		p.Nickname = &nickname
	}

	if lookupKey := GetString(data, "lookup_key"); lookupKey != "" {
		p.LookupKey = &lookupKey
	}

	if billingScheme := GetString(data, "billing_scheme"); billingScheme != "" {
		p.BillingScheme = api.PriceBillingScheme(billingScheme)
	}

	if tiersMode := GetString(data, "tiers_mode"); tiersMode != "" {
		tm := api.PriceTiersMode(tiersMode)
		p.TiersMode = &tm
	}

	if taxBehavior := GetString(data, "tax_behavior"); taxBehavior != "" {
		tb := api.PriceTaxBehavior(taxBehavior)
		p.TaxBehavior = &tb
	}

	if priceType := GetString(data, "type"); priceType != "" {
		p.Type = api.PriceTypeEnum(priceType)
	}

	if recurring := GetStringMap(data, "recurring"); len(recurring) > 0 {
		// Set type to recurring if recurring data is provided
		p.Type = api.PriceTypeEnumRecurring
		rec := api.Recurring{}
		if interval := GetString(recurring, "interval"); interval != "" {
			rec.Interval = api.RecurringInterval(interval)
		}
		if usageType := GetString(recurring, "usage_type"); usageType != "" {
			rec.UsageType = api.RecurringUsageType(usageType)
		}
		if intervalCount := GetInt64(recurring, "interval_count"); intervalCount != 0 {
			rec.IntervalCount = int(intervalCount)
		}
		var pr api.Price_Recurring
		_ = pr.FromRecurring(rec)
		p.Recurring = &pr
	}

	if metadata := GetStringMap(data, "metadata"); len(metadata) > 0 {
		p.Metadata = mapAnyToString(metadata)
	}

	return p
}

// buildCheckoutSession creates a CheckoutSession from input data with all supported fields
func buildCheckoutSession(id string, data map[string]any) *api.CheckoutSession {
	now := int(time.Now().Unix())

	s := &api.CheckoutSession{
		Id:            id,
		Object:        api.CheckoutSessionObjectEnumCheckoutSession,
		Livemode:      false,
		ExpiresAt:     now + 86400, // 24 hours from now
		Created:       now,
		PaymentStatus: api.CheckoutSessionPaymentStatusUnpaid,
		AutomaticTax: api.PaymentPagesCheckoutSessionAutomaticTax{
			Enabled: false,
		},
		CustomFields: []api.PaymentPagesCheckoutSessionCustomFields{},
		CustomText:   api.PaymentPagesCheckoutSessionCustomText{},
	}

	pmc := api.CheckoutSessionPaymentMethodCollectionAlways
	s.PaymentMethodCollection = &pmc

	status := api.CheckoutSessionStatusOpen
	s.Status = &status

	if mode := GetString(data, "mode"); mode != "" {
		s.Mode = api.CheckoutSessionModeEnum(mode)
	}

	if customer := GetString(data, "customer"); customer != "" {
		var c api.CheckoutSession_Customer
		_ = c.FromCheckoutSessionCustomer0(customer)
		s.Customer = &c
	}

	if customerEmail := GetString(data, "customer_email"); customerEmail != "" {
		s.CustomerEmail = &customerEmail
	}

	if clientRef := GetString(data, "client_reference_id"); clientRef != "" {
		s.ClientReferenceId = &clientRef
	}

	if successURL := GetString(data, "success_url"); successURL != "" {
		s.SuccessUrl = &successURL
	}

	if cancelURL := GetString(data, "cancel_url"); cancelURL != "" {
		s.CancelUrl = &cancelURL
	}

	if currency := GetString(data, "currency"); currency != "" {
		s.Currency = &currency
	}

	if locale := GetString(data, "locale"); locale != "" {
		l := api.CheckoutSessionLocale(locale)
		s.Locale = &l
	}

	if submitType := GetString(data, "submit_type"); submitType != "" {
		st := api.CheckoutSessionSubmitType(submitType)
		s.SubmitType = &st
	}

	if billingAddressCollection := GetString(data, "billing_address_collection"); billingAddressCollection != "" {
		bac := api.CheckoutSessionBillingAddressCollection(billingAddressCollection)
		s.BillingAddressCollection = &bac
	}

	if _, ok := GetStringWithEmpty(data, "allow_promotion_codes"); ok {
		// Parse the string to bool
		allowPromotionCodes := GetBool(data, "allow_promotion_codes")
		s.AllowPromotionCodes = &allowPromotionCodes
	}

	if metadata := GetStringMap(data, "metadata"); len(metadata) > 0 {
		m := mapAnyToString(metadata)
		s.Metadata = &m
	}

	// Handle line_items for subscription mode
	if lineItems := GetMapSlice(data, "line_items"); len(lineItems) > 0 {
		items := make([]api.Item, len(lineItems))
		for i, item := range lineItems {
			items[i] = api.Item{
				Id:     "li_" + generator.RandomString(14),
				Object: api.ItemObjectEnumItem,
			}
			if priceID := GetString(item, "price"); priceID != "" {
				var priceUnion api.Item_Price
				price := api.Price{
					Id:       priceID,
					Object:   api.PriceObjectEnumPrice,
					Currency: "usd",
					Active:   true,
					Livemode: false,
					Type:     api.PriceTypeEnumRecurring,
				}
				_ = priceUnion.FromPrice(price)
				items[i].Price = &priceUnion
			}
			if qty := GetInt64(item, "quantity"); qty != 0 {
				q := int(qty)
				items[i].Quantity = &q
			}
		}
		s.LineItems = &api.PaymentPagesCheckoutSessionListLineItems{
			Data:    items,
			HasMore: false,
			Object:  api.PaymentPagesCheckoutSessionListLineItemsObjectList,
			Url:     "/v1/checkout/sessions/" + id + "/line_items",
		}
	}

	return s
}

// buildPortalBusinessProfile creates a PortalBusinessProfile from input data
func buildPortalBusinessProfile(data map[string]any) api.PortalBusinessProfile {
	profile := api.PortalBusinessProfile{}

	if headline, ok := GetStringWithEmpty(data, "headline"); ok {
		profile.Headline = headline
	}
	if privacyURL, ok := GetStringWithEmpty(data, "privacy_policy_url"); ok {
		profile.PrivacyPolicyUrl = privacyURL
	}
	if tosURL, ok := GetStringWithEmpty(data, "terms_of_service_url"); ok {
		profile.TermsOfServiceUrl = tosURL
	}

	return profile
}

// buildPortalCustomerUpdate creates a PortalCustomerUpdate from input data
func buildPortalCustomerUpdate(data map[string]any) api.PortalCustomerUpdate {
	update := api.PortalCustomerUpdate{
		Enabled: GetBool(data, "enabled"),
	}

	if allowedUpdates := GetStringSlice(data, "allowed_updates"); len(allowedUpdates) > 0 {
		update.AllowedUpdates = parseCustomerUpdateAllowedUpdates(allowedUpdates)
	}

	return update
}

// buildPortalInvoiceList creates a PortalInvoiceList from input data
func buildPortalInvoiceList(data map[string]any) api.PortalInvoiceList {
	return api.PortalInvoiceList{
		Enabled: GetBool(data, "enabled"),
	}
}

// buildPortalPaymentMethodUpdate creates a PortalPaymentMethodUpdate from input data
func buildPortalPaymentMethodUpdate(data map[string]any) api.PortalPaymentMethodUpdate {
	update := api.PortalPaymentMethodUpdate{
		Enabled: GetBool(data, "enabled"),
	}

	if pmConfig, ok := GetStringWithEmpty(data, "payment_method_configuration"); ok {
		update.PaymentMethodConfiguration = pmConfig
	}

	return update
}

// buildPortalSubscriptionCancellationReason creates a PortalSubscriptionCancellationReason from input data
func buildPortalSubscriptionCancellationReason(data map[string]any) api.PortalSubscriptionCancellationReason {
	reason := api.PortalSubscriptionCancellationReason{
		Enabled: GetBool(data, "enabled"),
	}

	if options := GetStringSlice(data, "options"); len(options) > 0 {
		reason.Options = parseSubscriptionCancelReasonOptions(options)
	}

	return reason
}

// buildPortalSubscriptionCancel creates a PortalSubscriptionCancel from input data
func buildPortalSubscriptionCancel(data map[string]any) api.PortalSubscriptionCancel {
	cancel := api.PortalSubscriptionCancel{
		Enabled: GetBool(data, "enabled"),
	}

	if mode := GetString(data, "mode"); mode != "" {
		cancel.Mode = api.PortalSubscriptionCancelMode(mode)
	}

	if prorationBehavior := GetString(data, "proration_behavior"); prorationBehavior != "" {
		cancel.ProrationBehavior = api.PortalSubscriptionCancelProrationBehavior(prorationBehavior)
	}

	if reasonData := GetStringMap(data, "cancellation_reason"); reasonData != nil {
		cancel.CancellationReason = buildPortalSubscriptionCancellationReason(reasonData)
	}

	return cancel
}

// buildPortalResourceScheduleUpdateAtPeriodEnd creates a PortalResourceScheduleUpdateAtPeriodEnd from input data
func buildPortalResourceScheduleUpdateAtPeriodEnd(data map[string]any) api.PortalResourceScheduleUpdateAtPeriodEnd {
	var result api.PortalResourceScheduleUpdateAtPeriodEnd

	// Default to empty conditions
	result.Conditions = []api.PortalResourceScheduleUpdateAtPeriodEndCondition{}

	if conditions, ok := data["conditions"].([]any); ok {
		for _, c := range conditions {
			if condMap, ok := c.(map[string]any); ok {
				if typeVal, ok := condMap["type"]; ok {
					if typeStr, ok := typeVal.(string); ok {
						conditionType := api.PortalResourceScheduleUpdateAtPeriodEndConditionType(typeStr)
						result.Conditions = append(result.Conditions, api.PortalResourceScheduleUpdateAtPeriodEndCondition{
							Type: conditionType,
						})
					}
				}
			}
		}
	}

	return result
}

// buildPortalSubscriptionUpdate creates a PortalSubscriptionUpdate from input data
func buildPortalSubscriptionUpdate(data map[string]any) api.PortalSubscriptionUpdate {
	update := api.PortalSubscriptionUpdate{
		Enabled: GetBool(data, "enabled"),
	}

	if defaultUpdates := GetStringSlice(data, "default_allowed_updates"); len(defaultUpdates) > 0 {
		update.DefaultAllowedUpdates = parseSubscriptionUpdateDefaultAllowedUpdates(defaultUpdates)
	}

	if prorationBehavior := GetString(data, "proration_behavior"); prorationBehavior != "" {
		update.ProrationBehavior = api.PortalSubscriptionUpdateProrationBehavior(prorationBehavior)
	}

	if billingCycleAnchor := GetString(data, "billing_cycle_anchor"); billingCycleAnchor != "" {
		bca := api.PortalSubscriptionUpdateBillingCycleAnchor(billingCycleAnchor)
		update.BillingCycleAnchor = &bca
	}

	if trialUpdateBehavior := GetString(data, "trial_update_behavior"); trialUpdateBehavior != "" {
		update.TrialUpdateBehavior = api.PortalSubscriptionUpdateTrialUpdateBehavior(trialUpdateBehavior)
	}

	if scheduleData := GetStringMap(data, "schedule_at_period_end"); scheduleData != nil {
		update.ScheduleAtPeriodEnd = buildPortalResourceScheduleUpdateAtPeriodEnd(scheduleData)
	}

	if productsData := GetMapSlice(data, "products"); len(productsData) > 0 {
		var products []api.PortalSubscriptionUpdateProduct
		for _, p := range productsData {
			product := buildPortalSubscriptionUpdateProduct(p)
			products = append(products, product)
		}
		update.Products = &products
	}

	return update
}

// buildPortalSubscriptionUpdateProduct creates a PortalSubscriptionUpdateProduct from input data
func buildPortalSubscriptionUpdateProduct(data map[string]any) api.PortalSubscriptionUpdateProduct {
	product := api.PortalSubscriptionUpdateProduct{
		Product: GetString(data, "product"),
	}

	if prices := GetStringSlice(data, "prices"); len(prices) > 0 {
		product.Prices = prices
	}

	if adjustableQty := GetStringMap(data, "adjustable_quantity"); adjustableQty != nil {
		aq := api.PortalSubscriptionUpdateProductAdjustableQuantity{
			Enabled: GetBool(adjustableQty, "enabled"),
			Minimum: 0, // Default minimum
		}
		if minimum := GetInt64(adjustableQty, "minimum"); minimum != 0 {
			aq.Minimum = int(minimum)
		}
		if maximum := GetInt64(adjustableQty, "maximum"); maximum != 0 {
			maxInt := int(maximum)
			aq.Maximum = &maxInt
		}
		product.AdjustableQuantity = aq
	}

	return product
}

// buildPortalFeatures creates a PortalFeatures from input data
func buildPortalFeatures(data map[string]any) api.PortalFeatures {
	features := api.PortalFeatures{}

	if customerUpdate := GetStringMap(data, "customer_update"); customerUpdate != nil {
		features.CustomerUpdate = buildPortalCustomerUpdate(customerUpdate)
	}

	if invoiceHistory := GetStringMap(data, "invoice_history"); invoiceHistory != nil {
		features.InvoiceHistory = buildPortalInvoiceList(invoiceHistory)
	}

	if paymentMethodUpdate := GetStringMap(data, "payment_method_update"); paymentMethodUpdate != nil {
		features.PaymentMethodUpdate = buildPortalPaymentMethodUpdate(paymentMethodUpdate)
	}

	if subscriptionCancel := GetStringMap(data, "subscription_cancel"); subscriptionCancel != nil {
		features.SubscriptionCancel = buildPortalSubscriptionCancel(subscriptionCancel)
	}

	if subscriptionUpdate := GetStringMap(data, "subscription_update"); subscriptionUpdate != nil {
		features.SubscriptionUpdate = buildPortalSubscriptionUpdate(subscriptionUpdate)
	}

	return features
}

// buildSubscription creates a minimal Subscription from input data
func buildSubscription(id string, data map[string]any) *api.Subscription {
	now := int(time.Now().Unix())
	
	s := &api.Subscription{
		Id:       id,
		Object:   api.SubscriptionObjectEnumSubscription,
		Created:  now,
		Livemode: false,
		Status:   api.SubscriptionStatusIncomplete,
	}
	
	// Set customer if provided
	if customer := GetString(data, "customer"); customer != "" {
		var c api.Subscription_Customer
		_ = c.FromSubscriptionCustomer0(customer)
		s.Customer = c
	}
	
	// Set items if provided
	items := GetMapSlice(data, "items")
	if len(items) > 0 {
		s.Items.Data = make([]api.SubscriptionItem, len(items))
		for i, item := range items {
			if priceId := GetString(item, "price"); priceId != "" {
				si := api.SubscriptionItem{
					Id:      "si_" + generator.RandomString(14),
					Object:  api.SubscriptionItemObjectEnumSubscriptionItem,
					Created: now,
					Price: api.Price{
						Id:       priceId,
						Object:   api.PriceObjectEnumPrice,
						Currency: "usd",
						Active:   true,
						Livemode: false,
						Type:     api.PriceTypeEnumRecurring,
					},
				}
				// Set period dates only if not provided
				if periodStart := GetInt64(item, "current_period_start"); periodStart != 0 {
					si.CurrentPeriodStart = int(periodStart)
				} else {
					si.CurrentPeriodStart = now
				}
				if periodEnd := GetInt64(item, "current_period_end"); periodEnd != 0 {
					si.CurrentPeriodEnd = int(periodEnd)
				} else {
					si.CurrentPeriodEnd = now + 2592000 // 30 days in seconds
				}
				s.Items.Data[i] = si
			}
		}
	}
	
	return s
}
