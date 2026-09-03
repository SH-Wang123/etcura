// Lightweight i18n: two type-checked dictionaries, a persisted locale store,
// a reactive translator for templates ($t) and an imperative one for stores
// (tr). No external library — the app has ~100 strings.

import { derived, get, writable } from 'svelte/store';
import type { Readable } from 'svelte/store';

export type Locale = 'en' | 'zh-CN';

const en = {
  // toolbar
  'toolbar.noConnection': 'No connection',
  'toolbar.connectingSuffix': '(connecting…)',
  'toolbar.checkmark': ' ✓',
  'toolbar.disconnect': 'Disconnect',
  'placeholder.prefix': 'Base prefix, e.g. / or /registry/services/',
  'placeholder.exact': 'Exact key, press Enter to look up',
  'search.prefix': 'Prefix',
  'search.exact': 'Exact key',
  'tip.searchMode': 'Search mode',
  'tip.connection': 'Active connection',
  'btn.protos': 'Protos',
  'tip.protos': 'Import .proto definitions for protobuf value decoding',
  'btn.newKey': 'New Key',
  'tip.newKey': 'New Key (⌘N)',
  'btn.refresh': 'Refresh',
  'tip.refresh': 'Refresh (⌘R)',
  'tip.disconnect': 'Disconnect',

  // sidebar
  'sidebar.title': 'Connections',
  'sidebar.addTitle': 'Add connection',
  'sidebar.addConnection': 'Add Connection',
  'sidebar.connect': 'Connect',
  'sidebar.disconnect': 'Disconnect',
  'sidebar.edit': 'Edit',
  'sidebar.delete': 'Del',
  'sidebar.empty1': 'No connections yet.',
  'sidebar.deleteTitle': 'Delete connection profile?',
  'sidebar.deleteName': 'Name: {name}',
  'sidebar.deleteEndpoints': 'Endpoints: {endpoints}',
  'sidebar.deleteKeychainNote': 'The stored password (if any) will be removed from the Keychain.',
  'sidebar.deletedToast': 'Removed connection {name}',

  // connections
  'connections.connectedToast': 'Connected to {name}',

  // key tree
  'tree.title': 'Keys',
  'tree.connectFirst': 'Connect to an etcd server to browse keys.',
  'tree.more': 'More…',
  'tree.page': 'page',
  'tree.tipPageSize': 'Rows fetched per tree level',
  'tree.empty': 'empty',

  // inspector
  'inspector.empty': 'Select a key to inspect its value.',
  'inspector.loading': 'Loading…',
  'badge.binary': 'Binary value',
  'badge.large': 'Large Value: {size}',
  'badge.json': 'JSON',
  'badge.proto': 'Protobuf: {name}',
  'mode.text': 'Text',
  'mode.hex': 'Hex',
  'mode.base64': 'Base64',
  'mode.proto': 'Proto',
  'mode.tipText': 'UTF-8 text',
  'mode.tipNotUTF8': 'Value is not valid UTF-8',
  'tip.prettyJSON': 'Format as pretty JSON (content is saved as-is on Save)',
  'tip.reload': 'Reload from server',
  'tip.delete': 'Delete this key',
  'tip.save': 'Save (⌘S) with ModRevision compare-and-swap',
  'btn.prettyJSON': 'Pretty JSON',
  'btn.reload': 'Reload',
  'btn.delete': 'Delete',
  'btn.save': 'Save',
  'btn.saved': 'Saved',
  'inspector.invalidHex': 'Invalid hex input — fix it before saving.',
  'inspector.invalidBase64': 'Invalid base64 input — fix it before saving.',
  'inspector.protoNote': 'Read-only decode of the protobuf value (score-ranked candidates). Switch to Text/Hex/Base64 to edit.',
  'inspector.protoTooLarge': '(decoded payload too large to display — use Hex view)',
  'inspector.deleteTitle': 'Delete key?',
  'inspector.deleteServer': 'Server: {name}',
  'inspector.deleteKeyLine': 'Key: {key}',
  'inspector.cannotUndo': 'This operation cannot be undone.',
  'inspector.savedToast': 'Key saved',
  'inspector.deletedToast': 'Deleted {key}',
  'inspector.conflictTitle': 'Modified by another client',
  'inspector.conflictBody': '{message}',
  'inspector.conflictQuestion': 'Reload the key now? Your local edits will be discarded.',
  'inspector.reload': 'Reload',

  // connection dialog
  'conn.addTitle': 'Add Connection',
  'conn.editTitle': 'Edit Connection — {name}',
  'conn.name': 'Name',
  'conn.endpoints': 'Endpoints',
  'conn.endpointsHint': '(one per line)',
  'conn.endpointsPlaceholder': '127.0.0.1:2379\n127.0.0.1:22379',
  'conn.username': 'Username',
  'conn.optional': '(optional)',
  'conn.usernamePlaceholder': 'root',
  'conn.password': 'Password',
  'conn.passwordStored': ', stored in Keychain',
  'conn.passwordPlaceholderSaved': '•••••••• (saved)',
  'conn.remember': 'Remember password in macOS Keychain',
  'conn.dialTimeout': 'Dial timeout (ms)',
  'conn.requestTimeout': 'Request timeout (ms)',
  'conn.enableTLS': 'Enable TLS',
  'conn.ca': 'CA certificate',
  'conn.caPlaceholder': '/path/to/ca.pem',
  'conn.clientCert': 'Client cert',
  'conn.certPlaceholder': '/path/to/client.pem',
  'conn.clientKey': 'Client key',
  'conn.keyPlaceholder': '/path/to/client-key.pem',
  'conn.skipVerify': 'Skip certificate verification (insecure)',
  'conn.test': 'Test Connection',
  'conn.testing': 'Testing…',
  'conn.save': 'Save',
  'conn.saving': 'Saving…',
  'conn.cancel': 'Cancel',
  'conn.testOk': 'OK · revision {rev} · etcd {version} ({endpoint})',
  'conn.savedToast': 'Saved connection {name}',

  // new key dialog
  'newkey.title': 'New Key',
  'newkey.key': 'Key',
  'newkey.keyPlaceholder': '/myapp/config/key',
  'newkey.value': 'Value',
  'newkey.valuePlaceholder': 'value (saved as UTF-8 bytes)',
  'newkey.lease': 'Lease ID',
  'newkey.leasePlaceholder': '7587839385437545011',
  'newkey.hint': 'If the key already exists you will be asked whether to overwrite it.',
  'newkey.create': 'Create',
  'newkey.creating': 'Creating…',
  'newkey.emptyKey': 'Key must not be empty',
  'newkey.existsTitle': 'Key already exists',
  'newkey.existsKeyLine': 'Key: {key}',
  'newkey.existsQuestion': 'Overwrite the existing value?',
  'newkey.overwrite': 'Overwrite',
  'newkey.createdToast': 'Created {key}',
  'newkey.overwroteToast': 'Overwrote {key}',

  // proto dialog
  'proto.title': 'Protobuf definitions',
  'proto.hint': 'Import .proto files to decode binary values that hold protobuf data. The app tries every imported message type on each value and ranks the best matches (you can switch between them in the value panel). Cross-file imports are resolved from the files\' directories.',
  'proto.paths': 'Paths',
  'proto.pathsHint': '(one .proto file per line)',
  'proto.pathsPlaceholder': '/path/to/demo.proto\n/path/to/common.proto',
  'proto.browse': 'Browse…',
  'proto.selectTitle': 'Select .proto files',
  'proto.imported': 'Imported ({n} files)',
  'proto.none': 'No proto files imported yet.',
  'proto.clearAll': 'Clear all',
  'proto.close': 'Close',
  'proto.import': 'Import',
  'proto.importing': 'Importing…',
  'proto.importedToast': 'Imported {n} proto files ({m} message types)',
  'proto.clearedToast': 'Cleared imported protos',

  // confirm / common
  'confirm.ok': 'OK',
  'confirm.cancel': 'Cancel',
  'common.retry': 'Retry',
  'locale.label': 'Language',

  // status bar
  'status.connected': 'connected',
  'status.disconnected': 'disconnected',
  'status.connecting': 'connecting',
  'status.error': 'error',
  'status.hint': '⌘R refresh · ⌘N new key · ⌘S save · ⌘F search',

  // localized error messages (fallback: backend message)
  'error.CONNECTION_FAILED': 'Could not reach the etcd server',
  'error.NOT_CONNECTED': 'Not connected to an etcd server',
  'error.TIMEOUT': 'Request timed out',
  'error.CANCELLED': 'Request cancelled',
  'error.KEY_NOT_FOUND': 'Key not found',
  'error.KEY_ALREADY_EXISTS': 'Key already exists',
  'error.REVISION_CONFLICT': 'The key was modified by another client — reload before saving',
  'error.LEASE_NOT_FOUND': 'The requested lease does not exist',
  'error.PERMISSION_DENIED': 'Authentication or permission check failed',
  'error.TLS_ERROR': 'TLS error',
  'error.INVALID_ARGUMENT': 'Invalid request',
  'error.QUOTA_EXCEEDED': 'etcd quota exceeded',
  'error.PROTO_ERROR': 'Failed to import proto files',
  'error.CONFIG_ERROR': 'Failed to read or write the configuration',
  'error.UNKNOWN': 'Unexpected error',
} as const;

