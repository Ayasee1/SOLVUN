import { useEffect, useMemo, useState } from 'react'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { Search } from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { useCart } from '@/features/cart/cart-store'
import { fetchProducts, type Product } from '@/lib/api'

export const Route = createFileRoute('/')({ component: Catalog })

const rupiah = new Intl.NumberFormat('id-ID', {
  style: 'currency',
  currency: 'IDR',
  maximumFractionDigits: 0,
})

function Catalog() {
  const [products, setProducts] = useState<Product[]>([])
  const [loading, setLoading] = useState(true)
  const [query, setQuery] = useState('')
  const { add } = useCart()
  const navigate = useNavigate()

  useEffect(() => {
    fetchProducts()
      .then(setProducts)
      .catch((error: unknown) =>
        toast.error(error instanceof Error ? error.message : 'Failed to load products')
      )
      .finally(() => setLoading(false))
  }, [])

  const visible = useMemo(() => {
    const q = query.trim().toLowerCase()
    const list = q
      ? products.filter((p) => p.name.toLowerCase().includes(q))
      : products
    return [...list].sort((a, b) => a.price - b.price)
  }, [products, query])

  return (
    <main className='mx-auto max-w-5xl px-4 py-10'>
      <div className='flex flex-wrap items-end justify-between gap-4'>
        <div>
          <h1 className='text-2xl font-semibold tracking-tight'>Catalog</h1>
          <p className='text-muted-foreground mt-1 text-sm'>
            Free shipping options are calculated at checkout.
          </p>
        </div>
        <div className='relative w-64'>
          <Search className='text-muted-foreground absolute start-3 top-1/2 size-4 -translate-y-1/2' />
          <Input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder='Search products'
            className='ps-9'
          />
        </div>
      </div>

      <div className='mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-3'>
        {loading &&
          Array.from({ length: 6 }).map((_, index) => (
            <Card key={index} className='overflow-hidden'>
              <Skeleton className='h-40 w-full rounded-none' />
              <CardContent className='space-y-3 pt-4'>
                <Skeleton className='h-4 w-32' />
                <Skeleton className='h-8 w-full' />
              </CardContent>
            </Card>
          ))}

        {!loading &&
          visible.map((product) => (
            <Card key={product.id} className='flex flex-col overflow-hidden'>
              <Link
                to='/product/$productId'
                params={{ productId: String(product.id) }}
                className='bg-muted block aspect-[3/2] overflow-hidden'
              >
                {product.images?.[0] ? (
                  <img
                    src={product.images[0]}
                    alt={product.name}
                    className='h-full w-full object-cover transition-transform duration-300 hover:scale-[1.03]'
                  />
                ) : null}
              </Link>
              <CardContent className='flex flex-1 flex-col gap-3 pt-4'>
                <div className='flex items-start justify-between gap-2'>
                  <Link
                    to='/product/$productId'
                    params={{ productId: String(product.id) }}
                    className='font-medium hover:underline'
                  >
                    {product.name}
                  </Link>
                  {product.stock <= 5 && product.stock > 0 && (
                    <Badge variant='outline' className='bg-amber-500/10 text-amber-700 dark:text-amber-400'>
                      Low stock
                    </Badge>
                  )}
                </div>
                <p className='text-muted-foreground line-clamp-2 text-sm'>
                  {product.description}
                </p>
                <div className='mt-auto flex items-center justify-between gap-2'>
                  <span className='font-semibold tabular-nums'>
                    {rupiah.format(product.price)}
                  </span>
                  <Button
                    size='sm'
                    disabled={product.stock <= 0}
                    onClick={() => {
                      add({
                        product_id: product.id,
                        name: product.name,
                        price: product.price,
                        weight_gram: product.weight_gram,
                        image: product.images?.[0],
                      })
                      toast.success(`${product.name} added to cart`, {
                        action: {
                          label: 'View cart',
                          onClick: () => void navigate({ to: '/cart' }),
                        },
                      })
                    }}
                  >
                    {product.stock <= 0 ? 'Sold out' : 'Add to cart'}
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}

        {!loading && visible.length === 0 && (
          <p className='text-muted-foreground col-span-full py-16 text-center text-sm'>
            No products match your search.
          </p>
        )}
      </div>
    </main>
  )
}
