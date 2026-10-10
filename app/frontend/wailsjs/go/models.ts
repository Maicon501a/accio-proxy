export namespace gateway {
	
	export class ModelEntry {
	    id: string;
	    code: string;
	    name: string;
	    owned_by: string;
	    reason: boolean;
	    efforts?: string[];
	    default_effort?: string;
	    context_window: number;
	    multimodal: boolean;
	    free_use: boolean;
	    usage_multiple: number;
	
	    static createFrom(source: any = {}) {
	        return new ModelEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.code = source["code"];
	        this.name = source["name"];
	        this.owned_by = source["owned_by"];
	        this.reason = source["reason"];
	        this.efforts = source["efforts"];
	        this.default_effort = source["default_effort"];
	        this.context_window = source["context_window"];
	        this.multimodal = source["multimodal"];
	        this.free_use = source["free_use"];
	        this.usage_multiple = source["usage_multiple"];
	    }
	}

}