type Dict = Record<keyof typeof en, string>;

const zhCN: Dict = {
  'toolbar.noConnection': '未连接',
  'toolbar.connectingSuffix': '（连接中…）',
  'toolbar.checkmark': ' ✓',
  'toolbar.disconnect': '断开',
  'placeholder.prefix': '基础前缀，如 / 或 /registry/services/',
  'placeholder.exact': '精确 key，按 Enter 查找',
  'search.prefix': '前缀',
  'search.exact': '精确 key',
  'tip.searchMode': '搜索模式',
  'tip.connection': '当前连接',
  'btn.protos': 'Protos',
  'tip.protos': '导入 .proto 定义，用于解码 protobuf 二进制值',
  'btn.newKey': '新建 Key',
  'tip.newKey': '新建 Key（⌘N）',
  'btn.refresh': '刷新',
  'tip.refresh': '刷新（⌘R）',
  'tip.disconnect': '断开连接',

  'sidebar.title': '连接',
  'sidebar.addTitle': '添加连接',
  'sidebar.addConnection': '添加连接',
  'sidebar.connect': '连接',
  'sidebar.disconnect': '断开',
  'sidebar.edit': '编辑',
  'sidebar.delete': '删除',
  'sidebar.empty1': '还没有连接配置。',
  'sidebar.deleteTitle': '删除连接配置？',
  'sidebar.deleteName': '名称：{name}',
  'sidebar.deleteEndpoints': 'Endpoints：{endpoints}',
  'sidebar.deleteKeychainNote': '已存储的密码（如有）将同时从钥匙串移除。',
  'sidebar.deletedToast': '已删除连接 {name}',

  'connections.connectedToast': '已连接到 {name}',

  'tree.title': 'Keys',
  'tree.connectFirst': '连接到 etcd 服务器后即可浏览 key。',
  'tree.more': '更多…',
  'tree.page': '每页',
  'tree.tipPageSize': '每层目录拉取的行数',
  'tree.empty': '空',

  'inspector.empty': '选择一个 key 查看其内容。',
  'inspector.loading': '加载中…',
  'badge.binary': '二进制值',
  'badge.large': '大值：{size}',
  'badge.json': 'JSON',
  'badge.proto': 'Protobuf：{name}',
  'mode.text': '文本',
  'mode.hex': '十六进制',
  'mode.base64': 'Base64',
  'mode.proto': 'Proto',
  'mode.tipText': 'UTF-8 文本',
  'mode.tipNotUTF8': '值不是合法的 UTF-8',
  'tip.prettyJSON': '格式化为 JSON（保存时仍以编辑器当前内容为准）',
  'tip.reload': '从服务器重新加载',
  'tip.delete': '删除此 key',
  'tip.save': '保存（⌘S，带 ModRevision 比较防覆盖）',
  'btn.prettyJSON': '格式化 JSON',
  'btn.reload': '重新加载',
  'btn.delete': '删除',
  'btn.save': '保存',
  'btn.saved': '已保存',
  'inspector.invalidHex': '十六进制格式错误——保存前请修正。',
  'inspector.invalidBase64': 'Base64 格式错误——保存前请修正。',
  'inspector.protoNote': 'Protobuf 解码结果（只读，按匹配度排序）。切换到 文本/十六进制/Base64 可编辑。',
  'inspector.protoTooLarge': '（解码结果过大无法显示——请使用十六进制视图）',
  'inspector.deleteTitle': '删除 key？',
  'inspector.deleteServer': '服务器：{name}',
  'inspector.deleteKeyLine': 'Key：{key}',
  'inspector.cannotUndo': '此操作不可撤销。',
  'inspector.savedToast': '已保存',
  'inspector.deletedToast': '已删除 {key}',
  'inspector.conflictTitle': '已被其他客户端修改',
  'inspector.conflictBody': '{message}',
  'inspector.conflictQuestion': '现在重新加载该 key 吗？本地修改将被丢弃。',
  'inspector.reload': '重新加载',

  'conn.addTitle': '添加连接',
  'conn.editTitle': '编辑连接 — {name}',
  'conn.name': '名称',
  'conn.endpoints': 'Endpoints',
  'conn.endpointsHint': '（每行一个）',
  'conn.endpointsPlaceholder': '127.0.0.1:2379\n127.0.0.1:22379',
  'conn.username': '用户名',
  'conn.optional': '（可选）',
  'conn.usernamePlaceholder': 'root',
  'conn.password': '密码',
  'conn.passwordStored': '（已存入钥匙串）',
  'conn.passwordPlaceholderSaved': '••••••••（已保存）',
  'conn.remember': '记住密码（存入 macOS 钥匙串）',
  'conn.dialTimeout': '拨号超时（毫秒）',
  'conn.requestTimeout': '请求超时（毫秒）',
  'conn.enableTLS': '启用 TLS',
  'conn.ca': 'CA 证书',
  'conn.caPlaceholder': '/path/to/ca.pem',
  'conn.clientCert': '客户端证书',
  'conn.certPlaceholder': '/path/to/client.pem',
  'conn.clientKey': '客户端私钥',
  'conn.keyPlaceholder': '/path/to/client-key.pem',
  'conn.skipVerify': '跳过证书校验（不安全）',
  'conn.test': '测试连接',
  'conn.testing': '测试中…',
  'conn.save': '保存',
  'conn.saving': '保存中…',
  'conn.cancel': '取消',
  'conn.testOk': '正常 · revision {rev} · etcd {version}（{endpoint}）',
  'conn.savedToast': '已保存连接 {name}',

  'newkey.title': '新建 Key',
  'newkey.key': 'Key',
  'newkey.keyPlaceholder': '/myapp/config/key',
  'newkey.value': '值',
  'newkey.valuePlaceholder': '值（以 UTF-8 字节保存）',
  'newkey.lease': 'Lease ID',
  'newkey.leasePlaceholder': '7587839385437545011',
  'newkey.hint': '如果 key 已存在，会询问是否覆盖。',
  'newkey.create': '创建',
  'newkey.creating': '创建中…',
  'newkey.emptyKey': 'key 不能为空',
  'newkey.existsTitle': 'Key 已存在',
  'newkey.existsKeyLine': 'Key：{key}',
  'newkey.existsQuestion': '是否覆盖现有值？',
  'newkey.overwrite': '覆盖',
  'newkey.createdToast': '已创建 {key}',
  'newkey.overwroteToast': '已覆盖 {key}',

  'proto.title': 'Protobuf 定义',
  'proto.hint': '导入 .proto 文件后，可解码 protobuf 二进制值：应用会对每个值尝试所有已导入的消息类型并按匹配度排序（可在值面板切换候选）。跨文件 import 会从所选文件的目录解析。',
  'proto.paths': '文件路径',
  'proto.pathsHint': '（每行一个 .proto 文件）',
  'proto.pathsPlaceholder': '/path/to/demo.proto\n/path/to/common.proto',
  'proto.browse': '浏览…',
  'proto.selectTitle': '选择 .proto 文件',
  'proto.imported': '已导入（{n} 个文件）',
  'proto.none': '尚未导入任何 proto 文件。',
  'proto.clearAll': '全部清除',
  'proto.close': '关闭',
  'proto.import': '导入',
  'proto.importing': '导入中…',
  'proto.importedToast': '已导入 {n} 个 proto 文件（{m} 个消息类型）',
  'proto.clearedToast': '已清除导入的 proto',

  'confirm.ok': '确定',
  'confirm.cancel': '取消',
  'common.retry': '重试',
  'locale.label': '语言',

  'status.connected': '已连接',
  'status.disconnected': '未连接',
  'status.connecting': '连接中',
  'status.error': '错误',
  'status.hint': '⌘R 刷新 · ⌘N 新建 · ⌘S 保存 · ⌘F 搜索',

  'error.CONNECTION_FAILED': '无法连接到 etcd 服务器',
  'error.NOT_CONNECTED': '尚未连接到 etcd 服务器',
  'error.TIMEOUT': '请求超时',
  'error.CANCELLED': '请求已取消',
  'error.KEY_NOT_FOUND': 'key 不存在',
  'error.KEY_ALREADY_EXISTS': 'key 已存在',
  'error.REVISION_CONFLICT': 'key 已被其他客户端修改，请先重新加载再保存',
  'error.LEASE_NOT_FOUND': '指定的 lease 不存在',
  'error.PERMISSION_DENIED': '认证或权限校验失败',
  'error.TLS_ERROR': 'TLS 错误',
  'error.INVALID_ARGUMENT': '请求参数无效',
  'error.QUOTA_EXCEEDED': 'etcd 配额已超限',
  'error.PROTO_ERROR': 'proto 文件导入失败',
  'error.CONFIG_ERROR': '配置读写失败',
  'error.UNKNOWN': '未知错误',
};

