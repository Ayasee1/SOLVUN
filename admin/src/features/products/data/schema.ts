export type Product = {
  id: number
  name: string
  description?: string
  images?: string[]
  price: number
  stock: number
  is_available: boolean
  updated_at: string
}

// Only runtime fields can be changed from the admin UI. Name, description and
// images are content owned by the catalog/*.json files.
export type ProductUpdate = {
  price?: number
  stock?: number
  is_available?: boolean
}
