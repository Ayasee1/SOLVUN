import { beforeEach, describe, expect, it, vi } from 'vitest'

async function importAuthStore() {
  const { useAuthStore } = await import('./auth-store')
  return useAuthStore
}

const sampleUser = {
  email: 'owner@solvun.test',
}

describe('useAuthStore', () => {
  beforeEach(() => {
    vi.resetModules()
  })

  it('starts signed out', async () => {
    const useAuthStore = await importAuthStore()

    expect(useAuthStore.getState().auth.user).toBeNull()
  })

  it('updates the signed-in user via setUser', async () => {
    const useAuthStore = await importAuthStore()

    useAuthStore.getState().auth.setUser({ ...sampleUser })

    expect(useAuthStore.getState().auth.user).toEqual(sampleUser)
  })

  it('reset clears the user', async () => {
    const useAuthStore = await importAuthStore()
    useAuthStore.getState().auth.setUser({ ...sampleUser })

    useAuthStore.getState().auth.reset()

    expect(useAuthStore.getState().auth.user).toBeNull()
  })

  it('keeps only the email, with no token in the store', async () => {
    const useAuthStore = await importAuthStore()

    useAuthStore.getState().auth.setUser({ ...sampleUser })

    const state = useAuthStore.getState().auth as Record<string, unknown>
    expect(state.user).toEqual(sampleUser)
    expect(state.accessToken).toBeUndefined()
  })
})
