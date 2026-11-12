export interface InvoiceOrder {
  status: string
  parent_id?: number | null
  paid_at?: string | null
  refunded_amount?: string | number
  total_amount: string | number
}

// Display eligibility only; the API rechecks ownership and child-order refunds.
export function invoiceIneligibleReason(order: InvoiceOrder): string | null {
  if (order.parent_id != null) return 'invoice.ineligibleChild'
  if (order.status === 'canceled') return 'invoice.ineligibleCanceled'
  if (order.status === 'expired') return 'invoice.ineligibleExpired'
  if (['refunded', 'partially_refunded'].includes(order.status) || Number(order.refunded_amount || 0) !== 0) return 'invoice.ineligibleRefunded'
  if (!order.paid_at || order.status === 'pending_payment') return 'invoice.ineligibleUnpaid'
  if (!(Number(order.total_amount) > 0)) return 'invoice.ineligibleAmount'
  if (!['paid', 'fulfilling', 'partially_delivered', 'delivered', 'completed'].includes(order.status)) return 'invoice.eligible'
  return null
}
