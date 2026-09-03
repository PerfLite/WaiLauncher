export namespace auth {
	
	export class MCTokenData {
	    accessToken: string;
	    // Go type: time
	    expiresAt: any;
	
	    static createFrom(source: any = {}) {
	        return new MCTokenData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accessToken = source["accessToken"];
	        this.expiresAt = this.convertValues(source["expiresAt"], null);
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
	export class MSTokenData {
	    accessToken: string;
	    refreshToken: string;
	    // Go type: time
	    expiresAt: any;
	
	    static createFrom(source: any = {}) {
	        return new MSTokenData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accessToken = source["accessToken"];
	        this.refreshToken = source["refreshToken"];
	        this.expiresAt = this.convertValues(source["expiresAt"], null);
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
	export class MojangCape {
	    id: string;
	    state: string;
	    url: string;
	    alias: string;
	    dataUrl?: string;
	
	    static createFrom(source: any = {}) {
	        return new MojangCape(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.state = source["state"];
	        this.url = source["url"];
	        this.alias = source["alias"];
	        this.dataUrl = source["dataUrl"];
	    }
	}
	export class Account {
	    id: string;
	    type: string;
	    username: string;
	    uuid: string;
	    skinUrl?: string;
	    skinModel?: string;
	    capeUrl?: string;
	    capes?: MojangCape[];
	    avatarUrl?: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    lastUsed: any;
	    msToken?: MSTokenData;
	    mcToken?: MCTokenData;
	    xuid?: string;
	    ownsGame?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Account(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.username = source["username"];
	        this.uuid = source["uuid"];
	        this.skinUrl = source["skinUrl"];
	        this.skinModel = source["skinModel"];
	        this.capeUrl = source["capeUrl"];
	        this.capes = this.convertValues(source["capes"], MojangCape);
	        this.avatarUrl = source["avatarUrl"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.lastUsed = this.convertValues(source["lastUsed"], null);
	        this.msToken = this.convertValues(source["msToken"], MSTokenData);
	        this.mcToken = this.convertValues(source["mcToken"], MCTokenData);
	        this.xuid = source["xuid"];
	        this.ownsGame = source["ownsGame"];
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
	export class AccountsData {
	    accounts: Account[];
	    activeId: string;
	
	    static createFrom(source: any = {}) {
	        return new AccountsData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accounts = this.convertValues(source["accounts"], Account);
	        this.activeId = source["activeId"];
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
	export class DeviceCodeResponse {
	    deviceCode: string;
	    userCode: string;
	    verificationUri: string;
	    expiresIn: number;
	    interval: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new DeviceCodeResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deviceCode = source["deviceCode"];
	        this.userCode = source["userCode"];
	        this.verificationUri = source["verificationUri"];
	        this.expiresIn = source["expiresIn"];
	        this.interval = source["interval"];
	        this.message = source["message"];
	    }
	}
	
	
	
	export class PresetCape {
	    id: string;
	    name: string;
	    category: string;
	    url: string;
	    dataUrl?: string;
	
	    static createFrom(source: any = {}) {
	        return new PresetCape(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.category = source["category"];
	        this.url = source["url"];
	        this.dataUrl = source["dataUrl"];
	    }
	}

}

export namespace launcher {
	
	export class ArticleDetails {
	    title: string;
	    translatedTitle?: string;
	    heroImage: string;
	    author: string;
	    date: string;
	    displayDate: string;
	    tag: string;
	    kind: string;
	    link: string;
	    contentHtml: string;
	    translatedHtml?: string;
	
	    static createFrom(source: any = {}) {
	        return new ArticleDetails(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.translatedTitle = source["translatedTitle"];
	        this.heroImage = source["heroImage"];
	        this.author = source["author"];
	        this.date = source["date"];
	        this.displayDate = source["displayDate"];
	        this.tag = source["tag"];
	        this.kind = source["kind"];
	        this.link = source["link"];
	        this.contentHtml = source["contentHtml"];
	        this.translatedHtml = source["translatedHtml"];
	    }
	}
	export class CacheInfo {
	    sizeBytes: number;
	    formatted: string;
	    fileCount: number;
	
	    static createFrom(source: any = {}) {
	        return new CacheInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sizeBytes = source["sizeBytes"];
	        this.formatted = source["formatted"];
	        this.fileCount = source["fileCount"];
	    }
	}
	export class ContentItem {
	    filename: string;
	    name: string;
	    version: string;
	    type: string;
	    enabled: boolean;
	    size: number;
	    modTime: number;
	    author: string;
	    authorAvatar: string;
	    iconUrl: string;
	    sha1: string;
	    hasUpdate: boolean;
	    updateVer: string;
	    updateUrl: string;
	    updateFile: string;
	
	    static createFrom(source: any = {}) {
	        return new ContentItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filename = source["filename"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.type = source["type"];
	        this.enabled = source["enabled"];
	        this.size = source["size"];
	        this.modTime = source["modTime"];
	        this.author = source["author"];
	        this.authorAvatar = source["authorAvatar"];
	        this.iconUrl = source["iconUrl"];
	        this.sha1 = source["sha1"];
	        this.hasUpdate = source["hasUpdate"];
	        this.updateVer = source["updateVer"];
	        this.updateUrl = source["updateUrl"];
	        this.updateFile = source["updateFile"];
	    }
	}
	export class JavaUpdateInfo {
	    major: number;
	    installedVersion: string;
	    latestVersion: string;
	    updateAvailable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new JavaUpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.major = source["major"];
	        this.installedVersion = source["installedVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.updateAvailable = source["updateAvailable"];
	    }
	}
	export class LoaderVersionEntry {
	    version: string;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new LoaderVersionEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.label = source["label"];
	    }
	}
	export class ModItem {
	    filename: string;
	    name: string;
	    enabled: boolean;
	    size: number;
	    modTime: number;
	
	    static createFrom(source: any = {}) {
	        return new ModItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filename = source["filename"];
	        this.name = source["name"];
	        this.enabled = source["enabled"];
	        this.size = source["size"];
	        this.modTime = source["modTime"];
	    }
	}
	export class ModpackVersionItem {
	    id: string;
	    name: string;
	    version_number: string;
	    game_versions: string[];
	    loaders: string[];
	    download_url: string;
	    file_name: string;
	    file_size: number;
	    date_published: string;
	    changelog: string;
	
	    static createFrom(source: any = {}) {
	        return new ModpackVersionItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.version_number = source["version_number"];
	        this.game_versions = source["game_versions"];
	        this.loaders = source["loaders"];
	        this.download_url = source["download_url"];
	        this.file_name = source["file_name"];
	        this.file_size = source["file_size"];
	        this.date_published = source["date_published"];
	        this.changelog = source["changelog"];
	    }
	}
	export class ModpackItem {
	    id: string;
	    source: string;
	    title: string;
	    slug: string;
	    author: string;
	    description: string;
	    icon_url: string;
	    banner_url: string;
	    downloads: number;
	    follows: number;
	    categories: string[];
	    game_versions: string[];
	    loaders: string[];
	    date_modified: string;
	
	    static createFrom(source: any = {}) {
	        return new ModpackItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.source = source["source"];
	        this.title = source["title"];
	        this.slug = source["slug"];
	        this.author = source["author"];
	        this.description = source["description"];
	        this.icon_url = source["icon_url"];
	        this.banner_url = source["banner_url"];
	        this.downloads = source["downloads"];
	        this.follows = source["follows"];
	        this.categories = source["categories"];
	        this.game_versions = source["game_versions"];
	        this.loaders = source["loaders"];
	        this.date_modified = source["date_modified"];
	    }
	}
	export class ModpackDetails {
	    item: ModpackItem;
	    versions: ModpackVersionItem[];
	    body: string;
	
	    static createFrom(source: any = {}) {
	        return new ModpackDetails(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.item = this.convertValues(source["item"], ModpackItem);
	        this.versions = this.convertValues(source["versions"], ModpackVersionItem);
	        this.body = source["body"];
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
	
	
	export class ModrinthHit {
	    project_id: string;
	    slug: string;
	    title: string;
	    description: string;
	    categories: string[];
	    client_side: string;
	    server_side: string;
	    icon_url: string;
	    author: string;
	    downloads: number;
	    follows: number;
	    versions: string[];
	    gallery: string[];
	    date_modified: string;
	
	    static createFrom(source: any = {}) {
	        return new ModrinthHit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.project_id = source["project_id"];
	        this.slug = source["slug"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.categories = source["categories"];
	        this.client_side = source["client_side"];
	        this.server_side = source["server_side"];
	        this.icon_url = source["icon_url"];
	        this.author = source["author"];
	        this.downloads = source["downloads"];
	        this.follows = source["follows"];
	        this.versions = source["versions"];
	        this.gallery = source["gallery"];
	        this.date_modified = source["date_modified"];
	    }
	}
	export class ModrinthSearchResponse {
	    hits: ModrinthHit[];
	    offset: number;
	    limit: number;
	    total_hits: number;
	
	    static createFrom(source: any = {}) {
	        return new ModrinthSearchResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hits = this.convertValues(source["hits"], ModrinthHit);
	        this.offset = source["offset"];
	        this.limit = source["limit"];
	        this.total_hits = source["total_hits"];
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
	export class NewsEntry {
	    title: string;
	    tag: string;
	    kind: string;
	    date: string;
	    displayDate: string;
	    text: string;
	    image: string;
	    link: string;
	
	    static createFrom(source: any = {}) {
	        return new NewsEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.tag = source["tag"];
	        this.kind = source["kind"];
	        this.date = source["date"];
	        this.displayDate = source["displayDate"];
	        this.text = source["text"];
	        this.image = source["image"];
	        this.link = source["link"];
	    }
	}
	export class ResolvedDependency {
	    projectId: string;
	    projectSlug: string;
	    projectTitle: string;
	    iconUrl: string;
	    dependencyType: string;
	    versionId: string;
	    versionNumber: string;
	    fileName: string;
	    downloadUrl: string;
	    alreadyInstalled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ResolvedDependency(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.projectId = source["projectId"];
	        this.projectSlug = source["projectSlug"];
	        this.projectTitle = source["projectTitle"];
	        this.iconUrl = source["iconUrl"];
	        this.dependencyType = source["dependencyType"];
	        this.versionId = source["versionId"];
	        this.versionNumber = source["versionNumber"];
	        this.fileName = source["fileName"];
	        this.downloadUrl = source["downloadUrl"];
	        this.alreadyInstalled = source["alreadyInstalled"];
	    }
	}
	export class WorldItem {
	    name: string;
	    folderName: string;
	    size: number;
	    lastPlayed: number;
	    iconBase64: string;
	
	    static createFrom(source: any = {}) {
	        return new WorldItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.folderName = source["folderName"];
	        this.size = source["size"];
	        this.lastPlayed = source["lastPlayed"];
	        this.iconBase64 = source["iconBase64"];
	    }
	}

}

export namespace main {
	
	export class CrashReport {
	    filename: string;
	    modTime: number;
	    size: number;
	    summary: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new CrashReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filename = source["filename"];
	        this.modTime = source["modTime"];
	        this.size = source["size"];
	        this.summary = source["summary"];
	        this.content = source["content"];
	    }
	}
	export class DetectedInstance {
	    id: string;
	    name: string;
	    versionId: string;
	    loader: string;
	    loaderVersion?: string;
	    path: string;
	    icon?: string;
	    modpackSource?: string;
	    modpackId?: string;
	    modpackVersionId?: string;
	    modpackVersionName?: string;
	
	    static createFrom(source: any = {}) {
	        return new DetectedInstance(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.versionId = source["versionId"];
	        this.loader = source["loader"];
	        this.loaderVersion = source["loaderVersion"];
	        this.path = source["path"];
	        this.icon = source["icon"];
	        this.modpackSource = source["modpackSource"];
	        this.modpackId = source["modpackId"];
	        this.modpackVersionId = source["modpackVersionId"];
	        this.modpackVersionName = source["modpackVersionName"];
	    }
	}
	export class DetectedLauncher {
	    id: string;
	    name: string;
	    basePath: string;
	    found: boolean;
	    instances: DetectedInstance[];
	
	    static createFrom(source: any = {}) {
	        return new DetectedLauncher(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.basePath = source["basePath"];
	        this.found = source["found"];
	        this.instances = this.convertValues(source["instances"], DetectedInstance);
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
	export class FilePickResult {
	    filePath: string;
	    dataUrl: string;
	    fileName: string;
	
	    static createFrom(source: any = {}) {
	        return new FilePickResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filePath = source["filePath"];
	        this.dataUrl = source["dataUrl"];
	        this.fileName = source["fileName"];
	    }
	}
	export class Instance {
	    id: string;
	    name: string;
	    versionId: string;
	    loader: string;
	    loaderVersion: string;
	    dir: string;
	    created: number;
	    icon?: string;
	    group?: string;
	    sortOrder?: number;
	    playTime?: number;
	    playTimeToday?: number;
	    lastPlayDay?: string;
	    lastPlayed?: number;
	    serverAddress?: string;
	    modpackSource?: string;
	    modpackId?: string;
	    modpackVersionId?: string;
	    modpackVersionName?: string;
	    ramMb?: number;
	    javaPath?: string;
	    jvmPreset?: string;
	    jvmArgs?: string;
	    useCustomWindow?: boolean;
	    fullscreen?: boolean;
	    windowWidth?: number;
	    windowHeight?: number;
	
	    static createFrom(source: any = {}) {
	        return new Instance(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.versionId = source["versionId"];
	        this.loader = source["loader"];
	        this.loaderVersion = source["loaderVersion"];
	        this.dir = source["dir"];
	        this.created = source["created"];
	        this.icon = source["icon"];
	        this.group = source["group"];
	        this.sortOrder = source["sortOrder"];
	        this.playTime = source["playTime"];
	        this.playTimeToday = source["playTimeToday"];
	        this.lastPlayDay = source["lastPlayDay"];
	        this.lastPlayed = source["lastPlayed"];
	        this.serverAddress = source["serverAddress"];
	        this.modpackSource = source["modpackSource"];
	        this.modpackId = source["modpackId"];
	        this.modpackVersionId = source["modpackVersionId"];
	        this.modpackVersionName = source["modpackVersionName"];
	        this.ramMb = source["ramMb"];
	        this.javaPath = source["javaPath"];
	        this.jvmPreset = source["jvmPreset"];
	        this.jvmArgs = source["jvmArgs"];
	        this.useCustomWindow = source["useCustomWindow"];
	        this.fullscreen = source["fullscreen"];
	        this.windowWidth = source["windowWidth"];
	        this.windowHeight = source["windowHeight"];
	    }
	}
	export class JavaRuntimeStatus {
	    major: number;
	    found: boolean;
	    path: string;
	    managed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new JavaRuntimeStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.major = source["major"];
	        this.found = source["found"];
	        this.path = source["path"];
	        this.managed = source["managed"];
	    }
	}
	export class ModpackUpdateInfo {
	    hasUpdate: boolean;
	    currentVersion: string;
	    latestVersion: string;
	    latestVersionId: string;
	    downloadUrl: string;
	    changelog: string;
	    releaseDate: string;
	
	    static createFrom(source: any = {}) {
	        return new ModpackUpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasUpdate = source["hasUpdate"];
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.latestVersionId = source["latestVersionId"];
	        this.downloadUrl = source["downloadUrl"];
	        this.changelog = source["changelog"];
	        this.releaseDate = source["releaseDate"];
	    }
	}
	export class ScreenshotItem {
	    filename: string;
	    size: number;
	    modTime: number;
	    dataUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new ScreenshotItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filename = source["filename"];
	        this.size = source["size"];
	        this.modTime = source["modTime"];
	        this.dataUrl = source["dataUrl"];
	    }
	}
	export class Settings {
	    username: string;
	    ramMb: number;
	    resolution: string;
	    javaPath: string;
	    jvmPreset: string;
	    extraJvmArgs: string;
	    closeOnLaunch: boolean;
	    showSnapshots: boolean;
	    discordRpc: boolean;
	    discordAppId: string;
	    autoUpdate: boolean;
	    launcherUpdates: boolean;
	    autoCleanCache: boolean;
	    selectedVersion: string;
	    activeInstance: string;
	    language: string;
	    groups?: string[];
	    centerWindow: boolean;
	    windowCustom: boolean;
	    fullscreen: boolean;
	    windowWidth: number;
	    windowHeight: number;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.username = source["username"];
	        this.ramMb = source["ramMb"];
	        this.resolution = source["resolution"];
	        this.javaPath = source["javaPath"];
	        this.jvmPreset = source["jvmPreset"];
	        this.extraJvmArgs = source["extraJvmArgs"];
	        this.closeOnLaunch = source["closeOnLaunch"];
	        this.showSnapshots = source["showSnapshots"];
	        this.discordRpc = source["discordRpc"];
	        this.discordAppId = source["discordAppId"];
	        this.autoUpdate = source["autoUpdate"];
	        this.launcherUpdates = source["launcherUpdates"];
	        this.autoCleanCache = source["autoCleanCache"];
	        this.selectedVersion = source["selectedVersion"];
	        this.activeInstance = source["activeInstance"];
	        this.language = source["language"];
	        this.groups = source["groups"];
	        this.centerWindow = source["centerWindow"];
	        this.windowCustom = source["windowCustom"];
	        this.fullscreen = source["fullscreen"];
	        this.windowWidth = source["windowWidth"];
	        this.windowHeight = source["windowHeight"];
	    }
	}
	export class VersionEntry {
	    id: string;
	    type: string;
	    installed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new VersionEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.installed = source["installed"];
	    }
	}
	export class StatePayload {
	    settings: Settings;
	    versions: VersionEntry[];
	    latestRelease: string;
	    latestSnapshot: string;
	    versionsErr: string;
	    launcherVer: string;
	    dataDir: string;
	    accounts: auth.Account[];
	    activeId: string;
	    instances: Instance[];
	    activeInstance: string;
	
	    static createFrom(source: any = {}) {
	        return new StatePayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.settings = this.convertValues(source["settings"], Settings);
	        this.versions = this.convertValues(source["versions"], VersionEntry);
	        this.latestRelease = source["latestRelease"];
	        this.latestSnapshot = source["latestSnapshot"];
	        this.versionsErr = source["versionsErr"];
	        this.launcherVer = source["launcherVer"];
	        this.dataDir = source["dataDir"];
	        this.accounts = this.convertValues(source["accounts"], auth.Account);
	        this.activeId = source["activeId"];
	        this.instances = this.convertValues(source["instances"], Instance);
	        this.activeInstance = source["activeInstance"];
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
	export class UpdateInfo {
	    currentVersion: string;
	    latestVersion: string;
	    updateAvailable: boolean;
	    releaseUrl: string;
	    releaseNotes: string;
	    publishedAt: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.updateAvailable = source["updateAvailable"];
	        this.releaseUrl = source["releaseUrl"];
	        this.releaseNotes = source["releaseNotes"];
	        this.publishedAt = source["publishedAt"];
	        this.error = source["error"];
	    }
	}
	export class VerifyResult {
	    totalChecked: number;
	    repaired: number;
	    failed: number;
	    details: string[];
	
	    static createFrom(source: any = {}) {
	        return new VerifyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalChecked = source["totalChecked"];
	        this.repaired = source["repaired"];
	        this.failed = source["failed"];
	        this.details = source["details"];
	    }
	}

}

