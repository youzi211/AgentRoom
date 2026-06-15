import { strict as assert } from 'node:assert'
import { test } from 'node:test'
import { calculateResizeSize } from './resizeSizing.js'

test('calculates a resized panel from pointer delta', () => {
  assert.equal(
    calculateResizeSize({
      currentPosition: 330,
      maxSize: 400,
      minSize: 200,
      startPosition: 300,
      startSize: 270,
    }),
    300,
  )
})

test('can invert pointer delta for a panel on the far side of the handle', () => {
  assert.equal(
    calculateResizeSize({
      currentPosition: 860,
      invertDelta: true,
      maxSize: 450,
      minSize: 200,
      startPosition: 900,
      startSize: 320,
    }),
    360,
  )
})
