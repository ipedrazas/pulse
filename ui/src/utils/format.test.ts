import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { formatUptime, formatLastSeen, statusColor } from './format'

describe('formatUptime', () => {
  it('returns 0s for zero', () => {
    expect(formatUptime(0)).toBe('0s')
  })

  it('returns seconds for < 60', () => {
    expect(formatUptime(30)).toBe('30s')
    expect(formatUptime(59)).toBe('59s')
  })

  it('returns minutes for 60-3599', () => {
    expect(formatUptime(60)).toBe('1m')
    expect(formatUptime(300)).toBe('5m')
  })

  it('returns hours for 3600-86399', () => {
    expect(formatUptime(3600)).toBe('1h')
    expect(formatUptime(7200)).toBe('2h')
  })

  it('returns days for >= 86400', () => {
    expect(formatUptime(86400)).toBe('1d')
    expect(formatUptime(172800)).toBe('2d')
  })
})

describe('formatLastSeen', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-03-06T12:00:00Z'))
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('returns Never for undefined', () => {
    expect(formatLastSeen(undefined)).toBe('Never')
  })

  it('returns seconds ago', () => {
    expect(formatLastSeen('2026-03-06T11:59:30Z')).toBe('30s ago')
  })

  it('returns minutes ago', () => {
    expect(formatLastSeen('2026-03-06T11:55:00Z')).toBe('5m ago')
  })

  it('returns hours ago', () => {
    expect(formatLastSeen('2026-03-06T10:00:00Z')).toBe('2h ago')
  })

  it('returns date for > 24h ago', () => {
    const result = formatLastSeen('2026-03-04T12:00:00Z')
    // Should return a locale date string, not "Xs ago"
    expect(result).not.toContain('ago')
  })
})

describe('statusColor', () => {
  it('returns green for running', () => {
    expect(statusColor('running')).toBe('text-green-400')
  })

  it('returns red for exited', () => {
    expect(statusColor('exited')).toBe('text-red-400')
  })

  it('returns yellow for paused', () => {
    expect(statusColor('paused')).toBe('text-yellow-400')
  })

  it('returns blue for created', () => {
    expect(statusColor('created')).toBe('text-blue-400')
  })

  it('returns gray for unknown status', () => {
    expect(statusColor('unknown')).toBe('text-gray-400')
  })

  it('is case-insensitive', () => {
    expect(statusColor('Running')).toBe('text-green-400')
    expect(statusColor('EXITED')).toBe('text-red-400')
  })
})
