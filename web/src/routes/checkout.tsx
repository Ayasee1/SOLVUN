import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { createFileRoute, Link } from '@tanstack/react-router'
import {
  ArrowLeft,
  ChevronDown,
  Crosshair,
  Loader2,
  Lock,
  MapPin,
  ShieldCheck,
  ShoppingBag,
  Truck,
} from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Skeleton } from '@/components/ui/skeleton'
import { OrderSummary } from '@/components/checkout/order-summary'
import {
  MapPicker,
  forwardGeocode,
  reverseGeocode,
  type LatLon,
} from '@/components/map-picker'
import { useCart } from '@/features/cart/cart-store'
import { api, ApiError, fetchShippingRates, type ShippingRate } from '@/lib/api'
import { formatRupiah } from '@/lib/format'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/checkout')({ component: Checkout })

type Form = {
  contactName: string
  email: string
  contactPhone: string
  sameAsContact: boolean
  recipientName: string
  recipientPhone: string
  street: string
  line2: string
  city: string
  state: string
  zip: string
}

type Errors = Partial<Record<keyof Form | 'shipping', string>>

type StoreConfig = { midtrans_client_key: string; midtrans_env: string }
type CreatedOrder = {
  id: string
  total: number
  snap_token?: string
  payment_url?: string
  access_token?: string
}

// orderPagePath builds the guest link. The token is what authorises the order
// page; the id alone is not enough.
function orderPagePath(order: CreatedOrder) {
  return order.access_token
    ? `/order/${order.id}?t=${encodeURIComponent(order.access_token)}`
    : `/order/${order.id}`
}

const DRAFT_KEY = 'golearn.checkout'

const emptyForm: Form = {
  contactName: '',
  email: '',
  contactPhone: '',
  sameAsContact: true,
  recipientName: '',
  recipientPhone: '',
  street: '',
  line2: '',
  city: '',
  state: '',
  zip: '',
}

