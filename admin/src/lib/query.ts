import { toast } from 'sonner'

// Wraps a query function so a failed request still surfaces as a toast, the
// same behaviour the pages had when they fetched manually.
export async function queryWithToast<T>(run: () => Promise<T>): Promise<T> {
  try {
    return await run()
  } catch (error) {
    toast.error(error instanceof Error ? error.message : 'Request failed')
    throw error
  }
}
