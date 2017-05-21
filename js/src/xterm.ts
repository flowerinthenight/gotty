import * as bare from "xterm";

bare.loadAddon("fit");

export class TermXterm {
    elem: HTMLElement;

    message: HTMLElement;
    messageTimeout: number;
    messageTimer: number;

    term: bare;
    resizeListener: () => void;

    constructor(elem: HTMLElement) {
        this.elem = elem;
        this.term = new bare();

        this.message = elem.ownerDocument.createElement("div");
        this.message.style.cssText = (
            'border-radius: 15px;' +
            'font-size: xx-large;' +
            'opacity: 0.75;' +
            'padding: 0.2em 0.5em 0.2em 0.5em;' +
            'position: absolute;' +
            'top: 100px;' +
            'width: 100px;' +
            'height: 100px;' +
            'background: red;' +
            '-webkit-user-select: none;' +
            '-webkit-transition: opacity 180ms ease-in;' +
            '-moz-user-select: none;' +
            '-moz-transition: opacity 180ms ease-in; z-index: 1000;');
        this.messageTimeout = 2000;


        let resizing;
        this.resizeListener = () => {
            if (resizing != null) {
                clearTimeout(resizing);
            }
            resizing = setTimeout(() => {
                this.term.fit();
                this.term.reset(); // remove unnecessary scroll back
            }, 200);
        };

        this.term.on("open", () => {
            this.resizeListener();
            window.addEventListener("resize", this.resizeListener);
        });


        this.term.open(elem, true);
    };

    write(data: string) {
        this.term.write(data);
    };

    showMessage(message: string, timeout: number) {
        console.log(message);
        this.message.textContent = message;
        this.elem.appendChild(this.message);

        if (timeout > 0) {
            if (this.messageTimer) {
                clearTimeout(this.messageTimer);
            }
            this.messageTimer = setTimeout(() => {
                this.elem.removeChild(this.message);
            }, timeout);
        }
    };

    setWindowTitle(title: string) {
        document.title = title;
    };

    setPreferences(value: object) {
    };

    onInput(callback: (input: string) => void) {
        this.term.on("data", (data) => {
            callback(data);
        });

    };

    onResize(callback: (colmuns: number, rows: number) => void) {
        this.term.on("resize", (data) => {
            this.showMessage(String(data.cols) + " x " + String(data.rows), this.messageTimeout);
            callback(data.cols, data.rows);
        });
    };

    close() {
        window.removeEventListener("resize", this.resizeListener);
    }

}
