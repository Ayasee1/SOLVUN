import { Link } from '@tanstack/react-router'
import { ArrowLeft, Lock, ShoppingBag } from 'lucide-react'
import { useCart } from '@/features/cart/cart-store'

export function StoreLogo() {
  return (
    <Link to='/' aria-label='Solvun home' className='inline-flex items-center'>
      <img
        src='/images/brand/solvun-wordmark-sm.png'
        alt='Solvun'
        className='h-6 w-auto sm:h-7'
      />
    </Link>
  )
}

// Checkout runs in a minimal "enclosed" shell: no navigation that can pull the
// customer away mid-purchase.
export function CheckoutHeader() {
  return (
    <header className='bg-foreground text-background sticky top-0 z-40 border-b border-background/10'>
      <div className='mx-auto flex h-16 max-w-6xl items-center justify-between gap-4 px-4'>
        <StoreLogo />

        <div className='flex items-center gap-4'>
          <Link
            to='/cart'
            className='text-background/60 hover:text-background flex items-center gap-1.5 text-[11px] tracking-[0.14em] uppercase transition-colors'
          >
            <ArrowLeft className='size-3.5' />
            Back
          </Link>
          <span className='text-background/60 flex items-center gap-1.5 text-[11px] tracking-[0.14em] uppercase'>
            <Lock className='size-3.5' />
            <span className='hidden sm:inline'>Secure</span>
          </span>
        </div>
      </div>
    </header>
  )
}

export function StoreHeader() {
  const { count } = useCart()

  return (
    <header className='bg-foreground text-background sticky top-0 z-40 border-b border-background/10'>
      <div className='mx-auto grid h-16 max-w-6xl grid-cols-[1fr_auto_1fr] items-center px-4'>
        <div className='justify-self-start'>
          <StoreLogo />
        </div>

        <nav
          aria-label='Main'
          className='flex items-center gap-7 justify-self-center text-[10px] tracking-[0.16em] uppercase'
        >
          <Link
            to='/shop'
            className='text-background/60 hover:text-background transition-colors'
          >
            Shop
          </Link>
          <a
            href='/track'
            className='text-background/60 hover:text-background hidden transition-colors sm:inline'
          >
            Track order
          </a>
        </nav>

        <Link
          to='/cart'
          aria-label={`Cart, ${count} items`}
          className='text-background/80 hover:text-background col-start-3 flex items-center gap-1.5 justify-self-end text-[10px] tracking-[0.16em] uppercase transition-colors'
        >
          <ShoppingBag className='size-4' />
          <span className='hidden sm:inline'>Cart</span>
          <span className='tabular-nums'>({count})</span>
        </Link>
      </div>
    </header>
  )
}
