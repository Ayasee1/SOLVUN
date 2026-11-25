export type AddressField = 'street' | 'city' | 'state' | 'zip'

export type AddressValues = Record<AddressField, string>

export type AutoFilledAddress = Partial<Record<AddressField, string>>

export type PinnedPlace = {
  street?: string
  city?: string
  state?: string
  postalCode?: string
}

export type AppliedPlace = {
  values: AddressValues
  autoFilled: AutoFilledAddress
  missing: AddressField[]
}

const FIELDS: AddressField[] = ['street', 'city', 'state', 'zip']

// applyPinnedPlace merges a reverse-geocoded pin into the address form.
//
// "Fill address from pin" is an explicit action, so a resolved value always
// replaces what is in the field, including an address the shopper typed by hand.
// That was the bug: the old code refused to overwrite anything that was not
// empty or previously auto-filled, so moving from pin A to pin B changed nothing.
//
// When the new pin cannot resolve a field, a value that belonged to the previous
// pin is cleared (otherwise A and B would be mixed), while something the shopper
// typed is kept. Fields still empty afterwards are reported as missing.
export function applyPinnedPlace(
  current: AddressValues,
  previousAuto: AutoFilledAddress,
  place: PinnedPlace
): AppliedPlace {
  const candidates: AddressValues = {
    street: (place.street ?? '').trim(),
    city: (place.city ?? '').trim(),
    state: (place.state ?? '').trim(),
    zip: (place.postalCode ?? '').replace(/\D/g, '').slice(0, 5),
  }

  const values = { ...current }
  const autoFilled = { ...previousAuto }
  const missing: AddressField[] = []

  for (const field of FIELDS) {
    const candidate = candidates[field]

    if (candidate) {
      values[field] = candidate
      autoFilled[field] = candidate
      continue
    }

    if (previousAuto[field] && current[field] === previousAuto[field]) {
      values[field] = ''
      delete autoFilled[field]
    }

    if (!values[field]) {
      missing.push(field)
    }
  }

  return { values, autoFilled, missing }
}
