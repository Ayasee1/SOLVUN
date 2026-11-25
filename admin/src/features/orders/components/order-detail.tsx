import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { ArrowLeft, Download, PackageCheck } from 'lucide-react'
import { toast } from 'sonner'
import { queryWithToast } from '@/lib/query'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ConfigDrawer } from '@/components/config-drawer'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { cancelOrder, fetchOrder, refundOrder, retryShipment } from '../api'
import { orderStatusClass, paymentMethodLabel } from '../data/status'

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

export function OrderDetail({ orderId }: { orderId: string }) {
  const [cancelling, setCancelling] = useState(false)
  const [shipping, setShipping] = useState(false)
  const [refunding, setRefunding] = useState(false)
  const [confirmRefund, setConfirmRefund] = useState(false)
  const orderQuery = useQuery({
    queryKey: ['orders', orderId],
    queryFn: () => queryWithToast(() => fetchOrder(orderId)),
    retry: false,
  })

  const order = orderQuery.data ?? null
  const loading = orderQuery.isLoading

  async function onCancel() {
    if (!order) return
    setCancelling(true)
    try {
      await cancelOrder(order.id)
      toast.success('Order cancelled and stock released')
      void orderQuery.refetch()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Cancel failed')
    } finally {
      setCancelling(false)
    }
  }

  async function onCreateShipment() {
    if (!order) return
    setShipping(true)
    try {
      await retryShipment(order.id)
      toast.success('Shipment created')
      void orderQuery.refetch()
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : 'Could not create shipment'
      )
    } finally {
      setShipping(false)
    }
  }

  async function onRefund() {
    if (!order) return
    setRefunding(true)
    try {
      await refundOrder(order.id)
      toast.success('Refund sent to Midtrans')
      setConfirmRefund(false)
      void orderQuery.refetch()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Refund failed')
    } finally {
      setRefunding(false)
    }
  }

  return (
    <>
      <Header fixed>
        <Search className='me-auto' />
        <ThemeSwitch />
        <ConfigDrawer />
        <ProfileDropdown />
      </Header>

      <Main className='flex flex-1 flex-col gap-4 sm:gap-6'>
        <div className='flex flex-wrap items-start justify-between gap-3'>
          <div className='space-y-1'>
            <Button variant='ghost' size='sm' className='-ms-2' asChild>
              <Link to='/orders'>
                <ArrowLeft className='size-4' />
                Back to orders
              </Link>
            </Button>
            <h2 className='font-mono text-xl font-bold tracking-tight'>
              {orderId}
            </h2>
            {order && (
              <p className='text-sm text-muted-foreground'>
                placed {formatDate(order.created_at)}
              </p>
            )}
          </div>

          {order && (
            <div className='flex items-center gap-2'>
              <Badge
                variant='outline'
                className={orderStatusClass[order.status]}
              >
                {order.status}
              </Badge>
              <Button variant='outline' asChild>
                <a
                  href={`/api/admin/orders/${order.id}/invoice.pdf`}
                  target='_blank'
                  rel='noreferrer'
                >
                  <Download className='size-4' />
                  Invoice
                </a>
              </Button>
              {order.status === 'paid' && !order.shipment?.provider_id && (
                <Button
                  variant='outline'
                  disabled={shipping}
                  onClick={() => void onCreateShipment()}
                >
                  <PackageCheck className='size-4' />
                  {shipping ? 'Creating...' : 'Create shipment'}
                </Button>
              )}
              {order.status === 'needs_refund' && (
                <Button
                  variant='destructive'
                  disabled={refunding}
                  onClick={() => setConfirmRefund(true)}
                >
                  {refunding ? 'Refunding...' : 'Refund'}
                </Button>
              )}
              {order.status === 'pending' && (
                <Button
                  variant='outline'
                  disabled={cancelling}
                  onClick={() => void onCancel()}
                >
                  {cancelling ? 'Cancelling...' : 'Cancel order'}
                </Button>
              )}
            </div>
          )}
        </div>

        {loading && (
          <div className='grid gap-4 lg:grid-cols-3'>
            <Skeleton className='h-64 lg:col-span-2' />
            <Skeleton className='h-64' />
          </div>
        )}

        {!loading && order && (
          <div className='grid gap-4 lg:grid-cols-3'>
            <div className='space-y-4 lg:col-span-2'>
              <Card>
                <CardHeader>
                  <CardTitle>Items</CardTitle>
                </CardHeader>
                <CardContent className='space-y-4'>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Product</TableHead>
                        <TableHead className='text-right'>Price</TableHead>
                        <TableHead className='text-right'>Qty</TableHead>
                        <TableHead className='text-right'>Subtotal</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {order.order_item.map((item) => (
                        <TableRow key={item.id}>
                          <TableCell>{item.name}</TableCell>
                          <TableCell className='text-right tabular-nums'>
                            {rupiah.format(item.price)}
                          </TableCell>
                          <TableCell className='text-right tabular-nums'>
                            {item.qty}
                          </TableCell>
                          <TableCell className='text-right tabular-nums'>
                            {rupiah.format(item.total_price)}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                  <Separator />
                  <div className='space-y-1'>
                    <div className='flex items-center justify-between text-sm text-muted-foreground'>
                      <span>Subtotal</span>
                      <span className='tabular-nums'>
                        {rupiah.format(
                          order.total - (order.shipping_cost ?? 0)
                        )}
                      </span>
                    </div>
                    <div className='flex items-center justify-between text-sm text-muted-foreground'>
                      <span>
                        Shipping
                        {order.courier
                          ? ` (${order.courier} ${order.courier_service ?? ''})`
                          : ''}
                      </span>
                      <span className='tabular-nums'>
                        {rupiah.format(order.shipping_cost ?? 0)}
                      </span>
                    </div>
                    <div className='flex items-center justify-between'>
                      <span className='font-medium'>Total</span>
                      <span className='text-lg font-bold tabular-nums'>
                        {rupiah.format(order.total)}
                      </span>
                    </div>
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>Payment</CardTitle>
                </CardHeader>
                <CardContent className='space-y-2 text-sm'>
                  {order.payment ? (
                    <dl className='grid grid-cols-[140px_1fr] gap-y-2'>
                      <dt className='text-muted-foreground'>Method</dt>
                      <dd>{paymentMethodLabel(order.payment)}</dd>
                      {order.payment.bank && (
                        <>
                          <dt className='text-muted-foreground'>Bank</dt>
                          <dd>{order.payment.bank}</dd>
                        </>
                      )}
                      {order.payment.va_number && (
                        <>
                          <dt className='text-muted-foreground'>VA number</dt>
                          <dd className='font-mono'>
                            {order.payment.va_number}
                          </dd>
                        </>
                      )}
                      {order.payment.biller_code && (
                        <>
                          <dt className='text-muted-foreground'>Biller code</dt>
                          <dd className='font-mono'>
                            {order.payment.biller_code}
                          </dd>
                        </>
                      )}
                      {order.payment.bill_key && (
                        <>
                          <dt className='text-muted-foreground'>VA number</dt>
                          <dd className='font-mono'>
                            {order.payment.bill_key}
                          </dd>
                        </>
                      )}
                      {order.payment.payment_code && (
                        <>
                          <dt className='text-muted-foreground'>
                            Payment code
                          </dt>
                          <dd className='font-mono'>
                            {order.payment.payment_code}
                          </dd>
                        </>
                      )}
                      {order.payment.masked_card && (
                        <>
                          <dt className='text-muted-foreground'>Card</dt>
                          <dd className='font-mono'>
                            {order.payment.masked_card}
                          </dd>
                        </>
                      )}
                      {order.payment.store && (
                        <>
                          <dt className='text-muted-foreground'>Store</dt>
                          <dd>{order.payment.store}</dd>
                        </>
                      )}
                      {order.payment.transaction_id && (
                        <>
                          <dt className='text-muted-foreground'>
                            Transaction ID
                          </dt>
                          <dd className='font-mono text-xs'>
                            {order.payment.transaction_id}
                          </dd>
                        </>
                      )}
                      {order.payment.settled_at && (
                        <>
                          <dt className='text-muted-foreground'>Settled at</dt>
                          <dd>{order.payment.settled_at}</dd>
                        </>
                      )}
                      {order.refund && (
                        <>
                          <dt className='text-muted-foreground'>Refunded</dt>
                          <dd>
                            {rupiah.format(order.refund.amount)} ·{' '}
                            {formatDate(order.refund.refunded_at)}
                          </dd>
                        </>
                      )}
                    </dl>
                  ) : (
                    <p className='text-muted-foreground'>
                      Not paid yet.
                      {order.payment_url && (
                        <>
                          {' '}
                          <a
                            href={order.payment_url}
                            target='_blank'
                            rel='noreferrer'
                            className='underline underline-offset-4'
                          >
                            Open payment page
                          </a>
                        </>
                      )}
                    </p>
                  )}
                </CardContent>
              </Card>
            </div>

            <div className='space-y-4'>
              <Card>
                <CardHeader>
                  <CardTitle>Customer</CardTitle>
                </CardHeader>
                <CardContent className='space-y-1 text-sm'>
                  <p className='font-medium'>{order.username}</p>
                  <p className='text-muted-foreground'>{order.email}</p>
                  <p className='text-muted-foreground'>{order.phone}</p>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>Shipment</CardTitle>
                </CardHeader>
                <CardContent className='space-y-1 text-sm'>
                  {order.shipment ? (
                    <>
                      <div className='flex items-center justify-between'>
                        <span className='text-muted-foreground'>Waybill</span>
                        <span className='font-mono'>
                          {order.shipment.waybill_id || '-'}
                        </span>
                      </div>
                      <div className='flex items-center justify-between'>
                        <span className='text-muted-foreground'>Status</span>
                        <Badge variant='outline'>
                          {order.shipment.status || '-'}
                        </Badge>
                      </div>
                      {order.shipment.provider_id && (
                        <div className='pt-1 font-mono text-xs text-muted-foreground'>
                          {order.shipment.provider_id}
                        </div>
                      )}
                      {order.shipment.history &&
                        order.shipment.history.length > 0 && (
                          <ol className='mt-3 space-y-2 border-s ps-3'>
                            {order.shipment.history.map((event, index) => (
                              <li key={index} className='text-xs'>
                                <div className='font-medium'>
                                  {event.status}
                                </div>
                                {event.note && (
                                  <div className='text-muted-foreground'>
                                    {event.note}
                                  </div>
                                )}
                                {event.updated_at && (
                                  <div className='text-muted-foreground'>
                                    {event.updated_at}
                                  </div>
                                )}
                              </li>
                            ))}
                          </ol>
                        )}
                    </>
                  ) : (
                    <p className='text-muted-foreground'>
                      Not created yet (it is created once the order is paid).
                    </p>
                  )}
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>Shipping address</CardTitle>
                </CardHeader>
                <CardContent className='space-y-1 text-sm'>
                  <p>{order.address.street}</p>
                  <p className='text-muted-foreground'>
                    {order.address.city}, {order.address.state}{' '}
                    {order.address.zip_code}
                  </p>
                  <a
                    href={`https://www.google.com/maps?q=${order.address.latitude},${order.address.longitude}`}
                    target='_blank'
                    rel='noreferrer'
                    className='text-xs text-muted-foreground underline underline-offset-4'
                  >
                    {order.address.latitude}, {order.address.longitude}
                  </a>
                </CardContent>
              </Card>
            </div>
          </div>
        )}
      </Main>

      <ConfirmDialog
        open={confirmRefund}
        onOpenChange={setConfirmRefund}
        title='Refund this order?'
        desc={`Refund ${rupiah.format(
          order?.total ?? 0
        )} to the customer through Midtrans. This cannot be undone.`}
        confirmText='Refund'
        destructive
        isLoading={refunding}
        handleConfirm={() => void onRefund()}
        className='sm:max-w-sm'
      />
    </>
  )
}