function Checkout() {
  const { items, subtotal, clear } = useCart()

  // The checkout draft survives a refresh or an accidental tab reload.
  const draft = useMemo(() => readDraft(), [])
  const [form, setForm] = useState<Form>(() => ({ ...emptyForm, ...draft.form }))
  const [errors, setErrors] = useState<Errors>({})
  const [pin, setPin] = useState<LatLon | undefined>(draft.pin)
  const [showMap, setShowMap] = useState(Boolean(draft.pin))
  const [rates, setRates] = useState<ShippingRate[]>([])
  const [weightGram, setWeightGram] = useState(0)
  const [loadingRates, setLoadingRates] = useState(false)
  const [selected, setSelected] = useState('')
  const [locating, setLocating] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [payment, setPayment] = useState<CreatedOrder | null>(null)
  const [autoFilled, setAutoFilled] = useState<Record<string, string>>({})
  const formRef = useRef<HTMLFormElement>(null)

  const selectedRate = useMemo(
    () => rates.find((rate) => key(rate) === selected),
    [rates, selected]
  )
  const shipping = selectedRate?.price ?? 0
  const total = subtotal + shipping
  const itemCount = items.reduce((sum, item) => sum + item.qty, 0)

  useEffect(() => {
    writeDraft({ form, pin })
  }, [form, pin])

  function set<K extends keyof Form>(field: K, value: Form[K]) {
    setForm((current) => ({ ...current, [field]: value }))
    setErrors((current) => ({ ...current, [field]: undefined }))
  }

  // quote shipping as soon as we know the destination postal code
  useEffect(() => {
    const zip = form.zip.trim()
    if (!/^\d{5}$/.test(zip) || items.length === 0) {
      setRates([])
      setSelected('')
      setWeightGram(0)
      return
    }

    let cancelled = false
    setLoadingRates(true)
    setRates([])
    setSelected('')

    fetchShippingRates({
      destination_postal_code: zip,
      destination_lat: pin?.lat ?? 0,
      destination_lon: pin?.lon ?? 0,
      items: items.map((item) => ({ product_id: item.product_id, qty: item.qty })),
    })
      .then((res) => {
        if (cancelled) return
        const sorted = [...res.rates].sort((a, b) => a.price - b.price)
        setRates(sorted)
        setWeightGram(res.weight_gram)
        setSelected((current) =>
          sorted.some((rate) => key(rate) === current)
            ? current
            : sorted[0]
              ? key(sorted[0])
              : ''
        )
      })
      .catch((error: unknown) => {
        if (cancelled) return
        setRates([])
        setWeightGram(0)
        toast.error(
          error instanceof Error ? error.message : 'Failed to load shipping rates'
        )
      })
      .finally(() => {
        if (!cancelled) setLoadingRates(false)
      })

    return () => {
      cancelled = true
    }
  }, [form.zip, pin?.lat, pin?.lon, items])

  function applyPlace(place: {
    street?: string
    city?: string
    state?: string
    postalCode?: string
  }) {
    const filled: Record<string, string> = {}
    const next = { ...form }

    const setIfAuto = (field: keyof Form, value?: string) => {
      const candidate = (value ?? '').trim()
      if (!candidate) return
      const current = String(form[field])
      const previousAuto = autoFilled[field]
      if (!current || current === previousAuto) {
        next[field] = candidate as never
        filled[field] = candidate
      }
    }

    setIfAuto('street', place.street)
    setIfAuto('city', place.city)
    setIfAuto('state', place.state)
    setIfAuto('zip', (place.postalCode ?? '').replace(/\D/g, '').slice(0, 5))

    setForm(next)
    setAutoFilled((previous) => ({ ...previous, ...filled }))
  }

  async function locateTypedAddress() {
    const query = [form.street, form.line2, form.city, form.state, form.zip]
      .map((part) => part.trim())
      .filter(Boolean)
      .join(', ')
    if (!query) {
      toast.error('Type the address first, then locate it on the map')
      return
    }

    setShowMap(true)
    setLocating(true)
    const position = await forwardGeocode(`${query}, Indonesia`)
    setLocating(false)

    if (!position) {
      toast.error('Could not find that address on the map')
      return
    }
    setPin(position)
  }

  function useMyLocation() {
    setShowMap(true)
    navigator.geolocation?.getCurrentPosition(
      (position) =>
        setPin({ lat: position.coords.latitude, lon: position.coords.longitude }),
      () => toast.error('Could not read your location')
    )
  }

  async function fillAddressFromPin() {
    if (!pin) return
    const place = await reverseGeocode(pin)
    if (!place) {
      toast.error('Could not resolve an address for that pin')
      return
    }
    applyPlace(place)
    toast.success('Address filled from the pin')
  }

  function validate(): Errors {
    const next: Errors = {}

    if (!form.contactName.trim()) next.contactName = 'Enter your name'
    if (!/^\S+@\S+\.\S+$/.test(form.email.trim())) next.email = 'Enter a valid email'
    if (!form.contactPhone.trim()) next.contactPhone = 'Enter a phone number'
    if (!form.sameAsContact && !form.recipientName.trim()) {
      next.recipientName = 'Enter the recipient name'
    }
    if (!form.sameAsContact && !form.recipientPhone.trim()) {
      next.recipientPhone = 'Enter the recipient phone'
    }
    if (!form.street.trim()) next.street = 'Enter the street address'
    if (!form.city.trim()) next.city = 'Enter the city'
    if (!form.state.trim()) next.state = 'Enter the province'
    if (!/^\d{5}$/.test(form.zip.trim())) next.zip = 'Enter a 5-digit postal code'
    if (!selectedRate) next.shipping = 'Choose a shipping option'

    return next
  }

  async function pay() {
    const found = validate()
    setErrors(found)

    const firstError = Object.keys(found)[0]
    if (firstError) {
      toast.error(found[firstError as keyof Errors] ?? 'Please complete the form')
      const container = formRef.current?.querySelector(
        `[data-field="${firstError}"]`
      )
      container?.scrollIntoView({ behavior: 'smooth', block: 'center' })
      const focusable = container?.querySelector(
        'input, [role="radio"], button'
      ) as HTMLElement | undefined
      focusable?.focus()
      return
    }

    setSubmitting(true)
    try {
      const order = await api.post<CreatedOrder>('/orders', {
        username: form.contactName.trim(),
        email: form.email.trim(),
        phone: form.contactPhone.trim(),
        address: {
          recipient_name: form.sameAsContact ? '' : form.recipientName.trim(),
          recipient_phone: form.sameAsContact ? '' : form.recipientPhone.trim(),
          street: form.street.trim(),
          line2: form.line2.trim(),
          city: form.city.trim(),
          state: form.state.trim(),
          zip_code: form.zip.trim(),
          latitude: pin?.lat ?? 0,
          longitude: pin?.lon ?? 0,
        },
        courier: selectedRate!.courier,
        courier_service: selectedRate!.service,
        order_item: items.map((item) => ({
          id: item.product_id,
          qty: item.qty,
        })),
      })

      clear()
      clearDraft()
      setPayment(order)
      await startPayment(order)
    } catch (error) {
      toast.error(
        error instanceof ApiError
          ? error.message
          : 'Could not create the order, please try again'
      )
      setSubmitting(false)
    }
  }

  if (payment) {
    return <PaymentPending order={payment} />
  }

  if (items.length === 0) {
    if (submitting) {
      return (
        <main className='mx-auto max-w-3xl px-4 py-24 text-center'>
          <Loader2 className='text-muted-foreground mx-auto size-6 animate-spin' />
          <h1 className='mt-6 text-2xl font-semibold'>
            Taking you to secure payment
          </h1>
          <p className='text-muted-foreground mt-2 text-sm'>
            Please keep this page open while we open the payment window.
          </p>
        </main>
      )
    }

    return (
      <main className='mx-auto max-w-3xl px-4 py-24 text-center'>
        <div className='bg-muted mx-auto grid size-14 place-items-center rounded-2xl'>
          <ShoppingBag className='text-muted-foreground size-6' />
        </div>
        <h1 className='mt-6 text-2xl font-semibold'>Nothing to check out</h1>
        <p className='text-muted-foreground mt-2 text-sm'>
          Your cart is empty. Add a product before starting checkout.
        </p>
        <Button className='mt-6' asChild>
          <Link to='/'>Browse catalog</Link>
        </Button>
      </main>
    )
  }

  const summaryItems = items.map((item) => ({
    key: item.product_id,
    name: item.name,
    price: item.price,
    qty: item.qty,
    image: item.image,
  }))

  return (
    <main className='mx-auto max-w-6xl px-4 pt-6 pb-32 sm:pt-8 lg:pb-16'>
      <div className='flex flex-wrap items-start justify-between gap-6'>
        <div>
          <Link
            to='/cart'
            className='text-muted-foreground hover:text-foreground inline-flex items-center gap-1.5 text-sm transition-colors'
          >
            <ArrowLeft className='size-4' />
            Back to cart
          </Link>
          <h1 className='mt-4 text-3xl font-semibold tracking-tight sm:text-4xl'>
            Checkout
          </h1>
          <p className='text-muted-foreground mt-1.5 text-sm'>
            Complete your details and choose a delivery method.
          </p>
        </div>
        <CheckoutSteps />
      </div>

      <details className='group mt-6 rounded-2xl border bg-card lg:hidden'>
        <summary className='flex cursor-pointer list-none items-center gap-3 p-4'>
          <ShoppingBag className='text-muted-foreground size-4' />
          <span className='text-sm font-medium'>Order summary</span>
          <span className='bg-muted text-muted-foreground rounded-full px-2 py-0.5 text-xs tabular-nums'>
            {itemCount}
          </span>
          <span className='ms-auto flex items-center gap-2 text-sm font-semibold tabular-nums'>
            {formatRupiah(total)}
            <ChevronDown className='size-4 transition-transform group-open:rotate-180' />
          </span>
        </summary>
        <div className='border-t px-4 pt-2 pb-4'>
          <OrderSummary
            showHeader={false}
            className='border-0 shadow-none'
            items={summaryItems}
            subtotal={subtotal}
            shipping={selectedRate ? shipping : null}
            shippingNote='Enter a postal code'
            total={total}
          />
        </div>
      </details>

      <form
        ref={formRef}
        noValidate
        className='mt-6 grid items-start gap-6 lg:mt-8 lg:grid-cols-[minmax(0,1fr)_400px] lg:gap-8'
        onSubmit={(event) => {
          event.preventDefault()
          void pay()
        }}
      >
        <div className='space-y-5'>
          <Section
            step={1}
            title='Contact details'
            description='We send the receipt and delivery updates here.'
          >
            <FieldGroup>
              <TextField
                name='email'
                label='Email address'
                type='email'
                autoComplete='email'
                placeholder='you@example.com'
                hint='Your invoice and tracking link will be sent to this address.'
                value={form.email}
                error={errors.email}
                onChange={(value) => set('email', value)}
              />
              <div className='grid gap-5 sm:grid-cols-2'>
                <TextField
                  name='contactName'
                  label='Full name'
                  autoComplete='name'
                  placeholder='Your name'
                  value={form.contactName}
                  error={errors.contactName}
                  onChange={(value) => set('contactName', value)}
                />
                <TextField
                  name='contactPhone'
                  label='Phone number'
                  type='tel'
                  autoComplete='tel'
                  placeholder='08xx xxxx xxxx'
                  hint='The courier uses this number for delivery updates.'
                  value={form.contactPhone}
                  error={errors.contactPhone}
                  onChange={(value) => set('contactPhone', value)}
                />
              </div>
            </FieldGroup>
          </Section>

          <Section
            step={2}
            title='Delivery address'
            description='Where the parcel should be delivered.'
          >
            <FieldGroup>
              <div className='rounded-xl border bg-muted/30 p-4'>
                <label className='flex cursor-pointer items-start gap-3'>
                  <input
                    type='checkbox'
                    className='accent-primary mt-0.5 size-4 shrink-0'
                    checked={form.sameAsContact}
                    onChange={(event) => {
                      set('sameAsContact', event.target.checked)
                      setErrors({})
                    }}
                  />
                  <span>
                    <span className='block text-sm font-medium'>
                      The recipient is the same as me
                    </span>
                    <span className='text-muted-foreground block text-xs'>
                      Uncheck to send this order to someone else.
                    </span>
                  </span>
                </label>
              </div>

              {!form.sameAsContact && (
                <div className='grid gap-5 sm:grid-cols-2'>
                  <TextField
                    name='recipientName'
                    label='Recipient name'
                    autoComplete='shipping name'
                    placeholder='Who receives the parcel'
                    value={form.recipientName}
                    error={errors.recipientName}
                    onChange={(value) => set('recipientName', value)}
                  />
                  <TextField
                    name='recipientPhone'
                    label='Recipient phone'
                    autoComplete='shipping tel'
                    type='tel'
                    placeholder='08xx xxxx xxxx'
                    value={form.recipientPhone}
                    error={errors.recipientPhone}
                    onChange={(value) => set('recipientPhone', value)}
                  />
                </div>
              )}

              <TextField
                name='street'
                label='Street address'
                autoComplete='street-address'
                placeholder='Street, building, house number'
                value={form.street}
                error={errors.street}
                onChange={(value) => set('street', value)}
              />

              <TextField
                name='line2'
                label='Apartment, unit, floor'
                optional
                autoComplete='address-line2'
                placeholder='e.g. Unit 12B'
                value={form.line2}
                onChange={(value) => set('line2', value)}
              />

              <div className='grid gap-5 sm:grid-cols-2'>
                <TextField
                  name='city'
                  label='City'
                  autoComplete='address-level2'
                  placeholder='City or regency'
                  value={form.city}
                  error={errors.city}
                  onChange={(value) => set('city', value)}
                />
                <TextField
                  name='state'
                  label='Province'
                  autoComplete='address-level1'
                  placeholder='Province'
                  value={form.state}
                  error={errors.state}
                  onChange={(value) => set('state', value)}
                />
              </div>

              <div className='grid gap-5 sm:grid-cols-[minmax(0,200px)_1fr] sm:items-start'>
                <TextField
                  name='zip'
                  label='Postal code'
                  inputMode='numeric'
                  autoComplete='postal-code'
                  placeholder='40123'
                  hint='Used to calculate courier rates.'
                  value={form.zip}
                  error={errors.zip}
                  onChange={(value) =>
                    set('zip', value.replace(/\D/g, '').slice(0, 5))
                  }
                />
                <div className='sm:pt-7'>
                  <p className='text-muted-foreground text-xs leading-relaxed'>
                    Rates appear automatically once a valid 5-digit postal code
                    is entered.
                  </p>
                </div>
              </div>

              <div className='rounded-xl border bg-muted/20 p-4'>
                <div className='flex flex-wrap items-start justify-between gap-3'>
                  <div className='flex items-start gap-2.5'>
                    <MapPin className='text-muted-foreground mt-0.5 size-4' />
                    <div>
                      <p className='text-sm font-medium'>
                        Delivery pin{' '}
                        <span className='text-muted-foreground font-normal'>
                          (optional)
                        </span>
                      </p>
                      <p className='text-muted-foreground mt-0.5 text-xs'>
                        Drop a pin so the courier can find the exact spot.
                      </p>
                    </div>
                  </div>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={() => setShowMap((value) => !value)}
                  >
                    {showMap ? 'Hide map' : 'Add pin'}
                  </Button>
                </div>

                {showMap && (
                  <div className='mt-4 space-y-3'>
                    <MapPicker value={pin} onChange={setPin} />
                    <div className='flex flex-wrap gap-2'>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={useMyLocation}
                      >
                        <Crosshair className='size-4' />
                        Use my location
                      </Button>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        disabled={locating}
                        onClick={() => void locateTypedAddress()}
                      >
                        {locating ? (
                          <Loader2 className='size-4 animate-spin' />
                        ) : (
                          <MapPin className='size-4' />
                        )}
                        Find typed address
                      </Button>
                      {pin && (
                        <Button
                          type='button'
                          variant='ghost'
                          size='sm'
                          onClick={() => void fillAddressFromPin()}
                        >
                          Fill address from pin
                        </Button>
                      )}
                      {pin && (
                        <Button
                          type='button'
                          variant='ghost'
                          size='sm'
                          onClick={() => setPin(undefined)}
                        >
                          Clear pin
                        </Button>
                      )}
                    </div>
                  </div>
                )}
              </div>
            </FieldGroup>
          </Section>

          <Section
            step={3}
            title='Shipping method'
            description={
              selectedRate
                ? `${selectedRate.courier_name} ${selectedRate.service_name} selected`
                : 'Choose how you would like the parcel delivered.'
            }
          >
            <div data-field='shipping' className='grid'>
              <div className='mb-4 flex flex-wrap items-center justify-between gap-2'>
                <p className='text-muted-foreground text-xs'>
                  {/^\d{5}$/.test(form.zip)
                    ? 'Live rates for your postal code.'
                    : 'Enter a postal code above to see courier options.'}
                </p>
                {weightGram > 0 && (
                  <span className='bg-muted text-muted-foreground rounded-full px-2.5 py-1 text-xs tabular-nums'>
                    Package ~{weightGram} g
                  </span>
                )}
              </div>

              {loadingRates && (
                <div className='space-y-2.5'>
                  <Skeleton className='h-[74px] w-full rounded-xl' />
                  <Skeleton className='h-[74px] w-full rounded-xl' />
                </div>
              )}

              {!loadingRates && rates.length === 0 && (
                <div className='rounded-xl border border-dashed px-4 py-8 text-center'>
                  <Truck className='text-muted-foreground mx-auto size-5' />
                  <p className='text-muted-foreground mt-2 text-sm'>
                    {/^\d{5}$/.test(form.zip)
                      ? 'No shipping options for this postal code.'
                      : 'Courier options will appear here.'}
                  </p>
                </div>
              )}

              {!loadingRates && rates.length > 0 && (
                <RadioGroup
                  value={selected}
                  onValueChange={setSelected}
                  className='gap-2.5'
                >
                  {rates.map((rate) => {
                    const active = key(rate) === selected
                    return (
                      <label
                        key={key(rate)}
                        htmlFor={key(rate)}
                        className={cn(
                          'flex cursor-pointer items-center gap-3.5 rounded-xl border p-4 transition-colors',
                          active
                            ? 'border-foreground bg-muted/40'
                            : 'hover:border-foreground/30 hover:bg-muted/20'
                        )}
                      >
                        <RadioGroupItem value={key(rate)} id={key(rate)} />
                        <div className='min-w-0 flex-1'>
                          <p className='text-sm font-medium'>
                            {rate.courier_name}{' '}
                            <span className='text-muted-foreground font-normal'>
                              · {rate.service_name}
                            </span>
                          </p>
                          <p className='text-muted-foreground mt-0.5 text-xs'>
                            {rate.duration
                              ? `Estimated ${rate.duration}`
                              : (rate.description ?? 'Courier service')}
                          </p>
                        </div>
                        <span className='text-sm font-semibold tabular-nums'>
                          {formatRupiah(rate.price)}
                        </span>
                      </label>
                    )
                  })}
                </RadioGroup>
              )}

              <FieldError className='mt-3'>{errors.shipping}</FieldError>
            </div>
          </Section>

          <div className='text-muted-foreground flex flex-wrap items-center gap-x-6 gap-y-2 rounded-2xl border border-dashed px-5 py-4 text-xs'>
            <span className='flex items-center gap-1.5'>
              <Lock className='size-3.5' />
              SSL encrypted checkout
            </span>
            <span className='flex items-center gap-1.5'>
              <ShieldCheck className='size-3.5' />
              Payments processed by Midtrans
            </span>
            <span className='flex items-center gap-1.5'>
              <Truck className='size-3.5' />
              Tracked courier delivery
            </span>
          </div>
        </div>

        <aside className='hidden lg:block'>
          <div className='sticky top-24 space-y-4'>
            <OrderSummary
              items={summaryItems}
              subtotal={subtotal}
              shipping={selectedRate ? shipping : null}
              shippingNote='Enter a postal code'
              total={total}
              footer={
                <Button
                  type='button'
                  className='h-11 w-full'
                  disabled={submitting}
                  onClick={() => void pay()}
                >
                  {submitting ? (
                    <>
                      <Loader2 className='size-4 animate-spin' />
                      Starting payment...
                    </>
                  ) : (
                    <>
                      <Lock className='size-4' />
                      Pay {formatRupiah(total)}
                    </>
                  )}
                </Button>
              }
            />
            <p className='text-muted-foreground flex items-center justify-center gap-1.5 text-xs'>
              <Lock className='size-3' />
              You will be redirected to Midtrans to pay securely.
            </p>
          </div>
        </aside>
      </form>

      <div className='bg-background/95 fixed inset-x-0 bottom-0 z-30 border-t backdrop-blur lg:hidden'>
        <div className='mx-auto flex max-w-6xl items-center gap-4 px-4 py-3'>
          <div className='min-w-0 flex-1'>
            <p className='text-muted-foreground text-xs'>Total</p>
            <p className='font-display text-lg font-semibold tracking-tight tabular-nums'>
              {formatRupiah(total)}
            </p>
          </div>
          <Button
            type='button'
            className='h-11 min-w-36'
            disabled={submitting}
            onClick={() => void pay()}
          >
            {submitting ? (
              <>
                <Loader2 className='size-4 animate-spin' />
                Processing...
              </>
            ) : (
              <>
                <Lock className='size-4' />
                Pay now
              </>
            )}
          </Button>
        </div>
      </div>
    </main>
  )
}

