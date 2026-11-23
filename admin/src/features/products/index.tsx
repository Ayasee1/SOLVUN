import { useMemo, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { queryWithToast } from '@/lib/query'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ConfigDrawer } from '@/components/config-drawer'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { fetchProducts, updateProduct } from './api'
import { EditProductDialog } from './components/edit-product-dialog'
import type { Product } from './data/schema'

const rupiah = new Intl.NumberFormat('id-ID', {
  style: 'currency',
  currency: 'IDR',
  maximumFractionDigits: 0,
})

let formatter: Intl.DateTimeFormat | undefined

function formatDate(value: string) {
  if (!value || value.startsWith('0001-')) return 'never'
  formatter ??= new Intl.DateTimeFormat('id-ID', {
    dateStyle: 'medium',
    timeStyle: 'short',
  })
  return formatter.format(new Date(value))
}

export function Products() {
  const [search, setSearch] = useState('')
  const [editing, setEditing] = useState<Product | null>(null)
  const [togglingId, setTogglingId] = useState<number | null>(null)
  const queryClient = useQueryClient()
  const productsQuery = useQuery({
    queryKey: ['products'],
    queryFn: () => queryWithToast(fetchProducts),
    retry: false,
  })

  const products = productsQuery.data ?? []
  const loading = productsQuery.isLoading

  const visible = useMemo(() => {
    const query = search.trim().toLowerCase()
    if (!query) return products
    return products.filter((product) =>
      product.name.toLowerCase().includes(query)
    )
  }, [products, search])

  async function toggleAvailability(product: Product) {
    setTogglingId(product.id)
    try {
      const updated = await updateProduct(product.id, {
        is_available: !product.is_available,
      })
      queryClient.setQueryData<Product[]>(['products'], (current) =>
        current?.map((item) => (item.id === updated.id ? updated : item))
      )
      toast.success(
        `${updated.name} is now ${updated.is_available ? 'available' : 'hidden'}`
      )
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Update failed')
    } finally {
      setTogglingId(null)
    }
  }

  return (
    <>
      <Header fixed>
        <Search className='me-auto' />
        <ThemeSwitch />
        <ConfigDrawer />
        <ProfileDropdown />
      </Header>

      <Main className='flex flex-1 flex-col gap-4 sm:gap-6'>
        <div className='flex flex-wrap items-end justify-between gap-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>Products</h2>
            <p className='text-muted-foreground'>
              Manage price, stock and availability. Product content (name,
              description, images) lives in{' '}
              <code className='text-xs'>catalog/*.json</code>.
            </p>
          </div>

          <div className='flex items-center gap-2'>
            <Input
              placeholder='Search by name...'
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              className='w-56'
            />
            <Button
              variant='outline'
              onClick={() => void productsQuery.refetch()}
            >
              Refresh
            </Button>
          </div>
        </div>

        <div className='overflow-hidden rounded-md border'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className='w-16'>ID</TableHead>
                <TableHead>Product</TableHead>
                <TableHead className='text-right'>Price</TableHead>
                <TableHead className='text-right'>Stock</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Updated</TableHead>
                <TableHead className='text-right'>Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading &&
                Array.from({ length: 5 }).map((_, index) => (
                  <TableRow key={index}>
                    <TableCell colSpan={7}>
                      <Skeleton className='h-6 w-full' />
                    </TableCell>
                  </TableRow>
                ))}

              {!loading && visible.length === 0 && (
                <TableRow>
                  <TableCell
                    colSpan={7}
                    className='h-24 text-center text-muted-foreground'
                  >
                    No products found.
                  </TableCell>
                </TableRow>
              )}

              {!loading &&
                visible.map((product) => (
                  <TableRow key={product.id}>
                    <TableCell className='text-muted-foreground tabular-nums'>
                      {product.id}
                    </TableCell>
                    <TableCell className='max-w-80'>
                      <div className='font-medium'>{product.name}</div>
                      {product.description && (
                        <div className='truncate text-xs text-muted-foreground'>
                          {product.description}
                        </div>
                      )}
                    </TableCell>
                    <TableCell className='text-right tabular-nums'>
                      {rupiah.format(product.price)}
                    </TableCell>
                    <TableCell className='text-right tabular-nums'>
                      {product.stock}
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant='outline'
                        className={
                          product.is_available
                            ? 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-400'
                            : 'bg-muted text-muted-foreground'
                        }
                      >
                        {product.is_available ? 'available' : 'hidden'}
                      </Badge>
                    </TableCell>
                    <TableCell className='text-xs text-muted-foreground'>
                      {formatDate(product.updated_at)}
                    </TableCell>
                    <TableCell>
                      <div className='flex justify-end gap-2'>
                        <Button
                          size='sm'
                          variant='outline'
                          disabled={togglingId === product.id}
                          onClick={() => void toggleAvailability(product)}
                        >
                          {product.is_available ? 'Hide' : 'Show'}
                        </Button>
                        <Button size='sm' onClick={() => setEditing(product)}>
                          Edit
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
            </TableBody>
          </Table>
        </div>
      </Main>

      <EditProductDialog
        key={editing?.id ?? 'closed'}
        product={editing}
        onClose={() => setEditing(null)}
        onSaved={() => void productsQuery.refetch()}
      />
    </>
  )
}
