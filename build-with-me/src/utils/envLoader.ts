import * as fs from 'fs';
import * as path from 'path';

/**
 * Load environment variables from .env file
 * 
 * Search order:
 * 1. .fyp/.env in workspace (recommended - keeps secrets with FYP data)
 * 2. .env in workspace root
 * 3. .env one level up from workspace
 */
export function loadEnvFile(workspaceRoot: string): NodeJS.ProcessEnv {
  const env = { ...process.env };
  
  // Search locations in priority order
  const searchPaths = [
    path.join(workspaceRoot, '.fyp', '.env'),  // Recommended: inside .fyp folder
    path.join(workspaceRoot, '.env'),           // Workspace root
    path.join(path.resolve(workspaceRoot, '..'), '.env'),  // One level up
  ];
  
  let envPath: string | null = null;
  for (const p of searchPaths) {
    if (fs.existsSync(p)) {
      envPath = p;
      break;
    }
  }
  
  if (envPath) {
    console.log(`[FYP] Loading environment from: ${envPath}`);
    const content = fs.readFileSync(envPath, 'utf-8');
    const lines = content.split('\n');
    
    for (const line of lines) {
      const trimmed = line.trim();
      if (trimmed && !trimmed.startsWith('#')) {
        const [key, ...valueParts] = trimmed.split('=');
        if (key && valueParts.length > 0) {
          env[key.trim()] = valueParts.join('=').trim();
        }
      }
    }
  }
  
  return env;
}

