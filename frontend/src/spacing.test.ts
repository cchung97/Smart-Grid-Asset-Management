/// <reference types="node" />
import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

// The spacing scale: 4 / 8 / 12 / 16 / 24 / 32 px (Tailwind steps 1, 2, 3, 4, 6, 8; 0 and auto are also fine).
// Every padding, margin, gap and space-x/y utility in a component must sit on it, so page gutters, panel
// padding and gaps all come from one scale. `pl-10`/`pr-10` is the one extra step: the inset that clears an
// icon inside an input (12 px icon offset + 16 px icon + 12 px). Arbitrary values (p-[13px]) are refused.
const STEPS = new Set(['0', 'px', '1', '2', '3', '4', '6', '8', 'auto'])
const ICON_INSET = new Set(['pl', 'pr'])
const UTILITY =
  /(?<![\w\-[])(?:[a-z0-9]+:)*-?(px|py|pt|pr|pb|pl|p|mx|my|mt|mr|mb|ml|m|gap-x|gap-y|gap|space-x|space-y)-(\[[^\]]+\]|[0-9.]+|px|auto)(?![\w\-.%[/])/g

function sources(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const path = join(dir, e.name)
    if (e.isDirectory()) return e.name === 'test' ? [] : sources(path)
    return /\.tsx$/.test(e.name) && !/\.test\.tsx$/.test(e.name) ? [path] : []
  })
}

describe('spacing scale', () => {
  it('uses only 4/8/12/16/24/32 px steps in components', () => {
    const off: string[] = []
    for (const file of sources('src')) {
      readFileSync(file, 'utf8').split('\n').forEach((line, i) => {
        for (const m of line.matchAll(UTILITY)) {
          const [whole, kind, value] = m
          if (STEPS.has(value) || (value === '10' && ICON_INSET.has(kind))) continue
          off.push(`${file}:${i + 1}  ${whole.trim()}`)
        }
      })
    }
    expect(off, `off-scale spacing (use 4/8/12/16/24/32 px):\n${off.join('\n')}`).toEqual([])
  })
})
