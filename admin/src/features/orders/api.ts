import { api } from '@/lib/api'
import type { Order, OrderStatus } from './data/schema'

export type Page<T> = {
  items: T[]
  total: number
  limit: number
  offset: number
}

export type OrderStats = {
  orders_total: number
  revenue: number
  by_status: Partial<Record<OrderStatus, number>>
}

export function fetchOrders(
  status: OrderStatus | 'all',
  limit = 25,
  offset = 0
) {
  const params = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
  })
  if (status !== 'all') params.set('status', status)
  return api.get<Page<Order>>(`/admin/orders?${params.toString()}`)
}

// fetchOrderStats returns the dashboard aggregate computed by the API, so the
// dashboard does not need to load every order.
export function fetchOrderStats() {
  return api.get<OrderStats>('/admin/stats')
}

export function fetchOrder(id: string) {
  return api.get<Order>(`/admin/orders/${id}`)
}

// cancel goes through the admin route (the guest route needs the order token)
export function cancelOrder(id: string) {
  return api.post<Order>(`/admin/orders/${id}/cancel`)
}

// retryShipment asks the API to create the provider shipment for a paid order
// whose shipment creation failed.
export function retryShipment(id: string) {
  return api.post<Order>(`/admin/orders/${id}/shipment`)
}

// refundOrder asks the API to refund the captured payment of a needs_refund
// order. Only full refunds are supported.
export function refundOrder(id: string) {
  return api.post<Order>(`/admin/orders/${id}/refund`)
}
