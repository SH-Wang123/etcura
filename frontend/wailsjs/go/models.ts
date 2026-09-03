export namespace model {
	
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
	    endpoints: string[];
	    username?: string;
	    dialTimeoutMs: number;
	    requestTimeoutMs: number;
	    tls: TLSConfig;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.endpoints = source["endpoints"];
	        this.username = source["username"];
	        this.dialTimeoutMs = source["dialTimeoutMs"];
	        this.requestTimeoutMs = source["requestTimeoutMs"];
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
	export class StatusInfo {
	    state: string;
	    name: string;
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
	        this.name = source["name"];
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

}