function CheckoutSteps() {
  return (
    <ol className='hidden items-center gap-3 pt-1 text-xs sm:flex'>
      <li className='flex items-center gap-2 font-medium'>
        <span className='bg-primary text-primary-foreground grid size-6 place-items-center rounded-full text-[11px] font-semibold'>
          1
        </span>
        Details
      </li>
      <li aria-hidden className='bg-border h-px w-8' />
      <li className='text-muted-foreground flex items-center gap-2'>
        <span className='grid size-6 place-items-center rounded-full border text-[11px] font-semibold'>
          2
        </span>
        Payment
      </li>
    </ol>
  )
}

function Section({
  step,
  title,
  description,
  children,
}: {
  step: number
  title: string
  description: string
  children: ReactNode
}) {
  return (
    <section className='rounded-2xl border bg-card p-5 shadow-xs sm:p-6'>
      <div className='flex items-start gap-3'>
        <span className='bg-muted text-foreground grid size-7 shrink-0 place-items-center rounded-full text-xs font-semibold tabular-nums'>
          {step}
        </span>
        <div className='min-w-0'>
          <h2 className='font-display text-base font-semibold'>{title}</h2>
          <p className='text-muted-foreground mt-0.5 text-sm'>{description}</p>
        </div>
      </div>
      <div className='mt-5'>{children}</div>
    </section>
  )
}