const dictionaries: Record<Locale, Dict> = { en, 'zh-CN': zhCN };

const LOCALE_KEY = 'etcd-eye.locale';

function detectLocale(): Locale {
  try {
    const saved = localStorage.getItem(LOCALE_KEY);
    if (saved === 'en' || saved === 'zh-CN') return saved;
  } catch {
    /* localStorage unavailable */
  }
  const nav = typeof navigator !== 'undefined' ? navigator.language : 'en';
  return nav.toLowerCase().startsWith('zh') ? 'zh-CN' : 'en';
}

export const locale = writable<Locale>(detectLocale());

locale.subscribe((l) => {
  try {
    localStorage.setItem(LOCALE_KEY, l);
    document.documentElement.lang = l;
  } catch {
    /* non-browser or storage disabled */
  }
});

export type Params = Record<string, string | number>;

function translate(l: Locale, key: string, params?: Params): string {
  const dict = dictionaries[l] ?? dictionaries.en;
  let s: string = dict[key as keyof Dict] ?? (dictionaries.en[key as keyof Dict] ?? key);
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      s = s.replaceAll(`{${k}}`, String(v));
    }
  }
  return s;
}

/** Reactive translator for templates: `$t('btn.save')`. */
export const t: Readable<(key: string, params?: Params) => string> = derived(
  locale,
  ($l) => (key: string, params?: Params) => translate($l, key, params),
);

/** Imperative translator for store/action code: `tr('x.saved', { key })`. */
export function tr(key: string, params?: Params): string {
  return translate(get(locale), key, params);
}

/** True when the key exists in any dictionary (used for error fallbacks). */
export function has(key: string): boolean {
  return (
    dictionaries.en[key as keyof Dict] !== undefined ||
    dictionaries['zh-CN'][key as keyof Dict] !== undefined
  );
}

export function setLocale(l: Locale): void {
  locale.set(l);
}

export const locales: { id: Locale; label: string }[] = [
  { id: 'en', label: 'English' },
  { id: 'zh-CN', label: '简体中文' },
];
