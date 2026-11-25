export const orderStatuses = [
  'pending',
  'paid',
  'expired',
  'cancelled',
  'needs_refund',
  'refunded',
] as const

export type OrderStatus = (typeof orderStatuses)[number]

export type OrderItem = {
  id: number
  name: string
  price: number
  qty: number
  total_price: number
}

export type Address = {
  street: string
  city: string
  state: string
  zip_code: string
  longitude: number
  latitude: number
}

export type PaymentInfo = {
  type: string
  bank?: string
  va_number?: string
  biller_code?: string
  bill_key?: string
  payment_code?: string
  masked_card?: string
  store?: string
  transaction_id?: string
  settled_at?: string
}

export type ShipmentEvent = {
  status: string
  note?: string
  updated_at?: string
}

export type RefundInfo = {
  key: string
  amount: number
  reason?: string
  refunded_at: string
}

export type ShipmentInfo = {
  provider_id?: string
  waybill_id?: string
  tracking_id?: string
  status?: string
  history?: ShipmentEvent[]
}

export type Order = {
  id: string
  username: string
  email: string
  phone: string
  order_item: OrderItem[]
  address: Address
  total: number
  status: OrderStatus
  shipping_cost?: number
  courier?: string
  courier_service?: string
  snap_token?: string
  payment_url?: string
  payment?: PaymentInfo
  shipment?: ShipmentInfo
  refund?: RefundInfo
  created_at: string
}
