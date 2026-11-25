import { api } from '@/lib/api'
import type { Product, ProductInput, ProductUpdate } from './data/schema'

export function fetchProducts() {
  return api.get<Product[]>('/admin/products')
}

export function createProduct(input: ProductInput) {
  return api.post<Product>('/admin/products', input)
}

// uploadProductImage stores one file and returns the URL to put in the product.
export function uploadProductImage(file: File) {
  const form = new FormData()
  form.append('file', file)
  return api.upload<{ url: string }>('/admin/products/images', form)
}

export function updateProduct(id: number, update: ProductUpdate) {
  return api.patch<Product>(`/admin/products/${id}`, update)
}
