import type { ErrorResponse } from './types'

/** A non-2xx response (or a network failure, status 0) from the Go API. */
export class ApiError extends Error {
  readonly status: number
  readonly body: ErrorResponse | null

  constructor(status: number, message: string, body: ErrorResponse | null = null) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.body = body
  }
}

async function readError(res: Response): Promise<ApiError> {
  let body: ErrorResponse | null = null
  try {
    body = (await res.json()) as ErrorResponse
  } catch {
    // not JSON (e.g. a proxy error page) — fall through to the generic message
  }
  return new ApiError(res.status, body?.error || `Request failed (${res.status})`, body)
}

/** fetch + JSON decode, throwing ApiError for anything that is not a 2xx. A 204 has no body and yields undefined. */
export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response
  try {
    res = await fetch(path, init)
  } catch {
    throw new ApiError(0, 'Cannot reach the server. Check your connection and try again.')
  }
  if (!res.ok) throw await readError(res)
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : 'Something went wrong.'
}
