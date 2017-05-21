export const protocols = ["webtty"];

export const InputUnknown = '0';
export const Input = '1';
export const Ping = '2';
export const ResizeTerminal = '3';

export const UnknownOutput = '0';
export const Output = '1';
export const Pong = '2';
export const SetWindowTitle = '3';
export const SetPreferences = '4';
export const SetReconnect = '5';


export interface Terminal {
    write(data: string): void;
    showMessage(message: string, timeout: number): void;
    setWindowTitle(title: string): void;
    setPreferences(value: object): void;
    onInput(callback: (input: string) => void): void;
    onResize(callback: (colmuns: number, rows: number) => void): void;
    close(): void;
}

export interface Connection {
    open(): void;
    close(): void;
    send(data: string): void;
    onOpen(callback: () => void): void;
    onReceive(callback: (data: string) => void): void;
    onClose(callback: () => void): void;
}


export interface ConnectionFactory {
    create(): Connection;
}


export class WebTTY {
    term: Terminal;
    connectionFactory: ConnectionFactory;
    args: string;
    authToken: string;
    reconnect: number;


    constructor(term: Terminal, connectionFactory: ConnectionFactory, args: string, authToken: string) {
        this.term = term;
        this.connectionFactory = connectionFactory;
        this.args = args;
        this.authToken = authToken;
    };

    open() {
        const connection = this.connectionFactory.create();

        connection.onOpen(() => {
            connection.send(JSON.stringify(
                {
                    Arguments: this.args,
                    AuthToken: this.authToken,
                }
            ));

            this.term.onResize(
                (columns: number, rows: number) => {
                    connection.send(
                        ResizeTerminal + JSON.stringify(
                            {
                                columns: columns,
                                rows: rows
                            }
                        )
                    );
                }
            );

            this.term.onInput(
                (input: string) => {
                    connection.send(Input + input);
                }
            );
        });

        connection.onReceive((data) => {
            const payload = data.slice(1);
            switch (data[0]) {
                case Output:
                    this.term.write(
                        decodeURIComponent(Array.prototype.map.call(atob(payload), function(c) {
                            return '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2);
                        }).join(''))
                    );

                    break;
                case Pong:
                    break;
                case SetWindowTitle:
                    this.term.setWindowTitle(payload);
                    break;
                case SetPreferences:
                    const preferences = JSON.parse(payload);
                    this.term.setPreferences(preferences);
                    break;
                case SetReconnect:
                    const autoReconnect = JSON.parse(payload);
                    console.log("Enabling reconnect: " + autoReconnect + " seconds")
                    this.reconnect = autoReconnect;
                    break;
            }
        });

        connection.onClose(() => {
            this.term.showMessage("disconnected", 0);
            if (this.reconnect > 0) {
                setTimeout(open, this.reconnect * 1000);
            }
        });

        connection.open();
    };
};
