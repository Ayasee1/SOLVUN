import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { ApiError } from '@/lib/api'
import { AuthenticatedLayout } from '@/components/layout/authenticated-layout'
import { fetchSession } from '@/features/auth/api'

export const Route = createFileRoute('/_authenticated')({
  // The session token is an HttpOnly cookie, so the router asks the API who is
  // signed in instead of reading a token from JavaScript.
  beforeLoad: async ({ location }) => {
    const { auth } = useAuthStore.getState()
    if (auth.user) {
      return
    }

    try {
      const session = await fetchSession()
      auth.setUser({ email: session.email })
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        throw redirect({
          to: '/sign-in',
          search: { redirect: location.href },
        })
      }
      // Any other failure (network, 5xx) is left to the page so it can show an
      // error state instead of bouncing the operator to the sign-in screen.
    }
  },
  component: AuthenticatedLayout,
})
