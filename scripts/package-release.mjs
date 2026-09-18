import { execFileSync } from 'node:child_process'
import { rmSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const archive = fileURLToPath(new URL('../simpledms-integration.zip', import.meta.url))
const dist = fileURLToPath(new URL('../dist/', import.meta.url))

rmSync(archive, { force: true })
execFileSync('zip', ['-qr', archive, '.'], { cwd: dist, stdio: 'inherit' })
