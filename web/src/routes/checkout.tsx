import { useEffect, useMemo, useRef, useState } from 'react'
import { createFileRoute, Link } from '@tanstack/react-router'
import { Crosshair, Loader2, MapPin } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import {
  MapPicker,
  forwardGeocode,
  reverseGeocode,
  type LatLon,
} from '@/components/map-picker'
import { useCart } from '@/features/cart/cart-store'
import { api, ApiError, fetchShippingRates, type ShippingRate } from '@/lib/api'

export const Route = createFileRoute('/checkout')({ component: Checkout })

const rupiah = new Intl.NumberFormat('id-ID', {
  style: 'currency',
  currency: 'IDR',
  maximumFractionDigits: 0,
})

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
  const [rates, setRates] = useState<ShippingRate[]>([])
  const [loadingRates, setLoadingRates] = useState(false)
  const [selected, setSelected] = useState('')
  const [locating, setLocating] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [autoFilled, setAutoFilled] = useState<Record<string, string>>({})
  const formRef = useRef<HTMLFormElement>(null)

  const selectedRate = useMemo(
    () => rates.find((rate) => key(rate) === selected),
    [rates, selected]
  )
  const shipping = selectedRate?.price ?? 0
  const total = subtotal + shipping

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
      return
    }

    let cancelled = false
    setLoadingRates(true)

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
      ;(container?.querySelector('input') as HTMLInputElement | undefined)?.focus()
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

  if (items.length === 0) {
    return (
      <main className='mx-auto max-w-3xl px-4 py-24 text-center'>
        <h1 className='text-xl font-semibold'>Nothing to check out</h1>
        <Button className='mt-6' asChild>
          <Link to='/'>Browse catalog</Link>
        </Button>
      </main>
    )
  }

  return (
    <main className='mx-auto max-w-5xl px-4 py-10'>
      <h1 className='text-2xl font-semibold tracking-tight'>Checkout</h1>

      <div className='mt-8 grid gap-6 lg:grid-cols-[1fr_360px]'>
        <form
          ref={formRef}
          className='space-y-6'
          onSubmit={(event) => {
            event.preventDefault()
            void pay()
          }}
        >
          <Card>
            <CardHeader>
              <CardTitle>Contact</CardTitle>
              <CardDescription>
                We send the receipt and delivery updates here.
              </CardDescription>
            </CardHeader>
            <CardContent className='space-y-4'>
              <Field
                name='contactName'
                label='Full name'
                autoComplete='name'
                value={form.contactName}
                error={errors.contactName}
                onChange={(value) => set('contactName', value)}
              />
              <Field
                name='email'
                label='Email'
                type='email'
                autoComplete='email'
                value={form.email}
                error={errors.email}
                onChange={(value) => set('email', value)}
              />
              <Field
                name='contactPhone'
                label='Phone'
                optional
                autoComplete='tel'
                value={form.contactPhone}
                onChange={(value) => set('contactPhone', value)}
              />
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Delivery</CardTitle>
              <CardDescription>
                Who receives the parcel and where. Useful when sending to
                someone else.
              </CardDescription>
            </CardHeader>
            <CardContent className='space-y-4'>
              <label className='flex cursor-pointer items-center gap-3 rounded-lg border p-3'>
                <input
                  type='checkbox'
                  className='accent-foreground size-4'
                  checked={form.sameAsContact}
                  onChange={(event) => {
                    set('sameAsContact', event.target.checked)
                    setErrors({})
                  }}
                />
                <span className='text-sm'>
                  The recipient is the same as me
                </span>
              </label>

              {!form.sameAsContact && (
                <>
                  <Field
                    name='recipientName'
                    label='Recipient name'
                    autoComplete='shipping name'
                    value={form.recipientName}
                    error={errors.recipientName}
                    onChange={(value) => set('recipientName', value)}
                  />
                  <Field
                    name='recipientPhone'
                    label='Recipient phone'
                    autoComplete='shipping tel'
                    value={form.recipientPhone}
                    error={errors.recipientPhone}
                    onChange={(value) => set('recipientPhone', value)}
                  />
                </>
              )}

              <Field
                name='street'
                label='Street address'
                autoComplete='street-address'
                value={form.street}
                error={errors.street}
                onChange={(value) => set('street', value)}
              />
              <Field
                name='line2'
                label='Apartment, unit, floor'
                optional
                autoComplete='address-line2'
                value={form.line2}
                onChange={(value) => set('line2', value)}
              />
              <Field
                name='city'
                label='City'
                autoComplete='address-level2'
                value={form.city}
                error={errors.city}
                onChange={(value) => set('city', value)}
              />
              <Field
                name='state'
                label='Province'
                autoComplete='address-level1'
                value={form.state}
                error={errors.state}
                onChange={(value) => set('state', value)}
              />
              <Field
                name='zip'
                label='Postal code'
                inputMode='numeric'
                autoComplete='postal-code'
                placeholder='40123'
                value={form.zip}
                error={errors.zip}
                onChange={(value) =>
                  set('zip', value.replace(/\D/g, '').slice(0, 5))
                }
              />

              <div className='flex justify-end'>
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
                  Locate typed address on map
                </Button>
              </div>

              <div className='space-y-2'>
                <MapPicker value={pin} onChange={setPin} />

                <div className='flex flex-wrap items-center gap-2'>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={useMyLocation}
                  >
                    <Crosshair className='size-4' />
                    Use my location
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

                <p className='text-muted-foreground text-xs'>
                  The pin is optional. It marks the exact delivery point, which
                  helps the courier — handy when sending to someone else.
                </p>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Shipping method</CardTitle>
              <CardDescription>
                {selectedRate
                  ? `${selectedRate.courier_name} ${selectedRate.service_name}`
                  : 'Enter a postal code to see options'}
              </CardDescription>
            </CardHeader>
            <CardContent className='space-y-2' data-field='shipping'>
              {loadingRates && (
                <>
                  <Skeleton className='h-14 w-full' />
                  <Skeleton className='h-14 w-full' />
                </>
              )}

              {!loadingRates && rates.length === 0 && (
                <p className='text-muted-foreground text-sm'>
                  {/^\d{5}$/.test(form.zip)
                    ? 'No shipping options for this postal code.'
                    : 'Enter a postal code above to see courier options.'}
                </p>
              )}

              {!loadingRates && rates.length > 0 && (
                <RadioGroup value={selected} onValueChange={setSelected}>
                  {rates.map((rate) => (
                    <label
                      key={key(rate)}
                      htmlFor={key(rate)}
                      className='hover:bg-muted/50 flex cursor-pointer items-center gap-3 rounded-lg border p-3'
                    >
                      <RadioGroupItem value={key(rate)} id={key(rate)} />
                      <div className='flex-1'>
                        <p className='text-sm font-medium'>
                          {rate.courier_name} · {rate.service_name}
                        </p>
                        {rate.duration && (
                          <p className='text-muted-foreground text-xs'>
                            {rate.duration}
                          </p>
                        )}
                      </div>
                      <span className='text-sm font-medium tabular-nums'>
                        {rupiah.format(rate.price)}
                      </span>
                    </label>
                  ))}
                </RadioGroup>
              )}

              {errors.shipping && (
                <p className='text-destructive text-xs'>{errors.shipping}</p>
              )}
            </CardContent>
          </Card>
        </form>

        <Card className='h-fit lg:sticky lg:top-20'>
          <CardHeader>
            <CardTitle>Summary</CardTitle>
          </CardHeader>
          <CardContent className='space-y-3'>
            {items.map((item) => (
              <div key={item.product_id} className='flex justify-between text-sm'>
                <span className='text-muted-foreground'>
                  {item.name} ×{item.qty}
                </span>
                <span className='tabular-nums'>
                  {rupiah.format(item.price * item.qty)}
                </span>
              </div>
            ))}

            <Separator />

            <div className='flex justify-between text-sm'>
              <span className='text-muted-foreground'>Subtotal</span>
              <span className='tabular-nums'>{rupiah.format(subtotal)}</span>
            </div>
            <div className='flex justify-between text-sm'>
              <span className='text-muted-foreground'>Shipping</span>
              <span className='tabular-nums'>
                {selectedRate ? rupiah.format(shipping) : '—'}
              </span>
            </div>

            <Separator />

            <div className='flex justify-between font-semibold'>
              <span>Total</span>
              <span className='tabular-nums'>{rupiah.format(total)}</span>
            </div>

            <Button
              className='mt-2 w-full'
              type='button'
              disabled={submitting}
              onClick={() => void pay()}
            >
              {submitting ? (
                <>
                  <Loader2 className='size-4 animate-spin' />
                  Starting payment...
                </>
              ) : (
                'Pay now'
              )}
            </Button>

            <p className='text-muted-foreground text-center text-xs'>
              You will be redirected to the secure payment page.
            </p>
          </CardContent>
        </Card>
      </div>
    </main>
  )
}