function TextField({
  name,
  label,
  value,
  onChange,
  error,
  hint,
  optional,
  type = 'text',
  placeholder,
  inputMode,
  autoComplete,
  className,
}: {
  name: string
  label: string
  value: string
  onChange: (value: string) => void
  error?: string
  hint?: string
  optional?: boolean
  type?: string
  placeholder?: string
  inputMode?: 'numeric' | 'text' | 'tel' | 'email'
  autoComplete?: string
  className?: string
}) {
  return (
    <Field
      data-field={name}
      data-invalid={error ? true : undefined}
      className={className}
    >
      <FieldLabel htmlFor={name}>
        {label}
        {optional ? (
          <span className='text-muted-foreground font-normal'>(optional)</span>
        ) : (
          <span aria-hidden className='text-destructive'>
            *
          </span>
        )}
      </FieldLabel>
      <Input
        id={name}
        name={name}
        type={type}
        inputMode={inputMode}
        placeholder={placeholder}
        autoComplete={autoComplete}
        value={value}
        aria-invalid={error ? true : undefined}
        aria-required={!optional}
        onChange={(event) => onChange(event.target.value)}
      />
      {hint && !error ? <FieldDescription>{hint}</FieldDescription> : null}
      <FieldError>{error}</FieldError>
    </Field>
  )
}

