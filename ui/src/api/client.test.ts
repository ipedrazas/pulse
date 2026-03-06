import { describe, it, expect, vi, beforeEach } from 'vitest'
import { getNodes, getNode, getContainers, getContainer, getHealth } from './client'

const mockFetch = vi.fn()

beforeEach(() => {
  vi.stubGlobal('fetch', mockFetch)
  mockFetch.mockReset()
})

function mockOk(data: unknown) {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    json: () => Promise.resolve(data),
  })
}

function mockError(status: number, body: string) {
  mockFetch.mockResolvedValueOnce({
    ok: false,
    status,
    text: () => Promise.resolve(body),
  })
}

describe('getNodes', () => {
  it('calls correct URL and returns data', async () => {
    const data = [{ name: 'node-1', status: 'online' }]
    mockOk(data)

    const result = await getNodes()
    expect(mockFetch).toHaveBeenCalledWith('/api/v1/nodes')
    expect(result).toEqual(data)
  })
})

describe('getNode', () => {
  it('encodes name in URL', async () => {
    mockOk({ agent: {}, containers: [] })

    await getNode('node 1')
    expect(mockFetch).toHaveBeenCalledWith('/api/v1/nodes/node%201')
  })
})

describe('getContainers', () => {
  it('passes node, page_size, and offset params', async () => {
    mockOk({ containers: [], total: 0 })

    await getContainers('node-1', 10, 5)
    const url = mockFetch.mock.calls[0][0] as string
    expect(url).toContain('node=node-1')
    expect(url).toContain('page_size=10')
    expect(url).toContain('offset=5')
  })

  it('uses defaults when no args given', async () => {
    mockOk({ containers: [], total: 0 })

    await getContainers()
    const url = mockFetch.mock.calls[0][0] as string
    expect(url).toContain('page_size=50')
    expect(url).toContain('offset=0')
  })
})

describe('getContainer', () => {
  it('calls correct URL', async () => {
    mockOk({ container_id: 'c1' })

    await getContainer('c1')
    expect(mockFetch).toHaveBeenCalledWith('/api/v1/containers/c1')
  })
})

describe('getHealth', () => {
  it('calls /healthz', async () => {
    mockOk({ status: 'ok' })

    const result = await getHealth()
    expect(mockFetch).toHaveBeenCalledWith('/healthz')
    expect(result).toEqual({ status: 'ok' })
  })
})

describe('error handling', () => {
  it('throws with status on fetch error', async () => {
    mockError(500, 'Internal Server Error')

    await expect(getNodes()).rejects.toThrow('500')
  })

  it('throws with status on 404', async () => {
    mockError(404, 'not found')

    await expect(getNode('missing')).rejects.toThrow('404')
  })
})
