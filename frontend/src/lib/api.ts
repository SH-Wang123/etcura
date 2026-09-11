// Typed facade over the Wails-generated bindings. The generated functions in
// ../../wailsjs/go/main/App return generated model classes; this module casts
// them once into the plain interfaces from types.ts so the rest of the app is
// strictly typed and never touches window.go directly.

import {
  Connect as _connect,
  CreateKey as _createKey,
  DeleteConnection as _deleteConnection,
  DeleteKey as _deleteKey,
  Disconnect as _disconnect,
  GetConnection as _getConnection,
  GetKey as _getKey,
  GetStatus as _getStatus,
  ImportProtos as _importProtos,
  ListConnections as _listConnections,
  ListProtos as _listProtos,
  ListLevel as _listLevel,
  MatchProto as _matchProto,
  SaveConnection as _saveConnection,
  SaveKey as _saveKey,
  TestConnection as _testConnection,
  ClearProtos as _clearProtos,
  SetScript as _setScript,
  ClearScript as _clearScript,
  GetScript as _getScript,
  RunScript as _runScript,
} from '../../wailsjs/go/main/App';
import type {
  AppErrorShape,
  ConnectionConfig,
  ConnectionSummary,
  KvDetail,
  ProtoFileInfo,
  ProtoMatchResult,
  LevelPage,
  ScriptConfig,
  ScriptInfo,
  ScriptOutput,
  StatusInfo,
  StatusResponse,
} from './types';
import { has, tr } from './i18n';

function bindings(): NonNullable<typeof window.go> {
  const g = (window as unknown as { go?: typeof window.go }).go;
  if (!g || !g.main || !g.main.App) {
    throw new Error(
      'Wails runtime is not available. Run the app with `wails dev` or start the packaged binary.',
    );
  }
  return g;
}

// normalizeError turns a rejected binding promise into a typed AppErrorShape.
// Depending on the host (WebView vs dev-browser bridge) Wails rejects with
// either a plain string or an Error whose message carries the JSON encoding
// of the Go AppError — both forms are parsed here.
export function normalizeError(e: unknown): AppErrorShape {
  const raw = typeof e === 'string' ? e : e instanceof Error ? e.message : String(e);
  if (raw.startsWith('{')) {
    try {
      const parsed = JSON.parse(raw) as Partial<AppErrorShape>;
      if (parsed && typeof parsed.code === 'string') {
        return { code: parsed.code, message: parsed.message ?? 'Unknown error', detail: parsed.detail };
      }
    } catch {
      /* not JSON — fall through */
    }
  }
  return { code: 'UNKNOWN', message: raw };
}

// localizeError maps the error code to a translated headline in the active
// locale. The backend's own message is preserved in `detail` whenever it was
// more specific than the translation, so actionable guidance (e.g. which proto
// import failed) is never lost.
export function localizeError(e: unknown): AppErrorShape {
  const shape = normalizeError(e);
  const key = `error.${shape.code}`;
  if (!has(key)) {
    return shape;
  }
  const localized = tr(key);
  const backendSpecific = shape.message && shape.message !== localized;
  const detail = backendSpecific
    ? shape.detail
      ? `${shape.message}\n\n${shape.detail}`
      : shape.message
    : shape.detail;
  return { ...shape, message: localized, detail };
}

export const api = {
  listConnections(): Promise<ConnectionSummary[]> {
    bindings();
    return _listConnections() as Promise<ConnectionSummary[]>;
  },
  saveConnection(cfg: ConnectionConfig, password: string, remember: boolean): Promise<ConnectionConfig> {
    bindings();
    return _saveConnection(cfg as unknown as never, password, remember) as Promise<ConnectionConfig>;
  },
  deleteConnection(id: string): Promise<void> {
    bindings();
    return _deleteConnection(id);
  },
  getConnection(id: string): Promise<ConnectionConfig> {
    bindings();
    return _getConnection(id) as Promise<ConnectionConfig>;
  },
  testConnection(cfg: ConnectionConfig, password: string): Promise<StatusResponse> {
    bindings();
    return _testConnection(cfg as unknown as never, password) as Promise<StatusResponse>;
  },
  connect(id: string): Promise<StatusInfo> {
    bindings();
    return _connect(id) as Promise<StatusInfo>;
  },
  disconnect(): Promise<StatusInfo> {
    bindings();
    return _disconnect() as Promise<StatusInfo>;
  },
  getStatus(): Promise<StatusInfo> {
    bindings();
    return _getStatus() as Promise<StatusInfo>;
  },
  listLevel(prefix: string, cursor: string, pageSize: number): Promise<LevelPage> {
    bindings();
    return _listLevel(prefix, cursor, pageSize) as Promise<LevelPage>;
  },
  importProtos(paths: string[]): Promise<ProtoFileInfo[]> {
    bindings();
    return _importProtos(paths) as Promise<ProtoFileInfo[]>;
  },
  listProtos(): Promise<ProtoFileInfo[]> {
    bindings();
    return _listProtos() as Promise<ProtoFileInfo[]>;
  },
  matchProto(valueBase64: string): Promise<ProtoMatchResult> {
    bindings();
    return _matchProto(valueBase64) as Promise<ProtoMatchResult>;
  },
  clearProtos(): Promise<void> {
    bindings();
    return _clearProtos();
  },
  setScript(cfg: ScriptConfig): Promise<ScriptInfo> {
    bindings();
    return _setScript(cfg as unknown as never) as unknown as Promise<ScriptInfo>;
  },
  clearScript(): Promise<void> {
    bindings();
    return _clearScript();
  },
  getScript(): Promise<ScriptInfo> {
    bindings();
    return _getScript() as unknown as Promise<ScriptInfo>;
  },
  runScript(valueBase64: string): Promise<ScriptOutput> {
    bindings();
    return _runScript(valueBase64) as Promise<ScriptOutput>;
  },
  getKey(key: string): Promise<KvDetail> {
    bindings();
    return _getKey(key) as Promise<KvDetail>;
  },
  createKey(key: string, valueBase64: string, leaseID: string): Promise<KvDetail> {
    bindings();
    return _createKey(key, valueBase64, leaseID) as Promise<KvDetail>;
  },
  saveKey(key: string, valueBase64: string, expectedModRevision: number): Promise<KvDetail> {
    bindings();
    return _saveKey(key, valueBase64, expectedModRevision) as Promise<KvDetail>;
  },
  deleteKey(key: string): Promise<void> {
    bindings();
    return _deleteKey(key);
  },
};
