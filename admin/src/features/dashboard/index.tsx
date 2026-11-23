import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { queryWithToast } from '@/lib/query'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { ConfigDrawer } from '@/components/config-drawer'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { fetchOrders } from '@/features/orders/api'
import { orderStatuses } from '@/features/orders/data/schema'
import { fetchProducts } from '@/features/products/api'
import { OrdersByStatusChart } from './components/overview'
import { RecentOrders } from './components/recent-sales'

const rupiah = new Intl.NumberFormat('id-ID', {
  style: 'currency',
  currency: 'IDR',
  maximumFractionDigits: 0,
})

export function Dashboard() {
  const ordersQuery = useQuery({
    queryKey: ['orders', 'all'],
    queryFn: () => queryWithToast(() => fetchOrders('all')),
    retry: false,
  })
  const productsQuery = useQuery({
    queryKey: ['products'],
    queryFn: () => queryWithToast(fetchProducts),
    retry: false,
  })

  const orders = ordersQuery.data ?? []
  const products = productsQuery.data ?? []
  const loading = ordersQuery.isLoading || productsQuery.isLoading

  const stats = useMemo(() => {
    const paid = orders.filter((order) => order.status === 'paid')
    return {
      revenue: paid.reduce((sum, order) => sum + order.total, 0),
      paidCount: paid.length,
      orders: orders.length,
      pending: orders.filter((order) => order.status === 'pending').length,
      needsRefund: orders.filter((order) => order.status === 'needs_refund')
        .length,
      products: products.length,
      hidden: products.filter((product) => !product.is_available).length,
      lowStock: products.filter(
        (product) => product.is_available && product.stock <= 5
      ).length,
    }
  }, [orders, products])

  const statusData = useMemo(
    () =>
      orderStatuses.map((status) => ({
        status,
        count: orders.filter((order) => order.status === status).length,
      })),
    [orders]
  )

  return (
    <>
      <Header fixed>
        <Search className='me-auto' />
        <ThemeSwitch />
        <ConfigDrawer />
        <ProfileDropdown />
      </Header>

      <Main className='flex flex-1 flex-col gap-4 sm:gap-6'>
        <div className='flex flex-wrap items-end justify-between gap-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>Dashboard</h2>
            <p className='text-muted-foreground'>
              Live numbers straight from the store database.
            </p>
          </div>
          <Button
            variant='outline'
            onClick={() => {
              void ordersQuery.refetch()
              void productsQuery.refetch()
            }}
          >
            Refresh
          </Button>
        </div>

        <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-4'>
          {loading
            ? Array.from({ length: 4 }).map((_, index) => (
                <Card key={index}>
                  <CardHeader className='pb-2'>
                    <Skeleton className='h-4 w-24' />
                  </CardHeader>
                  <CardContent>
                    <Skeleton className='h-7 w-28' />
                  </CardContent>
                </Card>
              ))
            : null}

          {!loading && (
            <>
              <Card>
                <CardHeader className='pb-2'>
                  <CardDescription>Revenue</CardDescription>
                  <CardTitle className='text-2xl tabular-nums'>
                    {rupiah.format(stats.revenue)}
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <p className='text-xs text-muted-foreground'>
                    from {stats.paidCount} paid order
                    {stats.paidCount === 1 ? '' : 's'}
                  </p>
                </CardContent>
              </Card>

              <Card>
                <CardHeader className='pb-2'>
                  <CardDescription>Orders</CardDescription>
                  <CardTitle className='text-2xl tabular-nums'>
                    {stats.orders}
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <p className='text-xs text-muted-foreground'>
                    {stats.pending} pending
                  </p>
                </CardContent>
              </Card>

              <Card
                className={
                  stats.needsRefund > 0 ? 'border-red-500/50' : undefined
                }
              >
                <CardHeader className='pb-2'>
                  <CardDescription>Needs refund</CardDescription>
                  <CardTitle
                    className={`text-2xl tabular-nums ${stats.needsRefund > 0 ? 'text-red-600 dark:text-red-400' : ''}`}
                  >
                    {stats.needsRefund}
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <p className='text-xs text-muted-foreground'>
                    paid orders that could not be fulfilled
                  </p>
                </CardContent>
              </Card>

              <Card>
                <CardHeader className='pb-2'>
                  <CardDescription>Products</CardDescription>
                  <CardTitle className='text-2xl tabular-nums'>
                    {stats.products}
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <p className='text-xs text-muted-foreground'>
                    {stats.lowStock} low stock · {stats.hidden} hidden
                  </p>
                </CardContent>
              </Card>
            </>
          )}
        </div>

        <div className='grid gap-4 lg:grid-cols-7'>
          <Card className='lg:col-span-4'>
            <CardHeader>
              <CardTitle>Orders by status</CardTitle>
              <CardDescription>
                How many orders are in each stage.
              </CardDescription>
            </CardHeader>
            <CardContent className='ps-2'>
              <OrdersByStatusChart data={statusData} />
            </CardContent>
          </Card>

          <Card className='lg:col-span-3'>
            <CardHeader>
              <CardTitle>Recent orders</CardTitle>
              <CardDescription>The latest orders placed.</CardDescription>
            </CardHeader>
            <CardContent>
              <RecentOrders orders={orders.slice(0, 5)} />
            </CardContent>
          </Card>
        </div>
      </Main>
    </>
  )
}
