const fs = require('fs');
const path = require('path');

const root = path.resolve(__dirname, '..');
const releaseType = (process.argv[2] || 'patch').toLowerCase();
const versionPattern = /^\d+\.\d+\.\d+$/;

function readJson(file) {
  return JSON.parse(fs.readFileSync(path.join(root, file), 'utf8'));
}

function writeJson(file, data) {
  fs.writeFileSync(path.join(root, file), `${JSON.stringify(data, null, 2)}\n`);
}

function bumpVersion(version) {
  if (!versionPattern.test(version)) throw new Error(`Unsupported version format: ${version}`);
  const parts = version.split('.').map(Number);
  if (releaseType === 'major') {
    parts[0] += 1;
    parts[1] = 0;
    parts[2] = 0;
  } else if (releaseType === 'minor') {
    parts[1] += 1;
    parts[2] = 0;
  } else if (releaseType === 'patch') {
    parts[2] += 1;
  } else {
    throw new Error(`Unsupported release type: ${releaseType}`);
  }
  return parts.join('.');
}

function updateGoVersion(version) {
  const file = path.join(root, 'main.go');
  const text = fs.readFileSync(file, 'utf8');
  fs.writeFileSync(file, text.replace(/const appVersion = "[^"]+"/, `const appVersion = "${version}"`));
}

function updateBuildConfig(version) {
  const file = path.join(root, 'build', 'config.yml');
  if (!fs.existsSync(file)) return;
  const text = fs.readFileSync(file, 'utf8');
  fs.writeFileSync(file, text.replace(/productVersion:\s*[^\r\n]+/, `productVersion: ${version}`));
}

function updateWindowsResources(version) {
  const info = readJson('build/windows/info.json');
  info.fixed = info.fixed || {};
  info.fixed.file_version = version;
  info.fixed.product_version = version;
  Object.values(info.info || {}).forEach(values => {
    values.FileVersion = version;
    values.ProductVersion = version;
  });
  writeJson('build/windows/info.json', info);
  const manifestFile = path.join(root, 'build', 'windows', 'manifest.xml');
  const manifest = fs.readFileSync(manifestFile, 'utf8');
  fs.writeFileSync(manifestFile, manifest.replace(/(<assemblyIdentity version=")[^"]+(")/, `$1${version}.0$2`));
}

const pkg = readJson('package.json');
const nextVersion = releaseType === 'set'
  ? (process.argv[3] || '').replace(/^v/i, '')
  : bumpVersion(pkg.version);
if (!versionPattern.test(nextVersion)) throw new Error(`Unsupported version format: ${nextVersion}`);
pkg.version = nextVersion;
writeJson('package.json', pkg);

updateGoVersion(nextVersion);
updateBuildConfig(nextVersion);
updateWindowsResources(nextVersion);

console.log(nextVersion);
