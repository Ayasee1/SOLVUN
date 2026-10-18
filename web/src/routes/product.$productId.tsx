import { useEffect, useState } from 'react'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { ArrowLeft, Minus, Plus } from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { useCart } from '@/features/cart/cart-store'
import { fetchProduct, type Product } from '@/lib/api'

export const Route = createFileRoute('/product/$productId')({
  component: ProductDetail,
})

const rupiah = new Intl.NumberFormat('id-ID', {
  style: 'currency',
  currency: 'IDR',
  maximumFractionDigits: 0,
})

function ProductDetail() {
  const { productId } = Route.useParams()
  const navigate = useNavigate()
  const { add } = useCart()

  const [product, setProduct] = useState<Product | null>(null)
  const [loading, setLoading] = useState(true)
  const [qty, setQty] = useState(1)

  useEffect(() => {
    fetchProduct(Number(productId))
      .then(setProduct)
      .catch((error: unknown) =>
        toast.error(error instanceof Error ? error.message : 'Product not found')
      )
      .finally(() => setLoading(false))
  }, [productId])

  if (loading) {
    return (
      <main className='mx-auto grid max-w-5xl gap-8 px-4 py-10 md:grid-cols-2'>
        <Skeleton className='aspect-square w-full' />
        <div className='space-y-4'>
          <Skeleton className='h-8 w-2/3' />
          <Skeleton className='h-4 w-24' />
          <Skeleton className='h-24 w-full' />
        </div>
      </main>
    )
  }

  if (!product) {
    return (
      <main className='mx-auto max-w-3xl px-4 py-24 text-center'>
        <h1 className='text-xl font-semibold'>Product not found</h1>
        <Link to='/' className='mt-4 inline-block underline underline-offset-4'>
          Back to catalog
        </Link>
      </main>
    )
  }

  const soldOut = product.stock <= 0

  return (
    <main className='mx-auto max-w-5xl px-4 py-10'>
      <Button variant='ghost' size='sm' className='-ms-2 mb-4' asChild>
        <Link to='/'>
          <ArrowLeft className='size-4' />
          Back to catalog
        </Link>
      </Button>

      <div className='grid gap-8 md:grid-cols-2'>
        <div className='bg-muted overflow-hidden rounded-xl border'>
          {product.images?.[0] ? (
            <img
              src={product.images[0]}
              alt={product.name}
              className='h-full w-full object-cover'
            />
          ) : null}
        </div>

        <div>
          <h1 className='text-2xl font-semibold tracking-tight'>
            {product.name}
          </h1>

          <div className='mt-2 flex items-center gap-2'>
            <span className='text-xl font-semibold tabular-nums'>
              {rupiah.format(product.price)}
            </span>
            <Badge
              variant='outline'
              className={
                soldOut
                  ? 'bg-muted text-muted-foreground'
                  : 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400'
              }
            >
              {soldOut ? 'Sold out' : `${product.stock} in stock`}
            </Badge>
          </div>

          <p className='text-muted-foreground mt-4 text-sm leading-relaxed'>
            {product.description}
          </p>

          <Separator className='my-6' />

          <div className='flex items-center gap-4'>
            <div className='flex items-center rounded-lg border'>
              <Button
                variant='ghost'
                size='icon'
                disabled={qty <= 1}
                onClick={() => setQty((value) => Math.max(1, value - 1))}
              >
                <Minus className='size-4' />
              </Button>
              <span className='w-10 text-center tabular-nums'>{qty}</span>
              <Button
                variant='ghost'
                size='icon'
                disabled={qty >= product.stock}
                onClick={() => setQty((value) => Math.min(product.stock, value + 1))}
              >
                <Plus className='size-4' />
              </Button>
            </div>

            <Button
              disabled={soldOut}
              onClick={() => {
                add(
                  {
                    product_id: product.id,
                    name: product.name,
                    price: product.price,
                    weight_gram: product.weight_gram,
                    image: product.images?.[0],
                  },
                  qty
                )
                toast.success(`${product.name} added to cart`)
                void navigate({ to: '/cart' })
              }}
            >
              {soldOut ? 'Sold out' : 'Add to cart'}
            </Button>
          </div>

          <p className='text-muted-foreground mt-6 text-xs'>
            Shipping is calculated at checkout based on your postal code.
          </p>
        </div>
      </div>
    </main>
  )
}
