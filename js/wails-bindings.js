const { spawnSync } = require('child_process');
const env = { ...process.env, GOFLAGS: [process.env.GOFLAGS, '-mod=mod'].filter(Boolean).join(' ') };
const result = spawnSync('wails3', ['generate', 'bindings', '-b', '-clean'], { env, stdio: 'inherit' });
process.exit(result.status ?? 1);