const key = (rate: ShippingRate) => `${rate.courier}|${rate.service}`

// Opens the Snap modal when the public client key is configured. If it is not
// available we still avoid a same-tab redirect: the hosted payment page opens
// in a popup window instead, while the shopper stays on the status screen.
async function startPayment(order: CreatedOrder) {
  const orderPath = orderPagePath(order)
  const token = order.snap_token

  if (token) {
    let config: StoreConfig | undefined
    try {
      config = await api.get<StoreConfig>('/store/config')
    } catch {
      config = undefined
    }

    const clientKey = config?.midtrans_client_key?.trim()
    if (
      clientKey &&
      (await loadSnapScript(config?.midtrans_env === 'production', clientKey))
    ) {
      const snap = window.snap
      if (snap) {
        snap.pay(token, {
          onSuccess: () => window.location.assign(orderPath),
          onPending: () => window.location.assign(orderPath),
          onError: () => toast.error('Payment failed. Please try again.'),
          onClose: () => window.location.assign(orderPath),
        })
        return
      }
    }
  }

  if (order.payment_url) {
    openPaymentPopup(order.payment_url)
    return
  }

  toast.error('Payment is unavailable right now. Please view your order.')
}

function openPaymentPopup(url: string) {
  const width = 520
  const height = 760
  const left = window.screenX + Math.max(0, (window.outerWidth - width) / 2)
  const top = window.screenY + Math.max(0, (window.outerHeight - height) / 2)

  const popup = window.open(
    url,
    'golearn-payment',
    `popup=yes,width=${width},height=${height},left=${left},top=${top}`
  )

  if (!popup) {
    toast.error('The payment window was blocked by your browser.', {
      action: {
        label: 'Open payment page',
        onClick: () => window.location.assign(url),
      },
    })
    return
  }

  popup.focus()
}

