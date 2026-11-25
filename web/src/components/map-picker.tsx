import { useEffect, useMemo, useState } from 'react'
import L from 'leaflet'
import { MapContainer, Marker, TileLayer, useMap, useMapEvents } from 'react-leaflet'
import 'leaflet/dist/leaflet.css'

// A small dot marker avoids Leaflet's default icon assets, which break under
// bundlers without extra configuration.
const dotIcon = L.divIcon({
  className: '',
  html: '<span style="display:block;width:18px;height:18px;border-radius:50%;background:var(--primary);border:3px solid #fff;box-shadow:0 1px 6px rgba(0,0,0,.35)"></span>',
  iconSize: [18, 18],
  iconAnchor: [9, 9],
})

export type LatLon = { lat: number; lon: number }

type Props = {
  value?: LatLon
  onChange: (position: LatLon) => void
}

const DEFAULT_CENTER: [number, number] = [-6.9175, 107.6191] // Bandung

export function MapPicker({ value, onChange }: Props) {
  const [position, setPosition] = useState<LatLon | null>(value ?? null)

  useEffect(() => {
    if (!value) {
      setPosition(null)
      return
    }
    if (!position || value.lat !== position.lat || value.lon !== position.lon) {
      setPosition(value)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value?.lat, value?.lon])

  const move = useMemo(
    () => (next: LatLon) => {
      setPosition(next)
      onChange(next)
    },
    [onChange]
  )

  return (
    <div className='overflow-hidden rounded-xl border'>
      <MapContainer
        center={value ? [value.lat, value.lon] : DEFAULT_CENTER}
        zoom={13}
        scrollWheelZoom={false}
        style={{ height: 260, width: '100%' }}
      >
        <TileLayer
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
          url='https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png'
        />
        <ClickHandler onPick={move} />
        <Recenter position={position} />
        {position && (
          <Marker
            position={[position.lat, position.lon]}
            icon={dotIcon}
            draggable
            eventHandlers={{
              dragend: (event) => {
                const marker = event.target as L.Marker
                const { lat, lng } = marker.getLatLng()
                move({ lat, lon: lng })
              },
            }}
          />
        )}
      </MapContainer>

      <div className='text-muted-foreground flex items-center justify-between gap-2 border-t px-3 py-2 text-xs'>
        {position ? (
          <span className='font-mono'>
            {position.lat.toFixed(6)}, {position.lon.toFixed(6)}
          </span>
        ) : (
          <span>Tap the map to drop the delivery pin</span>
        )}
      </div>
    </div>
  )
}

function ClickHandler({ onPick }: { onPick: (position: LatLon) => void }) {
  useMapEvents({
    click(event) {
      onPick({ lat: event.latlng.lat, lon: event.latlng.lng })
    },
  })
  return null
}

function Recenter({ position }: { position: LatLon | null }) {
  const map = useMap()

  useEffect(() => {
    if (position) {
      map.setView([position.lat, position.lon], map.getZoom(), { animate: true })
    }
  }, [map, position?.lat, position?.lon])

  return null
}

// Best-effort reverse geocoding with OpenStreetMap Nominatim (no API key).
// Looks up a place by free-text address (used by "locate on map").
export async function forwardGeocode(query: string): Promise<LatLon | null> {
  if (!query.trim()) return null

  try {
    const res = await fetch(
      `https://nominatim.openstreetmap.org/search?format=jsonv2&limit=1&countrycodes=id&q=${encodeURIComponent(query)}`,
      { headers: { Accept: 'application/json' } }
    )
    if (!res.ok) return null

    const results = (await res.json()) as { lat: string; lon: string }[]
    if (results.length === 0) return null

    return {
      lat: Number(results[0].lat),
      lon: Number(results[0].lon),
    }
  } catch {
    return null
  }
}

export async function reverseGeocode({ lat, lon }: LatLon) {
  try {
    const res = await fetch(
      `https://nominatim.openstreetmap.org/reverse?format=jsonv2&lat=${lat}&lon=${lon}&accept-language=id`,
      { headers: { Accept: 'application/json' } }
    )
    if (!res.ok) return null

    const data = (await res.json()) as {
      address?: Record<string, string>
    }
    const address = data.address ?? {}

    return {
      street: [address.road, address.house_number].filter(Boolean).join(' '),
      city:
        address.city ?? address.town ?? address.village ?? address.county ?? '',
      state: address.state ?? '',
      postalCode: address.postcode ?? '',
    }
  } catch {
    return null
  }
}
