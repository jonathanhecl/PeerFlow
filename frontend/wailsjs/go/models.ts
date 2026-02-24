export namespace network {
	
	export class NetworkInfo {
	    peerId: string;
	    localIPs: string[];
	    tcpPort: number;
	    peers: number;
	
	    static createFrom(source: any = {}) {
	        return new NetworkInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.peerId = source["peerId"];
	        this.localIPs = source["localIPs"];
	        this.tcpPort = source["tcpPort"];
	        this.peers = source["peers"];
	    }
	}

}

