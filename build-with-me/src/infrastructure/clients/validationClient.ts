/**
 * Validation Client
 * Calls the backend validation service to verify user-filled masked code
 * Uses RPC API calls instead of CLI
 */

import * as vscode from 'vscode';
import { 
  sendApiRequest, 
  isApiStdioServerRunning, 
  startApiStdioServer 
} from '../services/apiStdioServer';

/**
 * Validation error
 */
export interface ValidationError {
  message: string;
  line?: number;
  column?: number;
  severity: 'error' | 'warning';
}

/**
 * Stage result
 */
export interface StageResult {
  stage: 'parse';
  passed: boolean;
  errors: ValidationError[];
  warnings: string[];
  suggestions: string[];
  hints: string[];
  metrics: {
    duration_ms: number;
    [key: string]: unknown;
  };
}

/**
 * Validation result
 */
export interface ValidationResult {
  passed: boolean;      // Overall validation result
  results: StageResult[];
}

/**
 * Validation parameters
 */
export interface ValidationParams {
  code: string;         // User-filled code (REQUIRED)
  filePath: string;     // REQUIRED
  sessionId: string;    // REQUIRED for session tracking
  stages?: string[];    // ["parse"] - default: all
}

/**
 * Client for calling the validation service via RPC
 */
export class ValidationClient {
  private context: vscode.ExtensionContext;
  private workspaceRoot: string;

  constructor(context: vscode.ExtensionContext) {
    this.context = context;
    this.workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath || '';
  }

  /**
   * Ensure the API server is running
   */
  private async ensureServerRunning(): Promise<void> {
    if (!isApiStdioServerRunning()) {
      const started = await startApiStdioServer(this.context);
      if (!started) {
        throw new Error('Failed to start API server for validation');
      }
    }
  }

  /**
   * Validate user-filled code
   * @param params Validation parameters
   */
  async validateCode(params: ValidationParams): Promise<ValidationResult> {
    if (!params.code) {
      throw new Error('Code is required for validation');
    }

    if (!params.filePath) {
      throw new Error('File path is required for validation');
    }

    if (!params.sessionId) {
      throw new Error('Session ID is required for validation');
    }

    await this.ensureServerRunning();

    try {
      const result = await sendApiRequest<ValidationResult>('validation.run', {
        code: params.code,
        filePath: params.filePath,
        sessionId: params.sessionId,
        stages: params.stages
      });

      return result;
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      throw new Error(`Validation failed: ${message}`);
    }
  }

  /**
   * Validate code with specific stages
   */
  async validateWithStages(
    code: string,
    filePath: string,
    sessionId: string,
    stages: ('parse')[]
  ): Promise<ValidationResult> {
    return this.validateCode({
      code,
      filePath,
      sessionId,
      stages
    });
  }

  /**
   * Quick parse-only validation (no LLM)
   */
  async quickValidate(code: string, filePath: string): Promise<ValidationResult> {
    return this.validateCode({
      code,
      filePath,
      sessionId: 'quick-check', // Dummy session ID for parse-only
      stages: ['parse']
    });
  }
}
