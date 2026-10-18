import { Badge } from '@/components/ui/badge'
import type { Order } from '@/features/orders/data/schema'
import { orderStatusClass } from '@/features/orders/data/status'

const rupiah = new Intl.NumberFormat('id-ID', {
  style: 'currency',
  currency: 'IDR',
  maximumFractionDigits: 0,
})

export function RecentOrders({ orders }: { orders: Order[] }) {
  if (orders.length === 0) {
    return <p className='text-muted-foreground text-sm'>No orders yet.</p>
  }

  return (
    <div className='space-y-6'>
      {orders.map((order) => (
        <div key={order.id} className='flex items-center justify-between gap-4'>
          <div className='space-y-1'>
            <p className='text-sm leading-none font-medium'>{order.username}</p>
            <p className='text-muted-foreground text-xs'>{order.email}</p>
            <p className='text-muted-foreground font-mono text-xs'>
              {order.id.slice(0, 8)}
            </p>
          </div>
          <div className='space-y-1 text-right'>
            <div className='font-medium tabular-nums'>
              {rupiah.format(order.total)}
            </div>
            <Badge
              variant='outline'
              className={orderStatusClass[order.status]}
            >
              {order.status}
            </Badge>
          </div>
        </div>
      ))}
    </div>
  )
}
