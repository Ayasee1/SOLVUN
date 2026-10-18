import type { OrderStatus } from './schema'

export const orderStatusClass: Record<OrderStatus, string> = {
  pending: 'bg-amber-500/15 text-amber-700 dark:text-amber-400',
  paid: 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-400',
  expired: 'bg-muted text-muted-foreground',
  cancelled: 'bg-muted text-muted-foreground',
  needs_refund: 'bg-red-500/15 text-red-700 dark:text-red-400',
}

// Human-readable payment method labels (keeps the raw gateway type in the API).
export function paymentMethodLabel(payment?: {
  type?: string
  bank?: string
  store?: string
}): string {
  if (!payment?.type) return '-'
  switch (payment.type) {
    case 'bank_transfer':
      return payment.bank
        ? `Bank transfer · ${payment.bank.toUpperCase()}`
        : 'Bank transfer'
    case 'echannel':
      return 'Mandiri Virtual Account'
    case 'cstore':
      return payment.store
        ? payment.store.charAt(0).toUpperCase() + payment.store.slice(1)
        : 'Convenience store'
    case 'qris':
      return 'QRIS'
    case 'gopay':
      return 'GoPay'
    case 'shopeepay':
      return 'ShopeePay'
    case 'credit_card':
      return 'Credit card'
    default:
      return payment.type
  }
}
