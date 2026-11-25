import { api } from '@/lib/api'

export interface AdminSession {
  email: string
}

interface SignInResponse {
  email: string
  expires_in: number
}

// signIn exchanges the operator credentials for an HttpOnly session cookie.
export function signIn(email: string, password: string) {
  return api.post<SignInResponse>('/admin/session', { email, password })
}

export function signOut() {
  return api.delete<void>('/admin/session')
}

export function fetchSession() {
  return api.get<AdminSession>('/admin/session')
}
