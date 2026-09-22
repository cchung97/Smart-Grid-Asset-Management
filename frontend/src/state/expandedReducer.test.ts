import { describe, expect, it } from 'vitest'
import { expandedReducer, type ExpandAction } from './expandedReducer'

describe('expandedReducer', () => {
  const start = new Set(['a'])
  it.each<[string, ExpandAction, string[]]>([
    ['toggle adds', { type: 'toggle', id: 'b' }, ['a', 'b']],
    ['toggle removes', { type: 'toggle', id: 'a' }, []],
    ['expand is idempotent', { type: 'expand', id: 'a' }, ['a']],
    ['collapse removes', { type: 'collapse', id: 'a' }, []],
    ['expandMany adds all', { type: 'expandMany', ids: ['b', 'c'] }, ['a', 'b', 'c']],
    ['collapseAll empties the set', { type: 'collapseAll' }, []],
  ])('%s', (_name, action, want) => {
    expect([...expandedReducer(start, action)].sort()).toEqual(want)
  })

  it('does not mutate its input, and returns the same set when nothing changes', () => {
    expandedReducer(start, { type: 'expand', id: 'z' })
    expect([...start]).toEqual(['a'])
    expect(expandedReducer(start, { type: 'expandMany', ids: ['a'] })).toBe(start)
    const empty = new Set<string>()
    expect(expandedReducer(empty, { type: 'collapseAll' })).toBe(empty)
  })
})
