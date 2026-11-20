import type { ReactNode } from 'react'
import { ImageIcon, ShoppingBag } from 'lucide-react'
import { Separator } from '@/components/ui/separator'
import { cn } from '@/lib/utils'
import { formatRupiah } from '@/lib/format'

export type OrderSummaryItem = {
  key: string | number
  name: string
  price: number
  qty: number
  image?: string
}

type OrderSummaryProps = {
  items: OrderSummaryItem[]
  subtotal: number
  shipping?: number | null
  shippingNote?: string
  total: number
  footer?: ReactNode
  className?: string
  showHeader?: boolean
}

/**
 * Sticky-friendly order summary used by the cart and checkout. Pricing is
 * display-only: the server always recomputes prices and shipping.
 */
export function OrderSummary({
  items,
  subtotal,
  shipping,
  shippingNote,
  total,
  footer,
  className,
  showHeader = true,
}: OrderSummaryProps) {
  const count = items.reduce((sum, item) => sum + item.qty, 0)

  return (
    <section
      data-slot='order-summary'
      aria-label='Order summary'
      className={cn(
        'overflow-hidden rounded-2xl border bg-card shadow-xs',
        className
      )}
    >
      {showHeader ? (
        <div className='flex items-center gap-2.5 border-b px-5 py-4'>
          <ShoppingBag className='text-muted-foreground size-4' />
          <h2 className='font-display text-sm font-semibold'>
            Order summary
          </h2>
          <span className='text-muted-foreground ms-auto text-xs tabular-nums'>
            {count} {count === 1 ? 'item' : 'items'}
          </span>
        </div>
      ) : null}

      <div className='p-5'>
        <ul className='space-y-4'>
          {items.map((item) => (
            <li key={item.key} className='flex items-center gap-3'>
              <div className='relative shrink-0'>
                <div className='bg-muted grid aspect-[2/3] w-12 place-items-center overflow-hidden rounded-xl border'>
                  {item.image ? (
                    <img
                      src={item.image}
                      alt=''
                      className='h-full w-full object-cover'
                    />
                  ) : (
                    <ImageIcon className='text-muted-foreground size-5' />
                  )}
                </div>
                <span className='bg-foreground text-background absolute -top-1.5 -right-1.5 grid size-5 place-items-center rounded-full text-[10px] font-semibold tabular-nums'>
                  {item.qty}
                </span>
              </div>

              <div className='min-w-0 flex-1'>
                <p className='truncate text-sm font-medium'>{item.name}</p>
                <p className='text-muted-foreground text-xs tabular-nums'>
                  {formatRupiah(item.price)} each
                </p>
              </div>

              <span className='text-sm font-medium tabular-nums'>
                {formatRupiah(item.price * item.qty)}
              </span>
            </li>
          ))}
        </ul>

        <Separator className='my-5' />

        <dl className='space-y-2.5 text-sm'>
          <div className='flex items-center justify-between'>
            <dt className='text-muted-foreground'>Subtotal</dt>
            <dd className='tabular-nums'>{formatRupiah(subtotal)}</dd>
          </div>
          <div className='flex items-center justify-between'>
            <dt className='text-muted-foreground'>Shipping</dt>
            <dd className='tabular-nums'>
              {shipping == null ? (
                <span className='text-muted-foreground text-xs'>
                  {shippingNote ?? 'Calculated next'}
                </span>
              ) : shipping === 0 ? (
                'Free'
              ) : (
                formatRupiah(shipping)
              )}
            </dd>
          </div>
        </dl>

        <Separator className='my-5' />

        <div className='flex items-baseline justify-between gap-3'>
          <span className='font-display text-base font-semibold'>Total</span>
          <span className='font-display text-xl font-semibold tracking-tight tabular-nums'>
            {formatRupiah(total)}
          </span>
        </div>

        {footer ? <div className='mt-5'>{footer}</div> : null}
      </div>
    </section>
  )
}
