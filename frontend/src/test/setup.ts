import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'

afterEach(() => {
  cleanup()
})

// jsdom does not implement scrollIntoView; the tree calls it for the selected node.
Element.prototype.scrollIntoView = () => {}
