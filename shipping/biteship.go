package shipping

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Biteship implements Provider against https://api.biteship.com.
// The API key prefix selects the mode: biteship_test. is test mode.
type Biteship struct {
	apiKey  string
	baseURL string
	origin  Origin
	client  *http.Client
}

var _ Provider = (*Biteship)(nil)

func NewBiteship(apiKey, baseURL string, origin Origin) *Biteship {
	return &Biteship{
		apiKey:  apiKey,
		baseURL: baseURL,
		origin:  origin,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

// Rates uses Biteship's draft-order flow: the plain /v1/rates/couriers endpoint
// rejects requests with code 40001002 (even in test mode with a valid account),
// while creating a draft, reading its pricing and deleting it works.
func (b *Biteship) Rates(req RateRequest) ([]Rate, error) {
	items := req.Items
	if len(items) == 0 {
		items = []ShipmentItem{{Name: "Package", Quantity: 1, WeightGram: req.WeightGram}}
	}

	draftBody := map[string]any{
		"shipper_contact_name":      b.origin.ContactName,
		"shipper_contact_phone":     b.origin.ContactPhone,
		"shipper_organization":      "Golearn Store",
		"origin_contact_name":       b.origin.ContactName,
		"origin_contact_phone":      b.origin.ContactPhone,
		"origin_address":            b.origin.Address,
		"origin_postal_code":        b.origin.PostalCode,
		"destination_contact_name":  "Customer",
		"destination_contact_phone": b.origin.ContactPhone,
		"destination_address":       "-",
		"destination_postal_code":   req.DestinationPostalCode,
		"delivery_type":             "now",
		"items":                     itemsPayload(items),
	}
	if req.DestinationLat != 0 && req.DestinationLon != 0 {
		draftBody["destination_coordinate"] = map[string]float64{
			"latitude": req.DestinationLat, "longitude": req.DestinationLon,
		}
	}

	var draft struct {
		ID           string `json:"id"`
		ErrorMessage string `json:"error"`
		Success      bool   `json:"success"`
	}
	if err := b.do(http.MethodPost, "/v1/draft_orders", draftBody, &draft); err != nil {
		return nil, err
	}
	if draft.ID == "" {
		return nil, fmt.Errorf("biteship draft order: %s", draft.ErrorMessage)
	}

	// the draft exists only to price the shipment; delete it either way
	defer b.deleteDraft(draft.ID)

	var payload struct {
		Pricing      []biteshipRate `json:"pricing"`
		Success      bool           `json:"success"`
		ErrorMessage string         `json:"error"`
	}
	if err := b.do(http.MethodGet, "/v1/draft_orders/"+draft.ID+"/rates", nil, &payload); err != nil {
		return nil, err
	}
	if !payload.Success && payload.ErrorMessage != "" {
		return nil, fmt.Errorf("biteship rates: %s", payload.ErrorMessage)
	}

	rates := make([]Rate, 0, len(payload.Pricing))
	for _, rate := range payload.Pricing {
		rates = append(rates, rate.toRate())
	}
	return rates, nil
}

// collectionMethod defaults to pickup: several couriers reject drop_off.
func (b *Biteship) collectionMethod() string {
	if b.origin.CollectionMethod != "" {
		return b.origin.CollectionMethod
	}
	return "pickup"
}

func (b *Biteship) deleteDraft(id string) {
	req, err := http.NewRequest(http.MethodDelete, b.baseURL+"/v1/draft_orders/"+id, nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", b.apiKey)

	resp, err := b.client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

func itemsPayload(items []ShipmentItem) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"name":     item.Name,
			"value":    item.Value,
			"quantity": item.Quantity,
			// item weight is in grams
			"weight": item.WeightGram,
		})
	}
	return out
}

