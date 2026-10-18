import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'

export type CartItem = {
  product_id: number
  name: string
  price: number
  qty: number
  weight_gram: number
  image?: string
}

type CartValue = {
  items: CartItem[]
  count: number
  subtotal: number
  add: (item: Omit<CartItem, 'qty'>, qty?: number) => void
  setQty: (productId: number, qty: number) => void
  remove: (productId: number) => void
  clear: () => void
}

const STORAGE_KEY = 'golearn.cart'
const CartContext = createContext<CartValue | null>(null)

export function CartProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<CartItem[]>(() => {
    try {
      const raw = localStorage.getItem(STORAGE_KEY)
      return raw ? (JSON.parse(raw) as CartItem[]) : []
    } catch {
      return []
    }
  })

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(items))
  }, [items])

  const value = useMemo<CartValue>(
    () => ({
      items,
      count: items.reduce((sum, item) => sum + item.qty, 0),
      subtotal: items.reduce((sum, item) => sum + item.price * item.qty, 0),
      add: (item, qty = 1) =>
        setItems((current) => {
          const existing = current.find((i) => i.product_id === item.product_id)
          if (existing) {
            return current.map((i) =>
              i.product_id === item.product_id ? { ...i, qty: i.qty + qty } : i
            )
          }
          return [...current, { ...item, qty }]
        }),
      setQty: (productId, qty) =>
        setItems((current) =>
          qty <= 0
            ? current.filter((i) => i.product_id !== productId)
            : current.map((i) =>
                i.product_id === productId ? { ...i, qty } : i
              )
        ),
      remove: (productId) =>
        setItems((current) =>
          current.filter((i) => i.product_id !== productId)
        ),
      clear: () => setItems([]),
    }),
    [items]
  )

  return <CartContext.Provider value={value}>{children}</CartContext.Provider>
}

export function useCart() {
  const context = useContext(CartContext)
  if (!context) {
    throw new Error('useCart must be used inside CartProvider')
  }
  return context
}
