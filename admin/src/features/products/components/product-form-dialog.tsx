import { useRef, useState } from 'react'
import { ImagePlus, X } from 'lucide-react'
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
import { Textarea } from '@/components/ui/textarea'
import { createProduct, updateProduct, uploadProductImage } from '../api'
import type { Product } from '../data/schema'

type Props = {
  product: Product | null
  onClose: () => void
  onSaved: () => void
}

// ProductFormDialog creates a product when product is null and edits it
// otherwise. Uploaded images are stored outside the web build, so a deploy does
// not touch them.
export function ProductFormDialog({ product, onClose, onSaved }: Props) {
  const isEdit = product !== null

  const [name, setName] = useState(product?.name ?? '')
  const [description, setDescription] = useState(product?.description ?? '')
  const [price, setPrice] = useState(product ? String(product.price) : '')
  const [stock, setStock] = useState(product ? String(product.stock) : '')
  const [weight, setWeight] = useState(
    product ? String(product.weight_gram) : ''
  )
  const [available, setAvailable] = useState(product?.is_available ?? true)
  const [images, setImages] = useState<string[]>(product?.images ?? [])
  const [imageURL, setImageURL] = useState('')
  const [uploading, setUploading] = useState(false)
  const [saving, setSaving] = useState(false)
  const fileInput = useRef<HTMLInputElement>(null)

  async function onFiles(files: FileList | null) {
    if (!files?.length) return
    setUploading(true)
    try {
      for (const file of Array.from(files)) {
        const { url } = await uploadProductImage(file)
        setImages((current) => [...current, url])
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Upload failed')
    } finally {
      setUploading(false)
      if (fileInput.current) fileInput.current.value = ''
    }
  }

  function addImageURL() {
    const value = imageURL.trim()
    if (!value) return
    if (
      !value.startsWith('/') &&
      !value.startsWith('http://') &&
      !value.startsWith('https://')
    ) {
      toast.error('Use a /path or an http(s) URL')
      return
    }
    setImages((current) => [...current, value])
    setImageURL('')
  }

  async function save() {
    const trimmedName = name.trim()
    const parsedPrice = Number(price)
    const parsedStock = Number(stock)
    const parsedWeight = Number(weight)

    if (!trimmedName) {
      toast.error('Name is required')
      return
    }
    if (!Number.isInteger(parsedPrice) || parsedPrice < 0) {
      toast.error('Price must be a non-negative integer (rupiah)')
      return
    }
    if (!Number.isInteger(parsedStock) || parsedStock < 0) {
      toast.error('Stock must be a non-negative integer')
      return
    }
    if (!Number.isInteger(parsedWeight) || parsedWeight < 1) {
      toast.error('Weight must be a positive integer (grams)')
      return
    }
    if (images.length > 8) {
      toast.error('At most 8 images')
      return
    }

    const input = {
      name: trimmedName,
      description: description.trim(),
      images,
      price: parsedPrice,
      stock: parsedStock,
      is_available: available,
      weight_gram: parsedWeight,
    }

    setSaving(true)
    try {
      if (product) {
        await updateProduct(product.id, input)
        toast.success(`Product #${product.id} updated`)
      } else {
        const created = await createProduct(input)
        toast.success(`Product #${created.id} created`)
      }
      onSaved()
      onClose()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Save failed')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open onOpenChange={(next) => !next && onClose()}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>
            {isEdit ? `Edit ${product?.name}` : 'Add product'}
          </DialogTitle>
          <DialogDescription>
            Everything is stored in the database. Uploaded images live outside
            the web build, so deploys leave them alone.
          </DialogDescription>
        </DialogHeader>

        <div className='grid gap-4 py-2'>
          <div className='grid gap-2'>
            <Label htmlFor='name'>Name</Label>
            <Input
              id='name'
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder='Denim Jacket Type 01'
            />
          </div>

          <div className='grid gap-2'>
            <Label htmlFor='description'>Description</Label>
            <Textarea
              id='description'
              rows={4}
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              placeholder='Heavyweight indigo denim...'
            />
          </div>

          <div className='grid grid-cols-2 gap-3'>
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
          </div>

          <div className='grid grid-cols-2 gap-3'>
            <div className='grid gap-2'>
              <Label htmlFor='weight'>Weight (grams)</Label>
              <Input
                id='weight'
                inputMode='numeric'
                value={weight}
                onChange={(event) => setWeight(event.target.value)}
              />
            </div>
            <div className='flex items-end'>
              <p className='text-xs text-muted-foreground'>
                Shipping is quoted from the weight and the destination.
              </p>
            </div>
          </div>

          <div className='grid gap-2'>
            <Label>Images</Label>
            <div className='flex flex-wrap gap-2'>
              {images.map((src) => (
                <div key={src} className='relative'>
                  <img
                    src={src}
                    alt=''
                    className='h-20 w-20 rounded-md border object-cover'
                  />
                  <button
                    type='button'
                    aria-label='Remove image'
                    onClick={() =>
                      setImages((current) =>
                        current.filter((item) => item !== src)
                      )
                    }
                    className='absolute -top-1.5 -right-1.5 rounded-full border bg-background p-0.5 shadow-sm'
                  >
                    <X className='size-3' />
                  </button>
                </div>
              ))}

              <Button
                type='button'
                variant='outline'
                className='h-20 w-20'
                disabled={uploading}
                onClick={() => fileInput.current?.click()}
              >
                <ImagePlus className='size-5' />
              </Button>
              <input
                ref={fileInput}
                type='file'
                accept='image/jpeg,image/png,image/webp'
                multiple
                className='hidden'
                onChange={(event) => void onFiles(event.target.files)}
              />
            </div>

            <div className='flex gap-2'>
              <Input
                value={imageURL}
                onChange={(event) => setImageURL(event.target.value)}
                placeholder='/images/products/x.jpg or https://...'
              />
              <Button type='button' variant='outline' onClick={addImageURL}>
                Add
              </Button>
            </div>
            <p className='text-xs text-muted-foreground'>
              JPEG, PNG or WebP, up to 5 MB each. Uploaded files are served from
              /media.
            </p>
          </div>

          <div className='flex items-center justify-between rounded-md border p-3'>
            <div>
              <Label htmlFor='available'>Available in storefront</Label>
              <p className='text-xs text-muted-foreground'>
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
          <Button onClick={() => void save()} disabled={saving || uploading}>
            {saving ? 'Saving...' : isEdit ? 'Save' : 'Create product'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