func (b *Biteship) CreateShipment(req ShipmentRequest) (*Shipment, error) {
	body := map[string]any{
		"reference_id":              req.ReferenceID,
		"shipper_contact_name":      b.origin.ContactName,
		"shipper_contact_phone":     b.origin.ContactPhone,
		"shipper_organization":      "Golearn Store",
		"origin_contact_name":       b.origin.ContactName,
		"origin_contact_phone":      b.origin.ContactPhone,
		"origin_address":            b.origin.Address,
		"origin_postal_code":        b.origin.PostalCode,
		"origin_collection_method":  b.collectionMethod(),
		"destination_contact_name":  req.ContactName,
		"destination_contact_phone": req.ContactPhone,
		"destination_contact_email": req.ContactEmail,
		"destination_address":       req.Address,
		"destination_postal_code":   req.PostalCode,
		"courier_company":           req.Courier,
		"courier_type":              req.Service,
		"delivery_type":             "now",
		"order_note":                req.Note,
		"items":                     itemsPayload(req.Items),
	}
	if req.Insurance > 0 {
		body["courier_insurance"] = req.Insurance
	}

	var payload struct {
		Success      bool   `json:"success"`
		ErrorMessage string `json:"error"`
		ID           string `json:"id"`
		Status       string `json:"status"`
		Courier      struct {
			WaybillID  string `json:"waybill_id"`
			TrackingID string `json:"tracking_id"`
		} `json:"courier"`
	}
	if err := b.do(http.MethodPost, "/v1/orders", body, &payload); err != nil {
		return nil, err
	}
	if !payload.Success {
		if payload.ErrorMessage == "" {
			payload.ErrorMessage = "unknown error"
		}
		return nil, fmt.Errorf("biteship order: %s", payload.ErrorMessage)
	}

	return &Shipment{
		ID:         payload.ID,
		WaybillID:  payload.Courier.WaybillID,
		TrackingID: payload.Courier.TrackingID,
		Status:     payload.Status,
	}, nil
}

// Tracking reads the current state of a Biteship order, which is the trusted
// source for shipment status (webhook payloads are never trusted directly).
func (b *Biteship) Tracking(providerOrderID string) (*Shipment, error) {
	var payload struct {
		Success bool   `json:"success"`
		ID      string `json:"id"`
		Status  string `json:"status"`
		Courier struct {
			WaybillID  string `json:"waybill_id"`
			TrackingID string `json:"tracking_id"`
			History    []struct {
				Status    string `json:"status"`
				Note      string `json:"note"`
				UpdatedAt string `json:"updated_at"`
			} `json:"history"`
		} `json:"courier"`
	}
	if err := b.do(http.MethodGet, "/v1/orders/"+providerOrderID, nil, &payload); err != nil {
		return nil, err
	}

	history := make([]ShipmentEvent, 0, len(payload.Courier.History))
	for _, event := range payload.Courier.History {
		history = append(history, ShipmentEvent{
			Status:    event.Status,
			Note:      event.Note,
			UpdatedAt: event.UpdatedAt,
		})
	}

	return &Shipment{
		ID:         payload.ID,
		WaybillID:  payload.Courier.WaybillID,
		TrackingID: payload.Courier.TrackingID,
		Status:     payload.Status,
		History:    history,
	}, nil
}

func (b *Biteship) do(method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}

	httpReq, err := http.NewRequest(method, b.baseURL+path, reader)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", b.apiKey)

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("biteship %s %s: status %d: %s", method, path, resp.StatusCode, string(data))
	}

	return json.Unmarshal(data, out)
}

// biteshipRate mirrors the provider payload; it accepts both documented shapes.
type biteshipRate struct {
	CourierName        string `json:"courier_name"`
	CourierCode        string `json:"courier_code"`
	CourierServiceName string `json:"courier_service_name"`
	CourierServiceCode string `json:"courier_service_code"`
	Description        string `json:"description"`
	Duration           string `json:"duration"`
	Price              int    `json:"price"`
	ShippingFee        int    `json:"shipping_fee"`
}

func (r biteshipRate) toRate() Rate {
	price := r.Price
	if price == 0 {
		price = r.ShippingFee
	}

	return Rate{
		Courier:     r.CourierCode,
		CourierName: r.CourierName,
		Service:     r.CourierServiceCode,
		ServiceName: r.CourierServiceName,
		Description: r.Description,
		Duration:    r.Duration,
		Price:       price,
	}
}
