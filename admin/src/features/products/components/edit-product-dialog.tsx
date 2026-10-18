import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { updateProduct } from '../api'
import type { Product } from '../data/schema'

type Props = {
  product: Product | null
  onClose: () => void
  onSaved: () => void
}

export function EditProductDialog({ product, onClose, onSaved }: Props) {
  const [price, setPrice] = useState('')
  const [stock, setStock] = useState('')
  const [available, setAvailable] = useState(true)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (product) {
      setPrice(String(product.price))
      setStock(String(product.stock))
      setAvailable(product.is_available)
    }
  }, [product])

  async function save() {
    if (!product) return

    const parsedPrice = Number(price)
    const parsedStock = Number(stock)
    if (!Number.isInteger(parsedPrice) || parsedPrice < 0) {
      toast.error('Price must be a non-negative integer (rupiah)')
      return
    }
    if (!Number.isInteger(parsedStock) || parsedStock < 0) {
      toast.error('Stock must be a non-negative integer')
      return
    }

    setSaving(true)
    try {
      await updateProduct(product.id, {
        price: parsedPrice,
        stock: parsedStock,
        is_available: available,
      })
      toast.success(`Product #${product.id} updated`)
      onSaved()
      onClose()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Update failed')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={product !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>Edit {product?.name}</DialogTitle>
          <DialogDescription>
            Price, stock and availability are runtime values stored in the
            database. Name, description and images come from{' '}
            <code className='text-xs'>catalog/*.json</code>.
          </DialogDescription>
        </DialogHeader>

        <div className='grid gap-4 py-2'>
          <div className='grid gap-2'>
            <Label htmlFor='price'>Price (IDR)</Label>
            <Input
              id='price'
              inputMode='numeric'
              value={price}
              onChange={(event) => setPrice(event.target.value)}
            />
          </div>

          <div className='grid gap-2'>
            <Label htmlFor='stock'>Stock</Label>
            <Input
              id='stock'
              inputMode='numeric'
              value={stock}
              onChange={(event) => setStock(event.target.value)}
            />
          </div>

          <div className='flex items-center justify-between rounded-md border p-3'>
            <div>
              <Label htmlFor='available'>Available in storefront</Label>
              <p className='text-muted-foreground text-xs'>
                Hidden products stay in the database but are not sold.
              </p>
            </div>
            <Switch
              id='available'
              checked={available}
              onCheckedChange={setAvailable}
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant='outline' onClick={onClose} disabled={saving}>
            Cancel
          </Button>
          <Button onClick={() => void save()} disabled={saving}>
            {saving ? 'Saving...' : 'Save'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
