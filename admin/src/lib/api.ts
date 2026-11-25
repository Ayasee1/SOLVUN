export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

async function errorFrom(res: Response): Promise<ApiError> {
  let message = res.statusText
  try {
    const body = await res.json()
    if (body?.error) message = body.error
  } catch {
    // response had no JSON body
  }
  return new ApiError(res.status, message)
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`/api${path}`, {
    ...init,
    // The admin session lives in an HttpOnly cookie, so every request has to
    // carry it. No API key is compiled into the bundle.
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...init?.headers,
    },
  })

  if (!res.ok) {
    throw await errorFrom(res)
  }

  if (res.status === 204) {
    return undefined as T
  }
  return (await res.json()) as T
}

// upload posts multipart data (file uploads). The browser sets the boundary, so
// no Content-Type header here.
async function upload<T>(path: string, formData: FormData): Promise<T> {
  const res = await fetch(`/api${path}`, {
    method: 'POST',
    credentials: 'include',
    body: formData,
  })

  if (!res.ok) {
    throw await errorFrom(res)
  }

  return (await res.json()) as T
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, {
      method: 'POST',
      body: body === undefined ? undefined : JSON.stringify(body),
    }),
  patch: <T>(path: string, body: unknown) =>
    request<T>(path, { method: 'PATCH', body: JSON.stringify(body) }),
  delete: <T>(path: string) => request<T>(path, { method: 'DELETE' }),
  upload,
}
