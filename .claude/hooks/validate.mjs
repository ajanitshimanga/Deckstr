#!/usr/bin/env node
// Cross-platform validation hook (runs on Stop).
// Validates the project (build + test) only when source files have changed
// since the last successful validation. This avoids blocking pure-chat turns
// with unnecessary work.
//
// Detection strategy: a stamp file is touched after a successful run. On the
// next invocation, walk tracked source files and check if any are newer than
// the stamp. If none, skip. If any, validate and re-stamp on success.

import { spawnSync } from 'node:child_process'
import { readdirSync, statSync, existsSync, utimesSync, closeSync, openSync } from 'node:fs'
import { join, resolve, relative, sep } from 'node:path'

const PROJECT_DIR = process.env.CLAUDE_PROJECT_DIR
  ? resolve(process.env.CLAUDE_PROJECT_DIR)
  : resolve(new URL('../..', import.meta.url).pathname.replace(/^\/(\w:)/, '$1'))

process.chdir(PROJECT_DIR)

const STAMP = join(PROJECT_DIR, '.claude', '.last-validation')

const EXCLUDE_DIRS = new Set([
  'node_modules',
  'wailsjs',
  'dist',
  'bin',
  '.git',
  '.claude',
  'build',
])

const SOURCE_EXT = new Set(['.go', '.ts', '.tsx'])

function log(msg) {
  console.log(`[validate] ${msg}`)
}

function err(msg) {
  // stderr is surfaced to Claude when we exit 2 (blocking).
  console.error(`[validate] ${msg}`)
}

function anySourceNewerThan(mtime) {
  const stack = [PROJECT_DIR]
  while (stack.length) {
    const dir = stack.pop()
    let entries
    try {
      entries = readdirSync(dir, { withFileTypes: true })
    } catch {
      continue
    }
    for (const entry of entries) {
      const full = join(dir, entry.name)
      if (entry.isDirectory()) {
        if (EXCLUDE_DIRS.has(entry.name)) continue
        stack.push(full)
      } else if (entry.isFile()) {
        const dot = entry.name.lastIndexOf('.')
        if (dot < 0) continue
        const ext = entry.name.slice(dot)
        if (!SOURCE_EXT.has(ext)) continue
        try {
          const st = statSync(full)
          if (st.mtimeMs > mtime) return relative(PROJECT_DIR, full)
        } catch {
          // ignore
        }
      }
    }
  }
  return null
}

function run(cmd, args, opts = {}) {
  // On Windows, tools like `npm` are `npm.cmd` shims that require a shell to
  // resolve. Pass the full command line as a single string so Node doesn't
  // trip the DEP0190 warning about mixing shell:true with an args array.
  if (process.platform === 'win32') {
    const cmdline = [cmd, ...args].map((a) => (/\s/.test(a) ? `"${a}"` : a)).join(' ')
    const res = spawnSync(cmdline, { stdio: 'inherit', shell: true, ...opts })
    return res.status === 0
  }
  const res = spawnSync(cmd, args, { stdio: 'inherit', ...opts })
  return res.status === 0
}

function touch(path) {
  const now = new Date()
  try {
    utimesSync(path, now, now)
  } catch {
    // Create if it doesn't exist
    const fd = openSync(path, 'w')
    closeSync(fd)
  }
}

// Skip if nothing changed since the last successful run
if (existsSync(STAMP)) {
  const stampMtime = statSync(STAMP).mtimeMs
  const changed = anySourceNewerThan(stampMtime)
  if (!changed) {
    log('No source changes since last run - skipping')
    process.exit(0)
  }
  log(`Source change detected (${changed}) - running build + tests`)
} else {
  log('No prior validation stamp - running build + tests')
}

const steps = [
  ['go', ['build', './...']],
  ['go', ['test', './...']],
  ['npm', ['run', 'build', '--prefix', 'frontend']],
]

for (const [cmd, args] of steps) {
  log(`> ${cmd} ${args.join(' ')}`)
  if (!run(cmd, args)) {
    err(`FAILED: ${cmd} ${args.join(' ')}`)
    err('Stop blocked: fix the failure above before ending the turn.')
    // Exit code 2 tells Claude Code this is a *blocking* hook error
    // (exit 1 is treated as a non-blocking warning and lets Stop proceed).
    process.exit(2)
  }
}

touch(STAMP)
log('All checks passed')
