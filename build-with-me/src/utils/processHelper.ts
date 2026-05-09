import { spawn, ChildProcess, SpawnOptions } from 'child_process';
import * as path from 'path';
import * as fs from 'fs';
import { loadEnvFile } from './envLoader';

/**
 * Map process.platform and process.arch to bundled binary directory name.
 * Matches layout: bin/darwin-arm64/, bin/darwin-x64/, bin/linux-x64/, bin/win32-x64/
 */
function getPlatformDir(): string {
  const p = process.platform;
  const a = process.arch;
  if (p === 'darwin') {
    return a === 'arm64' ? 'darwin-arm64' : 'darwin-x64';
  }
  if (p === 'linux') {
    return a === 'arm64' ? 'linux-arm64' : 'linux-x64';
  }
  if (p === 'win32') {
    return a === 'arm64' ? 'win32-arm64' : 'win32-x64';
  }
  return `${p}-${a}`;
}

/**
 * Get the path to the fypd binary.
 * 1. Primary: bundled binary inside extension (bin/<platform>/fypd) - for packaged .vsix
 * 2. Fallback: sibling server/bin/fypd - for Extension Development Host (F5 in monorepo)
 */
export function getFypdBinaryPath(extensionPath: string): string | null {
  const exe = process.platform === 'win32' ? 'fypd.exe' : 'fypd';
  const realExtensionPath = fs.realpathSync(extensionPath);

  // Primary: bundled binary (for packaged extension)
  const platformDir = getPlatformDir();
  const bundledBin = path.join(realExtensionPath, 'bin', platformDir, exe);
  if (fs.existsSync(bundledBin)) {
    return bundledBin;
  }

  // Fallback: development layout (monorepo: extensionPath is build-with-me/, server/ is sibling)
  const devRoot = path.resolve(realExtensionPath, '..');
  const devBin = path.join(devRoot, 'server', 'bin', exe);
  if (fs.existsSync(devBin)) {
    return devBin;
  }

  return null;
}

/**
 * Spawn fypd process with environment variables loaded
 */
export function spawnFypd(
  binary: string,
  args: string[],
  workspaceRoot: string,
  options?: SpawnOptions
): ChildProcess {
  const env = loadEnvFile(workspaceRoot);
  
  return spawn(binary, args, {
    ...options,
    cwd: workspaceRoot,
    env
  });
}

/**
 * Execute fypd command and return stdout
 */
export function execFypd(
  binary: string,
  args: string[],
  workspaceRoot: string
): Promise<string> {
  return new Promise((resolve, reject) => {
    const proc = spawnFypd(binary, args, workspaceRoot);
    
    let stdout = '';
    let stderr = '';
    
    proc.stdout?.on('data', (data) => {
      stdout += data.toString();
    });
    
    proc.stderr?.on('data', (data) => {
      stderr += data.toString();
    });
    
    proc.on('close', (code) => {
      if (code === 0) {
        resolve(stdout);
      } else {
        reject(new Error(`fypd exited with code ${code}: ${stderr}`));
      }
    });
    
    proc.on('error', (err) => {
      reject(err);
    });
  });
}

