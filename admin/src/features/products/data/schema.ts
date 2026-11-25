export type Product = {
  id: number
  name: string
  description?: string
  images?: string[]
  price: number
  stock: number
  is_available: boolean
  weight_gram: number
  updated_at: string
}

// The database owns the catalog now: every field can be edited from the UI.
export type ProductInput = {
  name: string
  description: string
  images: string[]
  price: number
  stock: number
  is_available: boolean
  weight_gram: number
}

export type ProductUpdate = Partial<ProductInput>
