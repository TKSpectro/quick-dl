const PORT = 9778;

/** @type{WebSocket} */
let ws;

let reconnectInterval = 1000;
let reconnectAttempts = 0;

/**
 *
 * @param {{command:string}} message
 */
async function sendToFE(message) {
    try {
        await browser.runtime.sendMessage(message);
    } catch (error) {
        // console.error('Failed to send message to FE:', error);
    }
}

async function handleError(error, key) {
    try {
        console.error('Error:', key, error);
        await browser.runtime.sendMessage({ command: 'error', error });
    } catch (error) {
        // console.error('Failed to send error to FE:', error);
    }
}

function connectWS() {
    if (ws) {
        ws.close();
    }

    console.info('Connecting to the server');
    ws = new WebSocket(`ws://localhost:${PORT}/ws`);

    ws.onopen = () => {
        console.info('Connected to the server');
    };

    ws.onmessage = async (event) => {
        const json = JSON.parse(event.data);
        switch (json.type) {
            case 'choose_path': {
                browser.tabs.query({ active: true, currentWindow: true }).then(async (tabs) => {
                    await browser.runtime.sendMessage({
                        command: 'choose_path',
                        url: tabs[0].url,
                        paths: json.paths,
                    });
                });
                break;
            }
            case 'download-started': {
                browser.browserAction.setBadgeText({ text: '...' });
                break;
            }
            case 'download-finished': {
                browser.browserAction.setBadgeText({ text: '' });
                const uuid = crypto.randomUUID();

                browser.notifications.create({
                    id: uuid,
                    type: 'basic',
                    iconUrl: browser.extension.getURL('icons/link-48.png'),
                    title: `quick-dl finished downloading ${json.title}`,
                    message: `Downloaded to ${json.path}`,
                });

                setTimeout(() => {
                    browser.notifications.clear(uuid);
                }, 5000);
                break;
            }
            case 'error': {
                browser.browserAction.setBadgeText({ text: 'ERR' });

                await handleError(json.message, json.error);
                break;
            }
            default: {
                await handleError('Unknown message type: ' + json.type);
                break;
            }
        }
    };

    ws.onclose = () => {
        console.info('Disconnected from the server');
    };

    ws.onerror = async (error) => {
        await handleError(error);
    };
}

console.info('Starting the background script');
connectWS();

setInterval(async () => {
    if (ws.readyState === WebSocket.CLOSED) {
        reconnectAttempts++;
        console.info('Reconnecting to the server. ReconnectAttempts: ' + reconnectAttempts);
        connectWS();
    } else if (ws.readyState === WebSocket.CONNECTING) {
        console.debug(`Still trying to connect to the server for attempt ${reconnectAttempts}`);

        // Increase the interval for each 5 attempts capping at 60 seconds
        if (reconnectAttempts % 5 === 0 && reconnectInterval < 60000) {
            reconnectInterval *= 2;
        }

        reconnectAttempts++;
    } else if (ws.readyState === WebSocket.CLOSING) {
        // Do nothing
    } else {
        reconnectAttempts = 0;
        reconnectInterval = 1000;
    }

    await sendToFE({ command: 'ws-connection', state: ws.readyState });
}, reconnectInterval);

browser.runtime.onMessage.addListener(async (message) => {
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
                        audioOnly: message.audioOnly,
                    },
                })
            );
            break;
        }
        case 'ws-reconnect': {
            console.info('Reconnecting to the server');
            connectWS();
            break;
        }
        default: {
            await handleError('Unknown command: ' + message.command);
            break;
        }
    }
});