function Field({
  name,
  label,
  value,
  onChange,
  error,
  optional,
  type = 'text',
  placeholder,
  inputMode,
  autoComplete,
}: {
  name: string
  label: string
  value: string
  onChange: (value: string) => void
  error?: string
  optional?: boolean
  type?: string
  placeholder?: string
  inputMode?: 'numeric' | 'text'
  autoComplete?: string
}) {
  return (
    <div className='grid gap-2' data-field={name}>
      <Label htmlFor={name}>
        {label}
        {optional && (
          <span className='text-muted-foreground font-normal'> (optional)</span>
        )}
      </Label>
      <Input
        id={name}
        type={type}
        inputMode={inputMode}
        placeholder={placeholder}
        autoComplete={autoComplete}
        value={value}
        aria-invalid={error ? true : undefined}
        onChange={(event) => onChange(event.target.value)}
      />
      {error && <p className='text-destructive text-xs'>{error}</p>}
    </div>
  )
}

const key = (rate: ShippingRate) => `${rate.courier}|${rate.service}`

// Opens the Snap popup when a client key is configured, otherwise falls back to
// the hosted payment page (which always works).
async function startPayment(order: CreatedOrder) {
  const orderPath = `/order/${order.id}`

  if (!order.snap_token) {
    window.location.href = order.payment_url ?? orderPath
    return
  }

  let config: StoreConfig | undefined
  try {
    config = await api.get<StoreConfig>('/store/config')
  } catch {
    config = undefined
  }

  const clientKey = config?.midtrans_client_key?.trim()
  if (
    !clientKey ||
    !(await loadSnapScript(config?.midtrans_env === 'production', clientKey))
  ) {
    window.location.href = order.payment_url ?? orderPath
    return
  }

  window.snap?.pay(order.snap_token, {
    onSuccess: () => (window.location.href = orderPath),
    onPending: () => (window.location.href = orderPath),
    onError: () => (window.location.href = orderPath),
    onClose: () => (window.location.href = orderPath),
  })
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
