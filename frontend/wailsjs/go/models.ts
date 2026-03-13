export namespace app {
	
	export class AboutInfo {
	    version: string;
	    author: string;
	    license: string;
	    github: string;
	
	    static createFrom(source: unknown = {}) {
	        return new AboutInfo(source);
	    }
	
	    constructor(source: unknown = {}) {
	        if ('string' === typeof source) source = JSON.parse(source as string);
	        const s = source as Record<string, any>;
	        this.version = s["version"];
	        this.author = s["author"];
	        this.license = s["license"];
	        this.github = s["github"];
	    }
	}
	export class Settings {
	    sync_interval: number;
	    auto_login: boolean;
	    notifications: boolean;
	    sound: boolean;
	    app_password_hash?: string;
	    app_password_salt?: string;
	    auto_lock_interval?: number;
	    app_password_setup_done: boolean;
	
	    static createFrom(source: unknown = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: unknown = {}) {
	        if ('string' === typeof source) source = JSON.parse(source as string);
	        const s = source as Record<string, any>;
	        this.sync_interval = s["sync_interval"];
	        this.auto_login = s["auto_login"];
	        this.notifications = s["notifications"];
	        this.sound = s["sound"];
	        this.app_password_hash = s["app_password_hash"];
	        this.app_password_salt = s["app_password_salt"];
	        this.auto_lock_interval = s["auto_lock_interval"];
	        this.app_password_setup_done = s["app_password_setup_done"];
	    }
	}
	export class UpdateInfo {
	    has_update: boolean;
	    latest_version: string;
	    current_version: string;
	    release_notes: string;
	    download_url: string;
	
	    static createFrom(source: unknown = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: unknown = {}) {
	        if ('string' === typeof source) source = JSON.parse(source as string);
	        const s = source as Record<string, any>;
	        this.has_update = s["has_update"];
	        this.latest_version = s["latest_version"];
	        this.current_version = s["current_version"];
	        this.release_notes = s["release_notes"];
	        this.download_url = s["download_url"];
	    }
	}

}

export namespace mail {
	
	export class Account {
	    email: string;
	    password: string;
	    imap_host: string;
	    imap_port: string;
	    host?: string;
	    port?: string;
	    label: string;
	    unread_count: number;
	
	    static createFrom(source: unknown = {}) {
	        return new Account(source);
	    }
	
	    constructor(source: unknown = {}) {
	        if ('string' === typeof source) source = JSON.parse(source as string);
	        const s = source as Record<string, any>;
	        this.email = s["email"];
	        this.password = s["password"];
	        this.imap_host = s["imap_host"];
	        this.imap_port = s["imap_port"];
	        this.host = s["host"];
	        this.port = s["port"];
	        this.label = s["label"];
	        this.unread_count = s["unread_count"];
	    }
	}
	export class Message {
	    uid: number;
	    subject: string;
	    from: string;
	    date: string;
	    body: string;
	    seen: boolean;
	    codes: string[];
	
	    static createFrom(source: unknown = {}) {
	        return new Message(source);
	    }
	
	    constructor(source: unknown = {}) {
	        if ('string' === typeof source) source = JSON.parse(source as string);
	        const s = source as Record<string, any>;
	        this.uid = s["uid"];
	        this.subject = s["subject"];
	        this.from = s["from"];
	        this.date = s["date"];
	        this.body = s["body"];
	        this.seen = s["seen"];
	        this.codes = s["codes"];
	    }
	}

}

export namespace totp {
	
	export class Entry {
	    account_name: string;
	    issuer: string;
	    secret: string;
	
	    static createFrom(source: unknown = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: unknown = {}) {
	        if ('string' === typeof source) source = JSON.parse(source as string);
	        const s = source as Record<string, any>;
	        this.account_name = s["account_name"];
	        this.issuer = s["issuer"];
	        this.secret = s["secret"];
	    }
	}
	export class Response {
	    code: string;
	    timeLeft: number;
	
	    static createFrom(source: unknown = {}) {
	        return new Response(source);
	    }
	
	    constructor(source: unknown = {}) {
	        if ('string' === typeof source) source = JSON.parse(source as string);
	        const s = source as Record<string, any>;
	        this.code = s["code"];
	        this.timeLeft = s["timeLeft"];
	    }
	}

}

