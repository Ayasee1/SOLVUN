import { Link } from '@tanstack/react-router'
import { Lock, PackageSearch, ShoppingBag } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useCart } from '@/features/cart/cart-store'

// On checkout we use a minimal "enclosed" header: no navigation that can pull
// the customer away mid-purchase.
export function CheckoutHeader() {
  return (
    <header className='bg-background border-b'>
      <div className='mx-auto flex max-w-5xl items-center gap-2 px-4 py-4'>
        <Link
          to='/'
          className='text-sm font-semibold tracking-[0.14em] uppercase'
        >
          Golearn Store
        </Link>
        <span className='text-muted-foreground ms-auto flex items-center gap-1.5 text-xs'>
          <Lock className='size-3.5' />
          Secure checkout
        </span>
      </div>
    </header>
  )
}

export function StoreHeader() {
  const { count } = useCart()

  return (
    <header className='bg-background/80 sticky top-0 z-20 border-b backdrop-blur'>
      <div className='mx-auto flex max-w-5xl items-center gap-2 px-4 py-3'>
        <Link
          to='/'
          className='text-sm font-semibold tracking-[0.14em] uppercase'
        >
          Golearn Store
        </Link>
        <nav className='ms-auto flex items-center gap-1'>
          <Button variant='ghost' size='sm' asChild>
            <a href='/track'>
              <PackageSearch className='size-4' />
              Track order
            </a>
          </Button>
          <Button variant='ghost' size='sm' asChild>
            <Link to='/cart'>
              <ShoppingBag className='size-4' />
              Cart{count > 0 ? ` (${count})` : ''}
            </Link>
          </Button>
        </nav>
      </div>
    </header>
  )
}
