import { describe, expect, it } from 'bun:test'
import { applyPinnedPlace, type AddressValues } from './address'

const empty: AddressValues = { street: '', city: '', state: '', zip: '' }

describe('applyPinnedPlace', () => {
  it('fills an empty address', () => {
    const { values, autoFilled, missing } = applyPinnedPlace(empty, {}, {
      street: 'Jl Asia Afrika 1',
      city: 'Bandung',
      state: 'Jawa Barat',
      postalCode: '40111',
    })

    expect(values).toEqual({
      street: 'Jl Asia Afrika 1',
      city: 'Bandung',
      state: 'Jawa Barat',
      zip: '40111',
    })
    expect(autoFilled).toEqual(values)
    expect(missing).toEqual([])
  })

  it('replaces a manually typed address when the pin moves', () => {
    const typed: AddressValues = {
      street: 'Jl Braga 5',
      city: 'Bandung',
      state: 'Jawa Barat',
      zip: '40111',
    }

    // No prior auto-fill: the shopper typed this address themselves.
    const { values } = applyPinnedPlace(typed, {}, {
      street: 'Jl Pajajaran 10',
      city: 'Bogor',
      state: 'Jawa Barat',
      postalCode: '16111',
    })

    expect(values).toEqual({
      street: 'Jl Pajajaran 10',
      city: 'Bogor',
      state: 'Jawa Barat',
      zip: '16111',
    })
  })

  it('replaces a previously auto-filled address', () => {
    const first = applyPinnedPlace(empty, {}, {
      street: 'Jl A',
      city: 'Bandung',
      state: 'Jawa Barat',
      postalCode: '40111',
    })

    const second = applyPinnedPlace(first.values, first.autoFilled, {
      street: 'Jl B',
      city: 'Bogor',
      state: 'Jawa Barat',
      postalCode: '16111',
    })

    expect(second.values.street).toBe('Jl B')
    expect(second.values.zip).toBe('16111')
  })

  it('clears an auto-filled field the new pin cannot resolve', () => {
    const first = applyPinnedPlace(empty, {}, {
      street: 'Jl A',
      city: 'Bandung',
      state: 'Jawa Barat',
      postalCode: '40111',
    })

    const second = applyPinnedPlace(first.values, first.autoFilled, {
      city: 'Bogor',
      state: 'Jawa Barat',
      postalCode: '16111',
    })

    expect(second.values.street).toBe('')
    expect(second.values.city).toBe('Bogor')
    expect(second.missing).toEqual(['street'])
  })

  it('keeps a hand-typed field the new pin cannot resolve', () => {
    const typed: AddressValues = {
      street: 'Gang Melati 3',
      city: '',
      state: '',
      zip: '',
    }

    const { values } = applyPinnedPlace(typed, {}, {
      city: 'Bogor',
      state: 'Jawa Barat',
      postalCode: '16111',
    })

    expect(values.street).toBe('Gang Melati 3')
    expect(values.city).toBe('Bogor')
  })

  it('normalizes the postal code', () => {
    const { values } = applyPinnedPlace(empty, {}, {
      street: 'Jl X',
      city: 'Bandung',
      state: 'Jawa Barat',
      postalCode: '40111-1234',
    })

    expect(values.zip).toBe('40111')
  })

  it('reports every field the pin left empty', () => {
    const { values, missing } = applyPinnedPlace(empty, {}, {})

    expect(values).toEqual(empty)
    expect(missing).toEqual(['street', 'city', 'state', 'zip'])
  })
})
