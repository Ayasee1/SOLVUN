import { createRootRoute, Link, Outlet, useRouterState } from '@tanstack/react-router'
import { Lock } from 'lucide-react'
import { CheckoutHeader, StoreHeader } from '@/components/layout/store-header'
import { Toaster } from '@/components/ui/sonner'
import { CartProvider } from '@/features/cart/cart-store'

export const Route = createRootRoute({
  component: RootLayout,
  notFoundComponent: NotFound,
})

function RootLayout() {
  const pathname = useRouterState({
    select: (state) => state.location.pathname,
  })
  // checkout runs in a minimal "enclosed" shell: no navigation that could pull
  // the customer away mid-purchase
  const enclosed = pathname.startsWith('/checkout')
  const isHome = pathname === '/'

  return (
    <CartProvider>
      <div className='bg-background text-foreground flex min-h-svh flex-col'>
        {enclosed ? <CheckoutHeader /> : <StoreHeader />}

        <div className='flex-1'>
          <Outlet />
        </div>

        {!enclosed && !isHome && (
          <footer className='bg-foreground text-background mt-auto'>
            <div className='mx-auto max-w-6xl px-4 pt-14 pb-8'>
              <div className='text-background/40 flex items-end justify-between text-[9px] tracking-[0.18em] uppercase'>
                <span>After Hours — Vol. 01</span>
                <span>{new Date().getFullYear()}</span>
              </div>

              <img
                src='/images/brand/solvun-wordmark.png'
                alt='Solvun'
                loading='lazy'
                decoding='async'
                className='mt-4 h-auto w-full'
              />

              <div className='mt-6 flex flex-col gap-4 border-t border-background/10 pt-4 text-[9px] tracking-[0.14em] uppercase sm:flex-row sm:items-center sm:justify-between'>
                <nav
                  aria-label='Footer'
                  className='flex flex-wrap items-center gap-x-7 gap-y-3'
                >
                  <Link
                    to='/shop'
                    className='text-background/60 hover:text-background transition-colors'
                  >
                    Shop
                  </Link>
                  <a
                    href='/track'
                    className='text-background/60 hover:text-background transition-colors'
                  >
                    Track order
                  </a>
                  <Link
                    to='/cart'
                    className='text-background/60 hover:text-background transition-colors'
                  >
                    Cart
                  </Link>
                </nav>

                <p className='text-background/40 flex items-center gap-1.5'>
                  <Lock className='size-3' />
                  © Solvun — Secure payments by Midtrans
                </p>
              </div>
            </div>
          </footer>
        )}

        <Toaster duration={4000} />
      </div>
    </CartProvider>
  )
}

function NotFound() {
  return (
    <div className='mx-auto max-w-3xl px-4 py-24 text-center'>
      <p className='text-muted-foreground text-xs font-semibold tracking-[0.18em] uppercase'>
        404
      </p>
      <h1 className='mt-3 font-editorial text-3xl uppercase'>Page not found</h1>
      <p className='text-muted-foreground mt-2 text-sm'>
        The page you are looking for does not exist.
      </p>
      <Link
        to='/'
        className='text-foreground mt-6 inline-block text-sm underline underline-offset-4'
      >
        Back to store
      </Link>
    </div>
  )
}