function PaymentPending({ order }: { order: CreatedOrder }) {
  const [status, setStatus] = useState('pending')
  const orderPath = orderPagePath(order)
  const tokenQuery = order.access_token
    ? `?t=${encodeURIComponent(order.access_token)}`
    : ''

  useEffect(() => {
    let cancelled = false

    const check = async () => {
      try {
        const data = await api.get<{ status: string }>(
          `/orders/${order.id}${tokenQuery}`
        )
        if (cancelled) return
        setStatus(data.status)
        if (data.status !== 'pending') {
          window.location.assign(orderPath)
        }
      } catch {
        // transient network errors are ignored; the next poll retries
      }
    }

    void check()
    const timer = window.setInterval(check, 4000)
    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [order.id, orderPath, tokenQuery])

  return (
    <main className='mx-auto max-w-xl px-4 py-20'>
      <div className='bg-muted mx-auto grid size-14 place-items-center rounded-2xl'>
        <Loader2 className='text-muted-foreground size-6 animate-spin' />
      </div>
      <h1 className='mt-6 text-center text-2xl font-semibold'>
        Complete your payment
      </h1>
      <p className='text-muted-foreground mx-auto mt-2 max-w-md text-center text-sm'>
        Finish the payment in the secure payment window. If it did not open,
        use the button below — this page updates automatically.
      </p>

      <div className='mt-8 rounded-2xl border bg-card p-5 shadow-xs'>
        <dl className='space-y-3 text-sm'>
          <div className='flex items-center justify-between'>
            <dt className='text-muted-foreground'>Order</dt>
            <dd className='font-mono'>#{order.id.slice(0, 8)}</dd>
          </div>
          <div className='flex items-center justify-between'>
            <dt className='text-muted-foreground'>Total</dt>
            <dd className='font-semibold tabular-nums'>
              {formatRupiah(order.total)}
            </dd>
          </div>
          <div className='flex items-center justify-between'>
            <dt className='text-muted-foreground'>Status</dt>
            <dd className='capitalize'>{status}</dd>
          </div>
        </dl>

        <div className='mt-5 grid gap-2 border-t pt-5'>
          {order.payment_url && (
            <Button
              type='button'
              onClick={() => openPaymentPopup(order.payment_url!)}
            >
              Reopen payment window
            </Button>
          )}
          <Button
            type='button'
            variant='outline'
            onClick={() => window.location.assign(orderPath)}
          >
            View order status
          </Button>
        </div>
      </div>

      <p className='text-muted-foreground mt-4 text-center text-xs'>
        Keep this tab open. If the window was blocked, use the button above.
      </p>
    </main>
  )
}

function loadSnapScript(production: boolean, clientKey: string) {
  return new Promise<boolean>((resolve) => {
    if (window.snap) {
      resolve(true)
      return
    }

    const script = document.createElement('script')
    script.src = production
      ? 'https://app.midtrans.com/snap/snap.js'
      : 'https://app.sandbox.midtrans.com/snap/snap.js'
    script.setAttribute('data-client-key', clientKey)
    script.onload = () => resolve(true)
    script.onerror = () => resolve(false)
    document.body.appendChild(script)
  })
}

type Draft = { form?: Partial<Form>; pin?: LatLon }

function readDraft(): Draft {
  try {
    const raw = sessionStorage.getItem(DRAFT_KEY)
    return raw ? (JSON.parse(raw) as Draft) : {}
  } catch {
    return {}
  }
}

function writeDraft(draft: Draft) {
  try {
    sessionStorage.setItem(DRAFT_KEY, JSON.stringify(draft))
  } catch {
    // storage unavailable (private mode) - drafts are a convenience only
  }
}

function clearDraft() {
  try {
    sessionStorage.removeItem(DRAFT_KEY)
  } catch {
    // ignore
  }
}
