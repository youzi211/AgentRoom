import { strict as assert } from 'node:assert'
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'
import { test } from 'node:test'

const frontendRoot = new URL('../', import.meta.url)
const srcRoot = new URL('../', import.meta.url)
const scannedDirectories = ['components', 'api']
const scannedRootFiles = ['App.jsx', 'routing.js']

const mojibakeMarkers = [
  '锛',
  '绠',
  '鎴',
  '浼',
  '鐨',
  '鍙',
  '鈥',
  '€',
  '�',
  '閿',
  '閵',
  '閳',
  '鐞',
  '璇',
  '悊',
]

const expectedReadableSnippets = [
  ['components/JoinScreen.jsx', ['人和 Agent 协作开会的工作台', '创建会议室', '加入会议室']],
  ['components/ChatRoom.jsx', ['会议上下文', '实时讨论', '会议控制']],
  ['components/AgentAdmin.jsx', ['管理 Agent', '角色模板', '专属知识库']],
  ['components/MessageList.jsx', ['开始一次协作会议', '系统', '成员']],
  ['App.jsx', ['创建房间失败，请稍后重试。', '加入房间失败，请稍后重试。']],
]

test('user-facing source files do not contain common mojibake markers', () => {
  const failures = []

  for (const filePath of collectSourceFiles()) {
    const source = readFileSync(filePath, 'utf8')
    for (const marker of mojibakeMarkers) {
      const index = source.indexOf(marker)
      if (index === -1) {
        continue
      }

      failures.push(`${relative(fileURLToPath(srcRoot), filePath)} contains "${marker}" near "${snippetAround(source, index)}"`)
      break
    }
  }

  assert.deepEqual(failures, [])
})

test('key product copy is stored as readable Chinese', () => {
  for (const [relativePath, snippets] of expectedReadableSnippets) {
    const source = readFileSync(new URL(relativePath, frontendRoot), 'utf8')
    for (const snippet of snippets) {
      assert.match(source, new RegExp(escapeRegExp(snippet)), `${relativePath} should contain "${snippet}"`)
    }
  }
})

function collectSourceFiles() {
  const files = scannedRootFiles.map((fileName) => fileURLToPath(new URL(fileName, srcRoot)))
  for (const directory of scannedDirectories) {
    files.push(...walk(fileURLToPath(new URL(`${directory}/`, srcRoot))))
  }
  return files.filter((filePath) => /\.(jsx|js)$/.test(filePath))
}

function walk(directory) {
  const result = []
  for (const entry of readdirSync(directory)) {
    const path = join(directory, entry)
    const stats = statSync(path)
    if (stats.isDirectory()) {
      result.push(...walk(path))
    } else {
      result.push(path)
    }
  }
  return result
}

function snippetAround(source, index) {
  return source.slice(Math.max(0, index - 12), index + 24).replace(/\s+/g, ' ')
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function fileURLToPath(url) {
  return decodeURIComponent(url.pathname).replace(/^\/([A-Za-z]:\/)/, '$1')
}
