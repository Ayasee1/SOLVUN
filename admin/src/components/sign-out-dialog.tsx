import { useState } from 'react'
import { useNavigate, useLocation } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { signOut } from '@/features/auth/api'

interface SignOutDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function SignOutDialog({ open, onOpenChange }: SignOutDialogProps) {
  const [signingOut, setSigningOut] = useState(false)
  const navigate = useNavigate()
  const location = useLocation()
  const { auth } = useAuthStore()

  const handleSignOut = async () => {
    if (signingOut) return
    setSigningOut(true)
    try {
      // Revokes the session server-side and clears the HttpOnly cookie.
      await signOut()
    } catch {
      // Even if the request fails, drop the local state so the UI signs out.
    } finally {
      setSigningOut(false)
      auth.reset()
      // Preserve current location for redirect after sign-in
      const currentPath = location.href
      void navigate({
        to: '/sign-in',
        search: { redirect: currentPath },
        replace: true,
      })
    }
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      title='Sign out'
      desc='Are you sure you want to sign out? You will need to sign in again to access your account.'
      confirmText='Sign out'
      destructive
      isLoading={signingOut}
      handleConfirm={() => void handleSignOut()}
      className='sm:max-w-sm'
    />
  )
}
