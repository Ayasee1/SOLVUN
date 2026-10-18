import { createRootRoute, Link, Outlet, useRouterState } from '@tanstack/react-router'
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

  return (
    <CartProvider>
      <div className='bg-background text-foreground flex min-h-svh flex-col'>
        {enclosed ? <CheckoutHeader /> : <StoreHeader />}

        <div className='flex-1'>
          <Outlet />
        </div>

        {!enclosed && (
          <footer className='text-muted-foreground border-t py-8 text-center text-xs'>
            <div className='mx-auto max-w-5xl px-4'>
              Golearn Store ·{' '}
              <a href='/track' className='underline underline-offset-4'>
                Track your order
              </a>
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
      <h1 className='text-2xl font-semibold'>Page not found</h1>
      <p className='text-muted-foreground mt-2 text-sm'>
        The page you are looking for does not exist.
      </p>
      <Link to='/' className='mt-6 inline-block underline underline-offset-4'>
        Back to store
      </Link>
    </div>
  )
}
