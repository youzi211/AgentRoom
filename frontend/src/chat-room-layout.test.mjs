import { strict as assert } from 'node:assert'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

const css = readFileSync(new URL('./chat-room.css', import.meta.url), 'utf8')

function ruleBlock(selector) {
  const escapedSelector = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const match = css.match(new RegExp(`${escapedSelector}\\s*\\{([^}]*)\\}`, 'm'))
  return match?.[1] ?? ''
}

test('left meeting context panel remains vertically scrollable', () => {
  const block = ruleBlock('.chat-context-panel')

  assert.match(block, /overflow-y:\s*auto;/)
  assert.doesNotMatch(block, /overflow:\s*hidden;/)
})
