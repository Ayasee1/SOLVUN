import { useCallback, useEffect, useState } from 'react'
import { Link } from '@tanstack/react-router'
import { toast } from 'sonner'
import { ConfigDrawer } from '@/components/config-drawer'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { fetchOrders } from './api'
import { orderStatuses, type Order, type OrderStatus } from './data/schema'
import { orderStatusClass, paymentMethodLabel } from './data/status'

const rupiah = new Intl.NumberFormat('id-ID', {
  style: 'currency',
  currency: 'IDR',
  maximumFractionDigits: 0,
})

let formatter: Intl.DateTimeFormat | undefined

function formatDate(value: string) {
  if (!value) return '-'
  formatter ??= new Intl.DateTimeFormat('id-ID', {
    dateStyle: 'medium',
    timeStyle: 'short',
  })
  return formatter.format(new Date(value))
}

export function Orders() {
  const [orders, setOrders] = useState<Order[]>([])
  const [status, setStatus] = useState<OrderStatus | 'all'>('all')
  const [loading, setLoading] = useState(true)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      setOrders(await fetchOrders(status))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to load orders')
    } finally {
      setLoading(false)
    }
  }, [status])

  useEffect(() => {
    void load()
  }, [load])

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
            <h2 className='text-2xl font-bold tracking-tight'>Orders</h2>
            <p className='text-muted-foreground'>
              Orders from the storefront, newest first.
            </p>
          </div>

          <div className='flex items-center gap-2'>
            <Select
              value={status}
              onValueChange={(value) => setStatus(value as OrderStatus | 'all')}
            >
              <SelectTrigger className='w-44' aria-label='Filter by status'>
                <SelectValue placeholder='All statuses' />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='all'>All statuses</SelectItem>
                {orderStatuses.map((value) => (
                  <SelectItem key={value} value={value}>
                    {value}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button variant='outline' onClick={() => void load()}>
              Refresh
            </Button>
          </div>
        </div>

        <div className='overflow-hidden rounded-md border'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Order</TableHead>
                <TableHead>Customer</TableHead>
                <TableHead>Items</TableHead>
                <TableHead className='text-right'>Total</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Payment</TableHead>
                <TableHead className='text-right'>Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading &&
                Array.from({ length: 4 }).map((_, index) => (
                  <TableRow key={index}>
                    <TableCell colSpan={7}>
                      <Skeleton className='h-6 w-full' />
                    </TableCell>
                  </TableRow>
                ))}

              {!loading && orders.length === 0 && (
                <TableRow>
                  <TableCell
                    colSpan={7}
                    className='text-muted-foreground h-24 text-center'
                  >
                    No orders found.
                  </TableCell>
                </TableRow>
              )}

              {!loading &&
                orders.map((order) => (
                  <TableRow key={order.id}>
                    <TableCell>
                      <div className='font-mono text-xs'>
                        {order.id.slice(0, 8)}
                      </div>
                      <div className='text-muted-foreground text-xs'>
                        {formatDate(order.created_at)}
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className='font-medium'>{order.username}</div>
                      <div className='text-muted-foreground text-xs'>
                        {order.email}
                      </div>
                    </TableCell>
                    <TableCell className='text-sm'>
                      {order.order_item
                        .map((item) => `${item.name} ×${item.qty}`)
                        .join(', ')}
                    </TableCell>
                    <TableCell className='text-right tabular-nums'>
                      {rupiah.format(order.total)}
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant='outline'
                        className={orderStatusClass[order.status]}
                      >
                        {order.status}
                      </Badge>
                    </TableCell>
                    <TableCell className='text-muted-foreground text-sm'>
                      {paymentMethodLabel(order.payment)}
                    </TableCell>
                    <TableCell className='text-right'>
                      <Button size='sm' variant='outline' asChild>
                        <Link
                          to='/orders/$orderId'
                          params={{ orderId: order.id }}
                        >
                          View
                        </Link>
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
            </TableBody>
          </Table>
        </div>
      </Main>
    </>
  )
}
