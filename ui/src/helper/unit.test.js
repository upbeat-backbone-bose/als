import { describe, it, expect } from 'vitest'
import { formatBytes } from '@/helper/unit'

describe('formatBytes', () => {
  it('returns 0 Bytes for zero', () => {
    expect(formatBytes(0)).toBe('0 Bytes')
  })

  it('formats bytes correctly', () => {
    expect(formatBytes(1)).toBe('1 Bytes')
    expect(formatBytes(1024)).toBe('1 KB')
    expect(formatBytes(1048576)).toBe('1 MB')
    expect(formatBytes(1073741824)).toBe('1 GB')
  })

  it('handles decimal precision', () => {
    expect(formatBytes(1536, 2)).toBe('1.5 KB')
    expect(formatBytes(1536, 0)).toBe('2 KB')
  })

  it('formats bandwidth with Bps units', () => {
    expect(formatBytes(125, 2, true)).toBe('1000 Bps')
    expect(formatBytes(125000, 2, true)).toBe('1000 Kbps')
    expect(formatBytes(1000, 2, true)).toBe('8000 Bps')
    expect(formatBytes(125000000, 2, true)).toBe('1000 Mbps')
  })

  it('handles negative decimals by treating as 0', () => {
    expect(formatBytes(1536, -1)).toBe('2 KB')
  })
})
