import { afterEach, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { webBackend } from './web-api'
import { WebLogin } from './WebLogin'
import { UIProvider } from './components/ui'
afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  delete window.runtime
})
it('sends typed RPC arguments with same-origin credentials and propagates backend errors', async () => {
  const request = vi
    .fn()
    .mockResolvedValue(new Response(JSON.stringify({ result: { stage: 'nickname' } })))
  vi.stubGlobal('fetch', request)
  expect(await webBackend().Bootstrap()).toEqual({ stage: 'nickname' })
  expect(request).toHaveBeenCalledWith(
    '/api/rpc',
    expect.objectContaining({
      credentials: 'same-origin',
      body: JSON.stringify({ method: 'Bootstrap', args: [] }),
    }),
  )
  request.mockResolvedValue(new Response(JSON.stringify({ error: 'The draft changed' })))
  await expect(webBackend().Bootstrap()).rejects.toThrow('The draft changed')
  request.mockResolvedValue(new Response('', { status: 401 }))
  await expect(webBackend().Bootstrap()).rejects.toThrow('Sign in')
})
it('keeps failed web login on screen and enters the workspace only on success', async () => {
  const request = vi.fn().mockResolvedValue(new Response('Invalid login token.', { status: 401 }))
  vi.stubGlobal('fetch', request)
  const login = vi.fn()
  render(
    <UIProvider>
      <WebLogin onLogin={login} />
    </UIProvider>,
  )
  fireEvent.change(screen.getByLabelText('Server login token'), {
    target: { value: 'synthetic-token' },
  })
  fireEvent.submit(screen.getByRole('button', { name: 'Sign in' }).closest('form')!)
  await screen.findByRole('alert')
  expect(login).not.toHaveBeenCalled()
  request.mockResolvedValue(new Response(null, { status: 204 }))
  fireEvent.submit(screen.getByRole('button', { name: 'Sign in' }).closest('form')!)
  await waitFor(() => expect(login).toHaveBeenCalledTimes(1))
  expect(request).toHaveBeenLastCalledWith(
    '/api/login',
    expect.objectContaining({ body: JSON.stringify({ token: 'synthetic-token' }) }),
  )
})
