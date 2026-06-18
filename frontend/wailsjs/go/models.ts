export namespace render {
	
	export class FrontMatter {
	    Title: string;
	    Date: string;
	    Tags: string[];
	
	    static createFrom(source: any = {}) {
	        return new FrontMatter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Title = source["Title"];
	        this.Date = source["Date"];
	        this.Tags = source["Tags"];
	    }
	}
	export class DocumentPayload {
	    HTML: string;
	    FrontMatter: FrontMatter;
	    WordCount: number;
	    ReadingMins: number;
	    NeedsMermaid: boolean;
	    NeedsMath: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DocumentPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.HTML = source["HTML"];
	        this.FrontMatter = this.convertValues(source["FrontMatter"], FrontMatter);
	        this.WordCount = source["WordCount"];
	        this.ReadingMins = source["ReadingMins"];
	        this.NeedsMermaid = source["NeedsMermaid"];
	        this.NeedsMath = source["NeedsMath"];
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

