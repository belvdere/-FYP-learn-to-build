import * as vscode from 'vscode';
import * as path from 'path';

export interface LSPReadiness {
  ready: boolean;
  reason?: string;
}

export interface LanguageLSPPlugin {
  name: string;
  fileExtensions: string[];
  vsCodeExtensionId: string;
  checkReady(): Promise<LSPReadiness>;
}

async function checkTypeScriptServerReady(): Promise<LSPReadiness> {
  const ext = vscode.extensions.getExtension('vscode.typescript-language-features');
  if (!ext) {
    return { ready: false, reason: 'TypeScript language features extension not available' };
  }
  if (!ext.isActive) {
    try {
      await ext.activate();
    } catch {
      return { ready: false, reason: 'TypeScript language features not activated' };
    }
  }
  // Probe whether tsserver has finished initializing by requesting document symbols.
  // Returns undefined (not []) when the provider hasn't registered yet.
  const tsFiles = await vscode.workspace.findFiles(
    '**/*.{ts,tsx,js,jsx}',
    '**/{.git,.fyp,node_modules,target,build,dist,out,coverage,.venv,venv,__pycache__}/**',
    1
  );
  if (tsFiles.length > 0) {
    try {
      const symbols = await vscode.commands.executeCommand<vscode.DocumentSymbol[] | undefined>(
        'vscode.executeDocumentSymbolProvider',
        tsFiles[0]
      );
      if (symbols === undefined) {
        return { ready: false, reason: 'TypeScript language server still initializing...' };
      }
    } catch {
      return { ready: false, reason: 'TypeScript language server still initializing...' };
    }
  }
  return { ready: true };
}

export const languageLSPPlugins: LanguageLSPPlugin[] = [
  {
    name: 'Java',
    fileExtensions: ['.java'],
    vsCodeExtensionId: 'redhat.java',
    async checkReady(): Promise<LSPReadiness> {
      const ext = vscode.extensions.getExtension('redhat.java');
      if (!ext) {
        return { ready: false, reason: 'Java extension (redhat.java) not installed' };
      }
      if (!ext.isActive) {
        try {
          await ext.activate();
        } catch {
          return { ready: false, reason: 'Java extension not activated' };
        }
      }
      try {
        await vscode.commands.executeCommand('java.project.getAll');
        return { ready: true };
      } catch {
        return { ready: false, reason: 'Java language server still initializing...' };
      }
    },
  },
  {
    name: 'Python',
    fileExtensions: ['.py'],
    vsCodeExtensionId: 'ms-python.vscode-pylance',
    async checkReady(): Promise<LSPReadiness> {
      const pythonExt = vscode.extensions.getExtension('ms-python.python');
      if (!pythonExt) {
        return { ready: false, reason: 'Python extension (ms-python.python) not installed' };
      }

      const pylanceExt = vscode.extensions.getExtension('ms-python.vscode-pylance');
      if (!pylanceExt) {
        return { ready: false, reason: 'Pylance extension (ms-python.vscode-pylance) not installed' };
      }

      if (!pythonExt.isActive) {
        try {
          await pythonExt.activate();
        } catch {
          return { ready: false, reason: 'Python extension not activated' };
        }
      }

      if (!pylanceExt.isActive) {
        try {
          await pylanceExt.activate();
        } catch {
          return { ready: false, reason: 'Pylance extension not activated' };
        }
      }

      // Probe whether Pylance's language server has finished initializing by
      // requesting document symbols for a Python file. Pylance registers its
      // document symbol provider only after the server is ready; if the call
      // returns undefined the provider hasn't started yet.
      const pyFiles = await vscode.workspace.findFiles(
        '**/*.py',
        '**/{.git,.fyp,node_modules,target,build,dist,out,coverage,.venv,venv,__pycache__}/**',
        1
      );
      if (pyFiles.length > 0) {
        try {
          const symbols = await vscode.commands.executeCommand<vscode.DocumentSymbol[] | undefined>(
            'vscode.executeDocumentSymbolProvider',
            pyFiles[0]
          );
          if (symbols === undefined) {
            return { ready: false, reason: 'Pylance language server still initializing...' };
          }
        } catch {
          return { ready: false, reason: 'Pylance language server still initializing...' };
        }
      }

      return { ready: true };
    },
  },
  {
    name: 'Go',
    fileExtensions: ['.go'],
    vsCodeExtensionId: 'golang.go',
    async checkReady(): Promise<LSPReadiness> {
      const ext = vscode.extensions.getExtension('golang.go');
      if (!ext) {
        return { ready: false, reason: 'Go extension (golang.go) not installed' };
      }
      if (!ext.isActive) {
        try {
          await ext.activate();
        } catch {
          return { ready: false, reason: 'Go extension not activated' };
        }
      }
      // Probe whether gopls has finished initializing by requesting document
      // symbols for a Go file. Returns undefined before the server is ready.
      const goFiles = await vscode.workspace.findFiles(
        '**/*.go',
        '**/{.git,.fyp,node_modules,vendor}/**',
        1
      );
      if (goFiles.length > 0) {
        try {
          const symbols = await vscode.commands.executeCommand<vscode.DocumentSymbol[] | undefined>(
            'vscode.executeDocumentSymbolProvider',
            goFiles[0]
          );
          if (symbols === undefined) {
            return { ready: false, reason: 'gopls still initializing...' };
          }
        } catch {
          return { ready: false, reason: 'gopls still initializing...' };
        }
      }
      return { ready: true };
    },
  },
  {
    name: 'TypeScript',
    fileExtensions: ['.ts', '.tsx', '.mts', '.cts'],
    vsCodeExtensionId: 'vscode.typescript-language-features',
    async checkReady(): Promise<LSPReadiness> {
      return checkTypeScriptServerReady();
    },
  },
  {
    name: 'JavaScript',
    fileExtensions: ['.js', '.jsx', '.mjs', '.cjs'],
    vsCodeExtensionId: 'vscode.typescript-language-features',
    async checkReady(): Promise<LSPReadiness> {
      return checkTypeScriptServerReady();
    },
  },
];

