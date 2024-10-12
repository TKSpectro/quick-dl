const PORT = 9778;

/** @type{WebSocket} */
let ws;
connectWS();

let reconnectInterval = 5000;
let reconnectAttempts = 0;

function connectWS() {
    ws = new WebSocket(`ws://localhost:${PORT}/ws`);

    ws.onopen = () => {
        console.info('Connected to the server');
    };

    ws.onmessage = async (event) => {
        const json = JSON.parse(event.data);
        switch (json.type) {
            case 'choose_path':
                browser.tabs.query({ active: true, currentWindow: true }).then((tabs) => {
                    browser.runtime.sendMessage({
                        command: 'choose_path',
                        url: tabs[0].url,
                        paths: json.paths,
                    });
                });
                break;
            default:
                console.error('Unknown message type:', json.type);
                break;
        }
    };

    ws.onclose = () => {
        console.info('Disconnected from the server');
    };

    ws.onerror = (error) => {
        console.error('Error:', error);
    };
}

setInterval(() => {
    if (ws.readyState === WebSocket.CLOSED) {
        reconnectAttempts++;
        console.info('Reconnecting to the server. ReconnectAttempts: ' + reconnectAttempts);
        connectWS();
    } else {
        reconnectAttempts = 0;
    }
}, reconnectInterval);

browser.runtime.onMessage.addListener((message) => {
    switch (message.command) {
        case 'download': {
            ws.send(JSON.stringify({ type: 'download', data: { url: message.url } }));
            break;
        }
        case 'picked_path': {
            ws.send(
                JSON.stringify({
                    type: 'picked_path',
                    data: {
                        url: message.url,
                        id: message.id,
                    },
                })
            );
            break;
        }
        default: {
            console.error('Unknown command:', message.command);
            break;
        }
    }
});
