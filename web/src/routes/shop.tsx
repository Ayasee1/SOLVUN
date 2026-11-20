import { useEffect, useState } from 'react'
import { createFileRoute, Link } from '@tanstack/react-router'
import { ImageIcon, Plus } from 'lucide-react'
import { toast } from 'sonner'
import { Skeleton } from '@/components/ui/skeleton'
import { useCart } from '@/features/cart/cart-store'
import { fetchProducts, type Product } from '@/lib/api'
import { formatRupiah } from '@/lib/format'

export const Route = createFileRoute('/shop')({ component: Shop })

function Shop() {
  const [products, setProducts] = useState<Product[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetchProducts()
      .then(setProducts)
      .catch((error: unknown) =>
        toast.error(error instanceof Error ? error.message : 'Failed to load products')
      )
      .finally(() => setLoading(false))
  }, [])

  return (
    <main className='mx-auto max-w-6xl px-4 pt-10 pb-20 sm:pt-14'>
      <div className='animate-in fade-in slide-in-from-bottom-4 flex items-end justify-between gap-6 border-b pb-6 duration-700'>
        <div>
          <p className='text-muted-foreground flex items-center gap-2 text-[9px] tracking-[0.18em] uppercase'>
            <span className='bg-brand size-1.5 rounded-full' />
            Shop — Vol. 01
          </p>
          <h1 className='mt-3 font-editorial text-5xl uppercase sm:text-6xl'>
            Drop 01
          </h1>
        </div>

        <div className='shrink-0 text-right'>
          <p className='text-muted-foreground text-[9px] tracking-[0.18em] uppercase'>
            After Hours
          </p>
          <p className='text-muted-foreground mt-1 text-[9px] tracking-[0.18em] uppercase'>
            {loading ? '—' : `${products.length} pieces`}
          </p>
        </div>
      </div>

      <div className='mt-12 grid gap-x-8 gap-y-16 sm:grid-cols-2'>
        {loading &&
          Array.from({ length: 2 }).map((_, index) => (
            <div key={index}>
              <Skeleton className='aspect-[2/3] w-full rounded-none' />
              <div className='mt-4 flex items-start justify-between gap-4 border-t pt-3'>
                <div className='space-y-2'>
                  <Skeleton className='h-6 w-40' />
                  <Skeleton className='h-3 w-16' />
                </div>
                <Skeleton className='size-10 rounded-full' />
              </div>
            </div>
          ))}

        {!loading &&
          products.map((product, index) => (
            <ProductPlate
              key={product.id}
              product={product}
              index={index}
            />
          ))}
      </div>

      {!loading && products.length === 0 && (
        <p className='text-muted-foreground py-20 text-center font-editorial text-2xl uppercase'>
          No pieces yet
        </p>
      )}
    </main>
  )
}

function ProductPlate({
  product,
  index,
}: {
  product: Product
  index: number
}) {
  const { add } = useCart()
  const soldOut = product.stock <= 0
  const [primary, secondary] = product.images ?? []

  function handleAdd() {
    add({
      product_id: product.id,
      name: product.name,
      price: product.price,
      weight_gram: product.weight_gram,
      image: primary,
    })
    toast.success(`${product.name} added to bag`)
  }

  return (
    <article
      className={
        index === 1
          ? 'group animate-in fade-in slide-in-from-bottom-4 delay-150 duration-700'
          : 'group animate-in fade-in slide-in-from-bottom-4 duration-700'
      }
    >
      <Link
        to='/product/$productId'
        params={{ productId: String(product.id) }}
        className='bg-muted border-foreground/10 relative block aspect-[2/3] overflow-hidden border'
      >
        {primary ? (
          <>
            <img
              src={primary}
              alt={product.name}
              className='absolute inset-0 h-full w-full object-cover transition-opacity duration-500 group-hover:opacity-0'
            />
            {secondary ? (
              <img
                src={secondary}
                alt=''
                className='absolute inset-0 h-full w-full object-cover opacity-0 transition-opacity duration-500 group-hover:opacity-100'
              />
            ) : null}
          </>
        ) : (
          <div className='grid h-full place-items-center'>
            <ImageIcon className='text-muted-foreground size-8' />
          </div>
        )}

        <span className='bg-background/85 text-foreground absolute top-3 left-3 px-2 py-1 text-[9px] tracking-[0.16em] tabular-nums uppercase backdrop-blur'>
          {String(index + 1).padStart(2, '0')}
        </span>
      </Link>

      <div className='mt-4 flex items-start justify-between gap-4 border-t pt-3'>
        <div className='min-w-0'>
          <Link
            to='/product/$productId'
            params={{ productId: String(product.id) }}
            className='font-editorial text-2xl uppercase hover:underline'
          >
            {product.name}
          </Link>
          <p className='text-muted-foreground mt-1 text-[10px] tracking-[0.14em] tabular-nums uppercase'>
            {formatRupiah(product.price)}
          </p>
        </div>

        <button
          type='button'
          aria-label={`Add ${product.name} to bag`}
          disabled={soldOut}
          onClick={handleAdd}
          className='bg-foreground text-background hover:bg-brand grid size-10 shrink-0 place-items-center rounded-full transition-colors disabled:cursor-not-allowed disabled:opacity-40'
        >
          <Plus className='size-4' />
        </button>
      </div>
    </article>
  )
}
