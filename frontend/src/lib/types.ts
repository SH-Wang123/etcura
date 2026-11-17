// Wire types mirrored from the Go models (see internal/model). The generated
// wailsjs bindings return class instances that are structurally identical to
// these interfaces; api.ts casts at that single boundary.

export type ConnectionState = 'disconnected' | 'connecting' | 'connected' | 'error';

export type ErrorCode =
  | 'CONNECTION_FAILED'
  | 'NOT_CONNECTED'
  | 'TIMEOUT'
  | 'CANCELLED'
  | 'KEY_NOT_FOUND'
  | 'KEY_ALREADY_EXISTS'
  | 'REVISION_CONFLICT'
  | 'LEASE_NOT_FOUND'
  | 'PERMISSION_DENIED'
  | 'TLS_ERROR'
  | 'INVALID_ARGUMENT'
  | 'QUOTA_EXCEEDED'
  | 'CONFIG_ERROR'
  | 'UNKNOWN';

export interface AppErrorShape {
  code: ErrorCode | string;
  message: string;
  detail?: string;
}

export interface TLSConfig {
  enabled: boolean;
  caFile: string;
  certFile: string;
  keyFile: string;
  skipVerify: boolean;
}

export interface ConnectionConfig {
  id: string;
  name: string;
  endpoints: string[];
  username: string;
  dialTimeoutMs: number;
  requestTimeoutMs: number;
  readOnly: boolean;
  tls: TLSConfig;
}

export interface ConnectionSummary {
  id: string;
  name: string;
  endpoints: string[];
  hasPassword: boolean;
}

export interface StatusInfo {
  state: ConnectionState;
  profileId: string;
  session: string;
  readOnly: boolean;
  name: string;
  endpoints: string;
  header: number;
  etcdVersion: string;
  message: string;
}

export interface ProtoFileInfo {
  name: string;
  package: string;
  messageCount: number;
  enumCount: number;
}

export interface ProtoImportSummary {
  files: ProtoFileInfo[];
  messages: number;
  enums: number;
}

export interface ProtoCandidate {
  messageName: string;
  score: number;
  setFieldNums: number[];
  unknownBytes: number;
  json: string;
}

export interface ProtoMatchResult {
  wireTagNums: number[];
  scanned: number;
  valueTooLarge: boolean;
  candidates: ProtoCandidate[];
}

export type ScriptLanguage = 'python' | 'node' | 'ruby' | 'shell';

export interface ScriptConfig {
  language: ScriptLanguage;
  /** Variable the raw value bytes are bound to ("" = not bound). */
  inputVar: string;
  /** Variable read back as the displayed result ("" = use stdout). */
  outputVar: string;
  body: string;
}

export interface ScriptInfo {
  configured: boolean;
  language: ScriptLanguage;
  inputVar: string;
  outputVar: string;
  interpreter: string;
  available: boolean;
  warning?: string;
}

export interface ScriptOutput {
  output: string;
  stderr: string;
  exitCode: number;
  durationMs: number;
  truncated: boolean;
}

export interface StatusResponse {
  ok: boolean;
  header: number;
  etcdVersion: string;
  endpoint: string;
}

export interface LevelRow {
  type: 'segment' | 'leaf';
  name: string;
  path: string;
  modRevision: number;
  version: number;
  lease: number;
}

export interface LevelPage {
  prefix: string;
  cursor: string;
  nextCursor: string;
  hasMore: boolean;
  rows: LevelRow[];
}

export interface KvDetail {
  key: string;
  valueBase64: string;
  isUTF8: boolean;
  sizeBytes: number;
  createRevision: number;
  modRevision: number;
  version: number;
  lease: number;
}

export const DEFAULT_DIAL_TIMEOUT_MS = 5000;
export const DEFAULT_REQUEST_TIMEOUT_MS = 10000;

export function emptyTLS(): TLSConfig {
  return { enabled: false, caFile: '', certFile: '', keyFile: '', skipVerify: false };
}

export function newConnectionForm(): ConnectionConfig {
  return {
    id: '',
    name: '',
    endpoints: ['127.0.0.1:2379'],
    username: '',
    dialTimeoutMs: DEFAULT_DIAL_TIMEOUT_MS,
    requestTimeoutMs: DEFAULT_REQUEST_TIMEOUT_MS,
    readOnly: false,
    tls: emptyTLS(),
  };
}
