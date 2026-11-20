export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`/api${path}`, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...init?.headers },
  })

  if (!res.ok) {
    let message = res.statusText
    try {
      const body = await res.json()
      if (body?.error) message = body.error
    } catch {
      // no JSON body
    }
    throw new ApiError(res.status, message)
  }

  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, {
      method: 'POST',
      body: body === undefined ? undefined : JSON.stringify(body),
    }),
}

export type Product = {
  id: number
  name: string
  description?: string
  images?: string[]
  price: number
  stock: number
  is_available: boolean
  weight_gram: number
}

export type ShippingRate = {
  courier: string
  courier_name: string
  service: string
  service_name: string
  description?: string
  duration?: string
  price: number
}

export function fetchProducts() {
  return api.get<Product[]>('/products')
}

export function fetchProduct(id: number) {
  return api.get<Product>(`/products/${id}`)
}

export function fetchShippingRates(payload: {
  destination_postal_code: string
  destination_lat?: number
  destination_lon?: number
  items: { product_id: number; qty: number }[]
}) {
  return api.post<{ weight_gram: number; rates: ShippingRate[] }>(
    '/shipping/rates',
    payload
  )
}