const WORKSPACE_SCAN_EXCLUDE =
  '**/{.git,.fyp,node_modules,target,build,dist,out,coverage,.venv,venv,__pycache__}/**';

function toExtensionGlob(fileExtensions: string[]): string {
  const exts = fileExtensions
    .map((ext) => ext.trim().replace(/^\./, ''))
    .filter((ext) => ext.length > 0);
  if (exts.length === 0) {
    return '';
  }
  if (exts.length === 1) {
    return `**/*.${exts[0]}`;
  }
  return `**/*.{${exts.join(',')}}`;
}

async function workspaceHasAnyFiles(fileExtensions: string[]): Promise<boolean> {
  const glob = toExtensionGlob(fileExtensions);
  if (!glob) {
    return false;
  }
  if (!vscode.workspace.workspaceFolders || vscode.workspace.workspaceFolders.length === 0) {
    return false;
  }

  for (const folder of vscode.workspace.workspaceFolders) {
    const matches = await vscode.workspace.findFiles(
      new vscode.RelativePattern(folder, glob),
      WORKSPACE_SCAN_EXCLUDE,
      1
    );
    if (matches.length > 0) {
      return true;
    }
  }
  return false;
}

export async function getRequiredPluginsForWorkspace(): Promise<LanguageLSPPlugin[]> {
  if (!vscode.workspace.workspaceFolders || vscode.workspace.workspaceFolders.length === 0) {
    return [];
  }

  const checks = await Promise.all(
    languageLSPPlugins.map(async (plugin) => ({
      plugin,
      required: await workspaceHasAnyFiles(plugin.fileExtensions),
    }))
  );
  const needed = checks.filter((item) => item.required).map((item) => item.plugin);

  // Preserve Java-first behavior when no supported source files are discovered yet.
  return needed.length > 0 ? needed : languageLSPPlugins.filter((p) => p.name === 'Java');
}

export function getPluginForFilePath(filePath: string): LanguageLSPPlugin | undefined {
  const ext = path.extname(filePath).toLowerCase();
  return languageLSPPlugins.find((plugin) =>
    plugin.fileExtensions.some((supported) => supported.toLowerCase() === ext)
  );
}
