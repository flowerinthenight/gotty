import { TermXterm } from "./xterm";
import { WebTTY, protocols } from "./webtty";
import { ConnectionFactory } from "./websocket";


const elem = document.getElementById("terminal")

if (elem !== null) {
    const term = new TermXterm(elem);
    const httpsEnabled = window.location.protocol == "https:";
    const url = (httpsEnabled ? 'wss://' : 'ws://') + window.location.hostname + ":8080" + window.location.pathname + 'ws';
    const args = window.location.search;
    const factory = new ConnectionFactory(url, protocols);
    const wt = new WebTTY(term, factory, args, "");
    wt.open();
};
