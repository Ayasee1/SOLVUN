import { api } from '@/lib/api'
import type { Product, ProductUpdate } from './data/schema'

export function fetchProducts() {
  return api.get<Product[]>('/admin/products')
}

export function updateProduct(id: number, update: ProductUpdate) {
  return api.patch<Product>(`/admin/products/${id}`, update)
}
