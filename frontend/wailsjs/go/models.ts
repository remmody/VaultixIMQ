export namespace app {
	
	export class AboutInfo {
	    version: string;
	    author: string;
	    license: string;
	    github: string;
	
	    static createFrom(source: any = {}) {
	        return new AboutInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.author = source["author"];
	        this.license = source["license"];
	        this.github = source["github"];
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
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sync_interval = source["sync_interval"];
	        this.auto_login = source["auto_login"];
	        this.notifications = source["notifications"];
	        this.sound = source["sound"];
	        this.app_password_hash = source["app_password_hash"];
	        this.app_password_salt = source["app_password_salt"];
	        this.auto_lock_interval = source["auto_lock_interval"];
	        this.app_password_setup_done = source["app_password_setup_done"];
	    }
	}
	export class UpdateInfo {
	    has_update: boolean;
	    latest_version: string;
	    current_version: string;
	    release_notes: string;
	    download_url: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.has_update = source["has_update"];
	        this.latest_version = source["latest_version"];
	        this.current_version = source["current_version"];
	        this.release_notes = source["release_notes"];
	        this.download_url = source["download_url"];
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
	    port: string;
	    label: string;
	    status: string;
	    unread_count: number;
	    last_message_time: number;
	
	    static createFrom(source: any = {}) {
	        return new Account(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.email = source["email"];
	        this.password = source["password"];
	        this.imap_host = source["imap_host"];
	        this.imap_port = source["imap_port"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.label = source["label"];
	        this.status = source["status"];
	        this.unread_count = source["unread_count"];
	        this.last_message_time = source["last_message_time"];
	    }
	}
	export class AccountLight {
	    email: string;
	    label: string;
	    status: string;
	    unread_count: number;
	    last_message_time: number;
	
	    static createFrom(source: any = {}) {
	        return new AccountLight(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.email = source["email"];
	        this.label = source["label"];
	        this.status = source["status"];
	        this.unread_count = source["unread_count"];
	        this.last_message_time = source["last_message_time"];
	    }
	}
	export class Message {
	    uid: number;
	    subject: string;
	    from: string;
	    date: string;
	    date_unix: number;
	    body: string;
	    seen: boolean;
	    codes: string[];
	
	    static createFrom(source: any = {}) {
	        return new Message(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uid = source["uid"];
	        this.subject = source["subject"];
	        this.from = source["from"];
	        this.date = source["date"];
	        this.date_unix = source["date_unix"];
	        this.body = source["body"];
	        this.seen = source["seen"];
	        this.codes = source["codes"];
	    }
	}

}

export namespace totp {
	
	export class Entry {
	    account_name: string;
	    issuer: string;
	    secret: string;
	    account: string;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.account_name = source["account_name"];
	        this.issuer = source["issuer"];
	        this.secret = source["secret"];
	        this.account = source["account"];
	    }
	}
	export class Response {
	    code: string;
	    timeLeft: number;
	
	    static createFrom(source: any = {}) {
	        return new Response(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.timeLeft = source["timeLeft"];
	    }
	}

}

