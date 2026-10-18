import { api } from '@/lib/api'
import type { Order, OrderStatus } from './data/schema'

export function fetchOrders(status: OrderStatus | 'all') {
  const query = status === 'all' ? '' : `?status=${status}`
  return api.get<Order[]>(`/admin/orders${query}`)
}

export function fetchOrder(id: string) {
  return api.get<Order>(`/admin/orders/${id}`)
}

// cancel is part of the guest flow (only pending orders can be cancelled)
export function cancelOrder(id: string) {
  return api.post<Order>(`/orders/${id}/cancel`)
}
