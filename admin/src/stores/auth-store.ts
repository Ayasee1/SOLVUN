import { create } from 'zustand'

export interface AuthUser {
  email: string
}

interface AuthState {
  auth: {
    user: AuthUser | null
    setUser: (user: AuthUser | null) => void
    reset: () => void
  }
}

// Only the signed-in operator's email is kept in memory. The session token
// itself is an HttpOnly cookie owned by the API, so there is nothing to persist
// in localStorage or a readable cookie.
export const useAuthStore = create<AuthState>()((set) => ({
  auth: {
    user: null,
    setUser: (user) => set((state) => ({ auth: { ...state.auth, user } })),
    reset: () => set((state) => ({ auth: { ...state.auth, user: null } })),
  },
}))
