export namespace model {
	
	export class RecordValue {
	    base64: string;
	    utf8: boolean;
	    bytes: number;
	    truncated: boolean;

	    static createFrom(source: any = {}) {
	        return new RecordValue(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.base64 = source["base64"];
	        this.utf8 = source["utf8"];
	        this.bytes = source["bytes"];
	        this.truncated = source["truncated"];
	    }
	}
	export class CompareRow {
	    key: string;
	    label: string;
	    state: string;
	    sourceLease: boolean;

	    static createFrom(source: any = {}) {
	        return new CompareRow(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.label = source["label"];
	        this.state = source["state"];
	        this.sourceLease = source["sourceLease"];
	    }
	}
	export class CompareDetail {
	    row: CompareRow;
	    source?: RecordValue;
	    target?: RecordValue;

	    static createFrom(source: any = {}) {
	        return new CompareDetail(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.row = this.convertValues(source["row"], CompareRow);
	        this.source = this.convertValues(source["source"], RecordValue);
	        this.target = this.convertValues(source["target"], RecordValue);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CompareInfo {
	    id: string;
	    sourceRevision: string;
	    targetRevision: string;
	    targetReadOnly: boolean;

	    static createFrom(source: any = {}) {
	        return new CompareInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sourceRevision = source["sourceRevision"];
	        this.targetRevision = source["targetRevision"];
	        this.targetReadOnly = source["targetReadOnly"];
	    }
	}
	export class ComparePage {
	    rows: CompareRow[];
	    nextCursor: string;
	    scanned: number;

	    static createFrom(source: any = {}) {
	        return new ComparePage(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rows = this.convertValues(source["rows"], CompareRow);
	        this.nextCursor = source["nextCursor"];
	        this.scanned = source["scanned"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CompareRequest {
	    sourceSession: string;
	    targetSession: string;
	    sourcePrefix: string;
	    targetPrefix: string;

	    static createFrom(source: any = {}) {
	        return new CompareRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourceSession = source["sourceSession"];
	        this.targetSession = source["targetSession"];
	        this.sourcePrefix = source["sourcePrefix"];
	        this.targetPrefix = source["targetPrefix"];
	    }
	}

	export class TLSConfig {
	    enabled: boolean;
	    caFile?: string;
	    certFile?: string;
	    keyFile?: string;
	    skipVerify: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TLSConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.caFile = source["caFile"];
	        this.certFile = source["certFile"];
	        this.keyFile = source["keyFile"];
	        this.skipVerify = source["skipVerify"];
	    }
	}
	export class ConnectionConfig {
	    id: string;
	    name: string;
	    environment?: string;
	    endpoints: string[];
	    username?: string;
	    dialTimeoutMs: number;
	    requestTimeoutMs: number;
	    readOnly: boolean;
	    tls: TLSConfig;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.environment = source["environment"];
	        this.endpoints = source["endpoints"];
	        this.username = source["username"];
	        this.dialTimeoutMs = source["dialTimeoutMs"];
	        this.requestTimeoutMs = source["requestTimeoutMs"];
	        this.readOnly = source["readOnly"];
	        this.tls = this.convertValues(source["tls"], TLSConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ConnectionSummary {
	    id: string;
	    name: string;
	    endpoints: string[];
	    hasPassword: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.endpoints = source["endpoints"];
	        this.hasPassword = source["hasPassword"];
	    }
	}
	export class KeySummary {
	    key: string;
	    createRevision: number;
	    modRevision: number;
	    version: number;
	    lease: number;
	
	    static createFrom(source: any = {}) {
	        return new KeySummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.createRevision = source["createRevision"];
	        this.modRevision = source["modRevision"];
	        this.version = source["version"];
	        this.lease = source["lease"];
	    }
	}
	export class KeyPage {
	    prefix: string;
	    cursor: string;
	    nextCursor: string;
	    hasMore: boolean;
	    revision: number;
	    items: KeySummary[];
	
	    static createFrom(source: any = {}) {
	        return new KeyPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.prefix = source["prefix"];
	        this.cursor = source["cursor"];
	        this.nextCursor = source["nextCursor"];
	        this.hasMore = source["hasMore"];
	        this.revision = source["revision"];
	        this.items = this.convertValues(source["items"], KeySummary);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class KvDetail {
	    key: string;
	    valueBase64: string;
	    isUTF8: boolean;
	    sizeBytes: number;
	    createRevision: number;
	    modRevision: number;
	    version: number;
	    lease: number;
	
	    static createFrom(source: any = {}) {
	        return new KvDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.valueBase64 = source["valueBase64"];
	        this.isUTF8 = source["isUTF8"];
	        this.sizeBytes = source["sizeBytes"];
	        this.createRevision = source["createRevision"];
	        this.modRevision = source["modRevision"];
	        this.version = source["version"];
	        this.lease = source["lease"];
	    }
	}
	export class LevelRow {
	    type: string;
	    name: string;
	    path: string;
	    modRevision: number;
	    version: number;
	    lease: number;
	
	    static createFrom(source: any = {}) {
	        return new LevelRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.modRevision = source["modRevision"];
	        this.version = source["version"];
	        this.lease = source["lease"];
	    }
	}
	export class LevelPage {
	    prefix: string;
	    cursor: string;
	    nextCursor: string;
	    hasMore: boolean;
	    rows: LevelRow[];
	
	    static createFrom(source: any = {}) {
	        return new LevelPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.prefix = source["prefix"];
	        this.cursor = source["cursor"];
	        this.nextCursor = source["nextCursor"];
	        this.hasMore = source["hasMore"];
	        this.rows = this.convertValues(source["rows"], LevelRow);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ProtoCandidate {
	    messageName: string;
	    score: number;
	    setFieldNums: number[];
	    unknownBytes: number;
	    json: string;
	
	    static createFrom(source: any = {}) {
	        return new ProtoCandidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.messageName = source["messageName"];
	        this.score = source["score"];
	        this.setFieldNums = source["setFieldNums"];
	        this.unknownBytes = source["unknownBytes"];
	        this.json = source["json"];
	    }
	}
	export class ProtoFileInfo {
	    name: string;
	    package: string;
	    messageCount: number;
	    enumCount: number;
	
	    static createFrom(source: any = {}) {
	        return new ProtoFileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.package = source["package"];
	        this.messageCount = source["messageCount"];
	        this.enumCount = source["enumCount"];
	    }
	}
	export class ProtoMatchResult {
	    wireTagNums: number[];
	    scanned: number;
	    valueTooLarge: boolean;
	    candidates: ProtoCandidate[];
	
	    static createFrom(source: any = {}) {
	        return new ProtoMatchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.wireTagNums = source["wireTagNums"];
	        this.scanned = source["scanned"];
	        this.valueTooLarge = source["valueTooLarge"];
	        this.candidates = this.convertValues(source["candidates"], ProtoCandidate);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

	export class ScriptConfig {
	    language: string;
	    inputVar: string;
	    outputVar: string;
	    body: string;
	
	    static createFrom(source: any = {}) {
	        return new ScriptConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.language = source["language"];
	        this.inputVar = source["inputVar"];
	        this.outputVar = source["outputVar"];
	        this.body = source["body"];
	    }
	}
	export class ScriptInfo {
	    configured: boolean;
	    language: string;
	    inputVar: string;
	    outputVar: string;
	    interpreter: string;
	    available: boolean;
	    warning?: string;
	
	    static createFrom(source: any = {}) {
	        return new ScriptInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configured = source["configured"];
	        this.language = source["language"];
	        this.inputVar = source["inputVar"];
	        this.outputVar = source["outputVar"];
	        this.interpreter = source["interpreter"];
	        this.available = source["available"];
	        this.warning = source["warning"];
	    }
	}
	export class ScriptOutput {
	    output: string;
	    stderr: string;
	    exitCode: number;
	    durationMs: number;
	    truncated: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ScriptOutput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.output = source["output"];
	        this.stderr = source["stderr"];
	        this.exitCode = source["exitCode"];
	        this.durationMs = source["durationMs"];
	        this.truncated = source["truncated"];
	    }
	}
	export class StatusInfo {
	    state: string;
	    profileId: string;
	    session: string;
	    readOnly: boolean;
	    name: string;
	    environment: string;
	    endpoints: string;
	    header: number;
	    etcdVersion: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new StatusInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.profileId = source["profileId"];
	        this.session = source["session"];
	        this.readOnly = source["readOnly"];
	        this.name = source["name"];
	        this.environment = source["environment"];
	        this.endpoints = source["endpoints"];
	        this.header = source["header"];
	        this.etcdVersion = source["etcdVersion"];
	        this.message = source["message"];
	    }
	}
	export class StatusResponse {
	    ok: boolean;
	    header: number;
	    etcdVersion: string;
	    endpoint: string;
	
	    static createFrom(source: any = {}) {
	        return new StatusResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.header = source["header"];
	        this.etcdVersion = source["etcdVersion"];
	        this.endpoint = source["endpoint"];
	    }
	}
	export class SyncPlan {
	    id: string;
	    rows: CompareRow[];

	    static createFrom(source: any = {}) {
	        return new SyncPlan(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.rows = this.convertValues(source["rows"], CompareRow);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SyncResult {
	    key: string;
	    label: string;
	    state: string;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new SyncResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.label = source["label"];
	        this.state = source["state"];
	        this.message = source["message"];
	    }
	}

	export class WatchEvent {
	    type: string;
	    key: string;
	    revision: string;
	    seenAt: number;
	    before?: RecordValue;
	    after?: RecordValue;

	    static createFrom(source: any = {}) {
	        return new WatchEvent(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.key = source["key"];
	        this.revision = source["revision"];
	        this.seenAt = source["seenAt"];
	        this.before = this.convertValues(source["before"], RecordValue);
	        this.after = this.convertValues(source["after"], RecordValue);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class WatchBatch {
	    events: WatchEvent[];
	    state: string;
	    revision: string;
	    missed: number;
	    error: string;

	    static createFrom(source: any = {}) {
	        return new WatchBatch(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.events = this.convertValues(source["events"], WatchEvent);
	        this.state = source["state"];
	        this.revision = source["revision"];
	        this.missed = source["missed"];
	        this.error = source["error"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

	export class WatchStart {
	    session: string;
	    key: string;
	    prefix: boolean;

	    static createFrom(source: any = {}) {
	        return new WatchStart(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.session = source["session"];
	        this.key = source["key"];
	        this.prefix = source["prefix"];
	    }
	}

}

