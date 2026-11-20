import { createFileRoute, Link } from '@tanstack/react-router'
import { ArrowLeft, ImageIcon, Minus, Plus, ShoppingBag, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { OrderSummary } from '@/components/checkout/order-summary'
import { useCart } from '@/features/cart/cart-store'
import { formatRupiah } from '@/lib/format'

export const Route = createFileRoute('/cart')({ component: CartPage })

function CartPage() {
  const { items, subtotal, count, setQty, remove } = useCart()

  if (items.length === 0) {
    return (
      <main className='mx-auto max-w-3xl px-4 py-24 text-center'>
        <div className='bg-muted mx-auto grid size-14 place-items-center rounded-2xl'>
          <ShoppingBag className='text-muted-foreground size-6' />
        </div>
        <h1 className='mt-6 text-2xl font-semibold'>Your cart is empty</h1>
        <p className='text-muted-foreground mt-2 text-sm'>
          Add a product to get started.
        </p>
        <Button className='mt-6' asChild>
          <Link to='/'>Browse catalog</Link>
        </Button>
      </main>
    )
  }

  return (
    <main className='mx-auto max-w-6xl px-4 py-10 lg:py-14'>
      <div>
        <Link
          to='/'
          className='text-muted-foreground hover:text-foreground inline-flex items-center gap-1.5 text-sm transition-colors'
        >
          <ArrowLeft className='size-4' />
          Continue shopping
        </Link>
        <h1 className='mt-4 text-3xl font-semibold tracking-tight sm:text-4xl'>
          Your cart
        </h1>
        <p className='text-muted-foreground mt-1.5 text-sm'>
          {count} {count === 1 ? 'item' : 'items'} ready for checkout.
        </p>
      </div>

      <div className='mt-8 grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_380px] lg:gap-8'>
        <div className='overflow-hidden rounded-2xl border bg-card shadow-xs'>
          <ul className='divide-y'>
            {items.map((item) => (
              <li
                key={item.product_id}
                className='flex gap-4 p-4 sm:gap-5 sm:p-5'
              >
                <Link
                  to='/product/$productId'
                  params={{ productId: String(item.product_id) }}
                  className='bg-muted grid aspect-[2/3] w-16 shrink-0 place-items-center overflow-hidden rounded-xl border sm:w-20'
                >
                  {item.image ? (
                    <img
                      src={item.image}
                      alt={item.name}
                      className='h-full w-full object-cover'
                    />
                  ) : (
                    <ImageIcon className='text-muted-foreground size-5' />
                  )}
                </Link>

                <div className='flex min-w-0 flex-1 flex-col gap-3'>
                  <div className='flex items-start justify-between gap-3'>
                    <div className='min-w-0'>
                      <Link
                        to='/product/$productId'
                        params={{ productId: String(item.product_id) }}
                        className='truncate text-sm font-medium hover:underline sm:text-base'
                      >
                        {item.name}
                      </Link>
                      <p className='text-muted-foreground mt-0.5 text-xs tabular-nums'>
                        {formatRupiah(item.price)} each
                      </p>
                    </div>
                    <Button
                      variant='ghost'
                      size='icon'
                      className='text-muted-foreground hover:text-destructive -mt-1 -me-1'
                      aria-label={`Remove ${item.name}`}
                      onClick={() => remove(item.product_id)}
                    >
                      <Trash2 className='size-4' />
                    </Button>
                  </div>

                  <div className='mt-auto flex items-center justify-between gap-3'>
                    <div className='flex items-center rounded-lg border'>
                      <Button
                        variant='ghost'
                        size='icon'
                        className='size-9 rounded-e-none'
                        aria-label={`Decrease quantity of ${item.name}`}
                        disabled={item.qty <= 1}
                        onClick={() => setQty(item.product_id, item.qty - 1)}
                      >
                        <Minus className='size-4' />
                      </Button>
                      <span className='w-9 text-center text-sm tabular-nums'>
                        {item.qty}
                      </span>
                      <Button
                        variant='ghost'
                        size='icon'
                        className='size-9 rounded-s-none'
                        aria-label={`Increase quantity of ${item.name}`}
                        onClick={() => setQty(item.product_id, item.qty + 1)}
                      >
                        <Plus className='size-4' />
                      </Button>
                    </div>

                    <span className='text-sm font-semibold tabular-nums sm:text-base'>
                      {formatRupiah(item.price * item.qty)}
                    </span>
                  </div>
                </div>
              </li>
            ))}
          </ul>
        </div>

        <div className='space-y-3 lg:sticky lg:top-24'>
          <OrderSummary
            items={items.map((item) => ({
              key: item.product_id,
              name: item.name,
              price: item.price,
              qty: item.qty,
              image: item.image,
            }))}
            subtotal={subtotal}
            shipping={null}
            shippingNote='calculated at checkout'
            total={subtotal}
            footer={
              <Button className='h-11 w-full' asChild>
                <Link to='/checkout'>Proceed to checkout</Link>
              </Button>
            }
          />
          <Button variant='outline' className='w-full' asChild>
            <Link to='/'>Continue shopping</Link>
          </Button>
          <p className='text-muted-foreground text-center text-xs'>
            Shipping is calculated at checkout based on your postal code.
          </p>
        </div>
      </div>
    </main>
  )
}
