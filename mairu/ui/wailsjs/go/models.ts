export namespace agent {
	
	export class SavedPart {
	    type: string;
	    text?: string;
	    func_name?: string;
	    func_args?: Record<string, any>;
	    func_resp?: Record<string, any>;
	    language?: number;
	    code?: string;
	    outcome?: number;
	    output?: string;
	
	    static createFrom(source: any = {}) {
	        return new SavedPart(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.text = source["text"];
	        this.func_name = source["func_name"];
	        this.func_args = source["func_args"];
	        this.func_resp = source["func_resp"];
	        this.language = source["language"];
	        this.code = source["code"];
	        this.outcome = source["outcome"];
	        this.output = source["output"];
	    }
	}
	export class SavedMessage {
	    role: string;
	    parts: SavedPart[];
	
	    static createFrom(source: any = {}) {
	        return new SavedMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.parts = this.convertValues(source["parts"], SavedPart);
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

}

export namespace contextsrv {
	
	export class ContextCreateInput {
	    URI: string;
	    Project: string;
	    ParentURI?: string;
	    Name: string;
	    Abstract: string;
	    Overview: string;
	    Content: string;
	    Metadata: number[];
	    ModerationStatus: string;
	    ModerationReasons: string[];
	    ReviewRequired: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ContextCreateInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.URI = source["URI"];
	        this.Project = source["Project"];
	        this.ParentURI = source["ParentURI"];
	        this.Name = source["Name"];
	        this.Abstract = source["Abstract"];
	        this.Overview = source["Overview"];
	        this.Content = source["Content"];
	        this.Metadata = source["Metadata"];
	        this.ModerationStatus = source["ModerationStatus"];
	        this.ModerationReasons = source["ModerationReasons"];
	        this.ReviewRequired = source["ReviewRequired"];
	    }
	}
	export class ContextNode {
	    uri: string;
	    project: string;
	    parent_uri?: string;
	    name: string;
	    abstract: string;
	    overview?: string;
	    content?: string;
	    metadata?: Record<string, any>;
	    moderation_status: string;
	    moderation_reasons: string[];
	    review_required: boolean;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	
	    static createFrom(source: any = {}) {
	        return new ContextNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uri = source["uri"];
	        this.project = source["project"];
	        this.parent_uri = source["parent_uri"];
	        this.name = source["name"];
	        this.abstract = source["abstract"];
	        this.overview = source["overview"];
	        this.content = source["content"];
	        this.metadata = source["metadata"];
	        this.moderation_status = source["moderation_status"];
	        this.moderation_reasons = source["moderation_reasons"];
	        this.review_required = source["review_required"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
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
	export class ContextUpdateInput {
	    URI: string;
	    Name: string;
	    Abstract: string;
	    Overview: string;
	    Content: string;
	
	    static createFrom(source: any = {}) {
	        return new ContextUpdateInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.URI = source["URI"];
	        this.Name = source["Name"];
	        this.Abstract = source["Abstract"];
	        this.Overview = source["Overview"];
	        this.Content = source["Content"];
	    }
	}
	export class Memory {
	    id: string;
	    project: string;
	    content: string;
	    category: string;
	    owner: string;
	    importance: number;
	    retrieval_count: number;
	    feedback_count: number;
	    // Go type: time
	    last_retrieved_at?: any;
	    moderation_status: string;
	    moderation_reasons: string[];
	    review_required: boolean;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	
	    static createFrom(source: any = {}) {
	        return new Memory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.project = source["project"];
	        this.content = source["content"];
	        this.category = source["category"];
	        this.owner = source["owner"];
	        this.importance = source["importance"];
	        this.retrieval_count = source["retrieval_count"];
	        this.feedback_count = source["feedback_count"];
	        this.last_retrieved_at = this.convertValues(source["last_retrieved_at"], null);
	        this.moderation_status = source["moderation_status"];
	        this.moderation_reasons = source["moderation_reasons"];
	        this.review_required = source["review_required"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
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
	export class MemoryCreateInput {
	    Project: string;
	    Content: string;
	    Category: string;
	    Owner: string;
	    Importance: number;
	    Metadata: number[];
	    ModerationStatus: string;
	    ModerationReasons: string[];
	    ReviewRequired: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MemoryCreateInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Project = source["Project"];
	        this.Content = source["Content"];
	        this.Category = source["Category"];
	        this.Owner = source["Owner"];
	        this.Importance = source["Importance"];
	        this.Metadata = source["Metadata"];
	        this.ModerationStatus = source["ModerationStatus"];
	        this.ModerationReasons = source["ModerationReasons"];
	        this.ReviewRequired = source["ReviewRequired"];
	    }
	}
	export class MemoryUpdateInput {
	    ID: string;
	    Content: string;
	    Category: string;
	    Owner: string;
	    Importance: number;
	
	    static createFrom(source: any = {}) {
	        return new MemoryUpdateInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Content = source["Content"];
	        this.Category = source["Category"];
	        this.Owner = source["Owner"];
	        this.Importance = source["Importance"];
	    }
	}
	export class ModerationEvent {
	    id: number;
	    entity_type: string;
	    entity_id: string;
	    project: string;
	    decision: string;
	    reasons: string[];
	    review_status: string;
	    reviewer_decision?: string;
	    review_required: boolean;
	    policy_version: string;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    reviewed_at?: any;
	    reviewer?: string;
	
	    static createFrom(source: any = {}) {
	        return new ModerationEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.entity_type = source["entity_type"];
	        this.entity_id = source["entity_id"];
	        this.project = source["project"];
	        this.decision = source["decision"];
	        this.reasons = source["reasons"];
	        this.review_status = source["review_status"];
	        this.reviewer_decision = source["reviewer_decision"];
	        this.review_required = source["review_required"];
	        this.policy_version = source["policy_version"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.reviewed_at = this.convertValues(source["reviewed_at"], null);
	        this.reviewer = source["reviewer"];
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
	export class ModerationReviewInput {
	    EventID: number;
	    Decision: string;
	    Reviewer: string;
	    Notes: string;
	    UpdatedBy: string;
	
	    static createFrom(source: any = {}) {
	        return new ModerationReviewInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.EventID = source["EventID"];
	        this.Decision = source["Decision"];
	        this.Reviewer = source["Reviewer"];
	        this.Notes = source["Notes"];
	        this.UpdatedBy = source["UpdatedBy"];
	    }
	}
	export class SearchOptions {
	    query: string;
	    project: string;
	    store: string;
	    topK: number;
	    minScore: number;
	    highlight: boolean;
	    fieldBoosts: Record<string, number>;
	    fuzziness: string;
	    phraseBoost: number;
	    weightVector: number;
	    weightKeyword: number;
	    weightRecency: number;
	    weightImportance: number;
	    recencyScale: string;
	    recencyDecay: number;
	
	    static createFrom(source: any = {}) {
	        return new SearchOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.query = source["query"];
	        this.project = source["project"];
	        this.store = source["store"];
	        this.topK = source["topK"];
	        this.minScore = source["minScore"];
	        this.highlight = source["highlight"];
	        this.fieldBoosts = source["fieldBoosts"];
	        this.fuzziness = source["fuzziness"];
	        this.phraseBoost = source["phraseBoost"];
	        this.weightVector = source["weightVector"];
	        this.weightKeyword = source["weightKeyword"];
	        this.weightRecency = source["weightRecency"];
	        this.weightImportance = source["weightImportance"];
	        this.recencyScale = source["recencyScale"];
	        this.recencyDecay = source["recencyDecay"];
	    }
	}
	export class Skill {
	    id: string;
	    project: string;
	    name: string;
	    description: string;
	    moderation_status: string;
	    moderation_reasons: string[];
	    review_required: boolean;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	
	    static createFrom(source: any = {}) {
	        return new Skill(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.project = source["project"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.moderation_status = source["moderation_status"];
	        this.moderation_reasons = source["moderation_reasons"];
	        this.review_required = source["review_required"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
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
	export class SkillCreateInput {
	    Project: string;
	    Name: string;
	    Description: string;
	    Metadata: number[];
	    ModerationStatus: string;
	    ModerationReasons: string[];
	    ReviewRequired: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SkillCreateInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Project = source["Project"];
	        this.Name = source["Name"];
	        this.Description = source["Description"];
	        this.Metadata = source["Metadata"];
	        this.ModerationStatus = source["ModerationStatus"];
	        this.ModerationReasons = source["ModerationReasons"];
	        this.ReviewRequired = source["ReviewRequired"];
	    }
	}
	export class SkillUpdateInput {
	    ID: string;
	    Name: string;
	    Description: string;
	
	    static createFrom(source: any = {}) {
	        return new SkillUpdateInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Name = source["Name"];
	        this.Description = source["Description"];
	    }
	}
	export class VibeMutationOp {
	    op: string;
	    target?: string;
	    description: string;
	    data: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new VibeMutationOp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.op = source["op"];
	        this.target = source["target"];
	        this.description = source["description"];
	        this.data = source["data"];
	    }
	}
	export class VibeMutationPlan {
	    reasoning: string;
	    operations: VibeMutationOp[];
	
	    static createFrom(source: any = {}) {
	        return new VibeMutationPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.reasoning = source["reasoning"];
	        this.operations = this.convertValues(source["operations"], VibeMutationOp);
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
	export class VibeSearchGroup {
	    store: string;
	    query: string;
	    items: any[];
	
	    static createFrom(source: any = {}) {
	        return new VibeSearchGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.store = source["store"];
	        this.query = source["query"];
	        this.items = source["items"];
	    }
	}
	export class VibeQueryResult {
	    reasoning: string;
	    results: VibeSearchGroup[];
	
	    static createFrom(source: any = {}) {
	        return new VibeQueryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.reasoning = source["reasoning"];
	        this.results = this.convertValues(source["results"], VibeSearchGroup);
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

}

export namespace desktop {
	
	export class WindowState {
	    x: number;
	    y: number;
	    width: number;
	    height: number;
	
	    static createFrom(source: any = {}) {
	        return new WindowState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.x = source["x"];
	        this.y = source["y"];
	        this.width = source["width"];
	        this.height = source["height"];
	    }
	}

}

export namespace keys {
	
	export class Accelerator {
	    Key: string;
	    Modifiers: string[];
	
	    static createFrom(source: any = {}) {
	        return new Accelerator(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Key = source["Key"];
	        this.Modifiers = source["Modifiers"];
	    }
	}

}

export namespace menu {
	
	export class MenuItem {
	    Label: string;
	    Role: number;
	    Accelerator?: keys.Accelerator;
	    Type: string;
	    Disabled: boolean;
	    Hidden: boolean;
	    Checked: boolean;
	    SubMenu?: Menu;
	
	    static createFrom(source: any = {}) {
	        return new MenuItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Label = source["Label"];
	        this.Role = source["Role"];
	        this.Accelerator = this.convertValues(source["Accelerator"], keys.Accelerator);
	        this.Type = source["Type"];
	        this.Disabled = source["Disabled"];
	        this.Hidden = source["Hidden"];
	        this.Checked = source["Checked"];
	        this.SubMenu = this.convertValues(source["SubMenu"], Menu);
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
	export class Menu {
	    Items: MenuItem[];
	
	    static createFrom(source: any = {}) {
	        return new Menu(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Items = this.convertValues(source["Items"], MenuItem);
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

}

