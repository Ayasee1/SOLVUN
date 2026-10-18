import { createFileRoute } from '@tanstack/react-router'
import { OrderDetail } from '@/features/orders/components/order-detail'

export const Route = createFileRoute('/_authenticated/orders/$orderId')({
  component: OrderDetailRoute,
})

function OrderDetailRoute() {
  const { orderId } = Route.useParams()
  return <OrderDetail orderId={orderId} />
}
