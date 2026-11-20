import { useEffect, useRef, useState } from 'react'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { ChevronLeft, ChevronRight, ImageIcon, Minus, Plus } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useCart } from '@/features/cart/cart-store'
import { fetchProduct, fetchProducts, type Product } from '@/lib/api'
import { formatRupiah } from '@/lib/format'

export const Route = createFileRoute('/product/$productId')({
  component: ProductDetail,
})

function ProductDetail() {
  const { productId } = Route.useParams()
  const navigate = useNavigate()
  const { add } = useCart()

  const [product, setProduct] = useState<Product | null>(null)
  const [related, setRelated] = useState<Product[]>([])
  const [loading, setLoading] = useState(true)
  const [qty, setQty] = useState(1)
  const [active, setActive] = useState(0)
  const touchStart = useRef<number | null>(null)

  useEffect(() => {
    const id = Number(productId)

    setLoading(true)
    setProduct(null)
    setRelated([])
    setQty(1)
    setActive(0)

    fetchProduct(id)
      .then(setProduct)
      .catch((error: unknown) =>
        toast.error(error instanceof Error ? error.message : 'Product not found')
      )
      .finally(() => setLoading(false))

    fetchProducts()
      .then((all) => setRelated(all.filter((item) => item.id !== id).slice(0, 3)))
      .catch(() => setRelated([]))
  }, [productId])

  if (loading) {
    return (
      <main className='mx-auto max-w-6xl px-4 py-8 lg:py-12'>
        <Skeleton className='h-3 w-40' />
        <div className='mt-6 grid gap-4 lg:grid-cols-[1.25fr_0.75fr] lg:gap-8'>
          <div className='bg-muted border-foreground/10 relative aspect-[2/3] overflow-hidden border'>
            <Skeleton className='h-full w-full rounded-none' />
          </div>
          <Skeleton className='h-[520px] w-full rounded-none' />
        </div>
      </main>
    )
  }

  if (!product) {
    return (
      <main className='mx-auto max-w-3xl px-4 py-24 text-center'>
        <p className='text-muted-foreground text-[10px] tracking-[0.18em] uppercase'>
          404
        </p>
        <h1 className='mt-3 font-editorial text-3xl uppercase'>
          Product not found
        </h1>
        <p className='text-muted-foreground mt-2 text-sm'>
          This piece may have been removed from the drop.
        </p>
        <Button className='mt-6 rounded-full px-6' asChild>
          <Link to='/'>Back to shop</Link>
        </Button>
      </main>
    )
  }

  const current = product
  const soldOut = current.stock <= 0
  const lowStock = !soldOut && current.stock <= 5
  const images = current.images ?? []

  function addToBag(then?: '/cart' | '/checkout') {
    add(
      {
        product_id: current.id,
        name: current.name,
        price: current.price,
        weight_gram: current.weight_gram,
        image: current.images?.[0],
      },
      qty
    )
    toast.success(`${current.name} added to bag`)
    if (then) void navigate({ to: then })
  }

  return (
    <main className='mx-auto max-w-6xl px-4 py-8 lg:py-12'>
      <nav
        aria-label='Breadcrumb'
        className='text-muted-foreground flex items-center gap-2 text-[10px] tracking-[0.14em] uppercase'
      >
        <Link to='/' className='hover:text-foreground transition-colors'>
          Shop
        </Link>
        <span aria-hidden>/</span>
        <span className='text-foreground truncate'>{current.name}</span>
      </nav>

      <div className='mt-6 grid gap-4 lg:grid-cols-[1.25fr_0.75fr] lg:items-start lg:gap-8'>
        <div>
          {images.length > 0 ? (
            <div
              className='bg-muted border-foreground/10 relative aspect-[2/3] overflow-hidden border'
              onTouchStart={(event) => {
                touchStart.current = event.touches[0]?.clientX ?? null
              }}
              onTouchEnd={(event) => {
                if (touchStart.current == null) return
                const delta =
                  (event.changedTouches[0]?.clientX ?? touchStart.current) -
                  touchStart.current
                if (delta > 40) {
                  setActive((value) => (value - 1 + images.length) % images.length)
                } else if (delta < -40) {
                  setActive((value) => (value + 1) % images.length)
                }
                touchStart.current = null
              }}
            >
              <div
                className='flex h-full transition-transform duration-500 ease-out'
                style={{ transform: `translateX(-${active * 100}%)` }}
              >
                {images.map((image) => (
                  <img
                    key={image}
                    src={image}
                    alt={current.name}
                    className='h-full w-full shrink-0 object-cover'
                  />
                ))}
              </div>

              {images.length > 1 && (
                <>
                  <button
                    type='button'
                    aria-label='Previous image'
                    onClick={() =>
                      setActive((value) => (value - 1 + images.length) % images.length)
                    }
                    className='bg-background/80 text-foreground hover:bg-background absolute top-1/2 left-3 grid size-9 -translate-y-1/2 place-items-center rounded-full backdrop-blur transition-colors'
                  >
                    <ChevronLeft className='size-4' />
                  </button>
                  <button
                    type='button'
                    aria-label='Next image'
                    onClick={() =>
                      setActive((value) => (value + 1) % images.length)
                    }
                    className='bg-background/80 text-foreground hover:bg-background absolute top-1/2 right-3 grid size-9 -translate-y-1/2 place-items-center rounded-full backdrop-blur transition-colors'
                  >
                    <ChevronRight className='size-4' />
                  </button>
                  <span className='bg-background/80 text-foreground absolute right-3 bottom-3 px-2 py-1 text-[9px] tracking-[0.14em] tabular-nums uppercase backdrop-blur'>
                    {String(active + 1).padStart(2, '0')} /{' '}
                    {String(images.length).padStart(2, '0')}
                  </span>
                </>
              )}
            </div>
          ) : (
            <div className='bg-muted grid aspect-[2/3] place-items-center'>
              <ImageIcon className='text-muted-foreground size-8' />
            </div>
          )}
        </div>

        <aside className='bg-foreground text-background grain animate-in fade-in relative overflow-hidden p-6 duration-700 sm:p-8 lg:sticky lg:top-24'>
          <span className='text-background/10 pointer-events-none absolute top-2 right-4 z-0 font-editorial text-7xl leading-none select-none'>
            {String(current.id).slice(-2)}
          </span>

          <div className='relative z-10'>
            <p className='text-background/50 flex items-center gap-2 text-[9px] tracking-[0.18em] uppercase'>
            <span className='bg-brand size-1.5 rounded-full' />
            Solvun — After Hours
          </p>

          <h1 className='mt-4 font-editorial text-4xl leading-[0.95] uppercase sm:text-5xl'>
            {current.name}
          </h1>

          <p className='mt-4 text-sm tabular-nums'>
            {formatRupiah(current.price)}
          </p>
          <p className='text-background/50 mt-1 text-[10px] tracking-[0.14em] uppercase'>
            {soldOut
              ? 'Sold out'
              : lowStock
                ? `Only ${current.stock} left`
                : 'In stock'}
          </p>

          {current.description ? (
            <p className='text-background/60 mt-6 border-t border-background/15 pt-5 text-sm leading-7'>
              {current.description}
            </p>
          ) : null}

          <div className='mt-6 flex items-center justify-between gap-4 border-t border-background/15 pt-5'>
            <span className='text-background/50 text-[9px] tracking-[0.16em] uppercase'>
              Quantity
            </span>
            <div className='flex items-center border border-background/20'>
              <button
                type='button'
                aria-label='Decrease quantity'
                disabled={qty <= 1}
                onClick={() => setQty((value) => Math.max(1, value - 1))}
                className='text-background/70 hover:text-background grid size-9 place-items-center transition-colors disabled:opacity-30'
              >
                <Minus className='size-4' />
              </button>
              <span className='w-9 text-center text-sm tabular-nums'>{qty}</span>
              <button
                type='button'
                aria-label='Increase quantity'
                disabled={qty >= current.stock}
                onClick={() =>
                  setQty((value) => Math.min(current.stock, value + 1))
                }
                className='text-background/70 hover:text-background grid size-9 place-items-center transition-colors disabled:opacity-30'
              >
                <Plus className='size-4' />
              </button>
            </div>
          </div>

          <Button
            className='bg-background text-foreground hover:bg-background/90 mt-4 h-11 w-full rounded-none text-[10px] tracking-[0.18em] uppercase'
            disabled={soldOut}
            onClick={() => addToBag('/cart')}
          >
            {soldOut ? 'Sold out' : 'Add to bag'}
          </Button>

          {!soldOut && (
            <Button
              variant='outline'
              className='border-background/25 text-background hover:bg-background hover:text-foreground mt-2 h-11 w-full rounded-none bg-transparent text-[10px] tracking-[0.18em] uppercase'
              onClick={() => addToBag('/checkout')}
            >
              Buy now
            </Button>
          )}

          <div className='text-background/40 mt-6 flex items-center justify-between border-t border-background/15 pt-4 text-[9px] tracking-[0.14em] uppercase'>
            <span>{current.weight_gram} g</span>
            <span>Secure payment</span>
            <span>Tracked delivery</span>
          </div>
          </div>
        </aside>
      </div>

      {related.length > 1 && (
        <section className='mt-20 border-t pt-8'>
          <h2 className='font-editorial text-2xl uppercase'>
            You might also like
          </h2>

          <div className='mt-6 grid gap-6 sm:grid-cols-3'>
            {related.map((item) => (
              <Link
                key={item.id}
                to='/product/$productId'
                params={{ productId: String(item.id) }}
                className='group block'
              >
                <div className='bg-muted border-foreground/10 overflow-hidden border'>
                  {item.images?.[0] ? (
                    <img
                      src={item.images[0]}
                      alt={item.name}
                      className='h-auto w-full transition-transform duration-500 group-hover:scale-[1.02]'
                    />
                  ) : (
                    <div className='grid aspect-[2/3] place-items-center'>
                      <ImageIcon className='text-muted-foreground size-8' />
                    </div>
                  )}
                </div>
                <h3 className='mt-3 font-editorial text-sm uppercase'>
                  {item.name}
                </h3>
                <p className='text-muted-foreground mt-1 text-[10px] tabular-nums'>
                  {formatRupiah(item.price)}
                </p>
              </Link>
            ))}
          </div>
        </section>
      )}
    </main>
  )
}
