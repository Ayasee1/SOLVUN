import { createFileRoute, Link } from '@tanstack/react-router'
import { Minus, Plus, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { useCart } from '@/features/cart/cart-store'

export const Route = createFileRoute('/cart')({ component: CartPage })

const rupiah = new Intl.NumberFormat('id-ID', {
  style: 'currency',
  currency: 'IDR',
  maximumFractionDigits: 0,
})

function CartPage() {
  const { items, subtotal, setQty, remove } = useCart()

  if (items.length === 0) {
    return (
      <main className='mx-auto max-w-3xl px-4 py-24 text-center'>
        <h1 className='text-xl font-semibold'>Your cart is empty</h1>
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
    <main className='mx-auto max-w-3xl px-4 py-10'>
      <h1 className='text-2xl font-semibold tracking-tight'>Your cart</h1>

      <Card className='mt-6'>
        <CardContent className='divide-y p-0'>
          {items.map((item) => (
            <div key={item.product_id} className='flex items-center gap-4 p-4'>
              {item.image ? (
                <img
                  src={item.image}
                  alt={item.name}
                  className='bg-muted size-16 rounded-lg object-cover'
                />
              ) : null}

              <div className='min-w-0 flex-1'>
                <p className='truncate font-medium'>{item.name}</p>
                <p className='text-muted-foreground text-sm tabular-nums'>
                  {rupiah.format(item.price)}
                </p>
              </div>

              <div className='flex items-center rounded-lg border'>
                <Button
                  variant='ghost'
                  size='icon'
                  onClick={() => setQty(item.product_id, item.qty - 1)}
                >
                  <Minus className='size-4' />
                </Button>
                <span className='w-8 text-center text-sm tabular-nums'>
                  {item.qty}
                </span>
                <Button
                  variant='ghost'
                  size='icon'
                  onClick={() => setQty(item.product_id, item.qty + 1)}
                >
                  <Plus className='size-4' />
                </Button>
              </div>

              <span className='w-24 text-right font-medium tabular-nums'>
                {rupiah.format(item.price * item.qty)}
              </span>

              <Button
                variant='ghost'
                size='icon'
                aria-label={`Remove ${item.name}`}
                onClick={() => remove(item.product_id)}
              >
                <Trash2 className='text-muted-foreground size-4' />
              </Button>
            </div>
          ))}
        </CardContent>
      </Card>

      <div className='mt-6 space-y-2'>
        <div className='flex items-center justify-between text-sm'>
          <span className='text-muted-foreground'>Subtotal</span>
          <span className='tabular-nums'>{rupiah.format(subtotal)}</span>
        </div>
        <div className='text-muted-foreground flex items-center justify-between text-sm'>
          <span>Shipping</span>
          <span>calculated at checkout</span>
        </div>
        <Separator />
        <div className='flex items-center justify-between font-semibold'>
          <span>Total</span>
          <span className='tabular-nums'>{rupiah.format(subtotal)} + shipping</span>
        </div>
      </div>

      <div className='mt-6 flex justify-end gap-2'>
        <Button variant='outline' asChild>
          <Link to='/'>Continue shopping</Link>
        </Button>
        <Button asChild>
          <Link to='/checkout'>Checkout</Link>
        </Button>
      </div>
    </main>
  )
}
