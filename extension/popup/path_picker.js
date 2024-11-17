let url = '';

// Proxy for WebSocket connection state to get getter/setter
let wsConnectionState = WebSocket.CLOSED;

document.addEventListener('click', (e) => {
    function download(tabs) {
        browser.runtime.sendMessage({
            command: 'download',
            url: tabs[0].url,
        });
    }

    function reportError(error) {
        console.error(`Could not beastify: ${error}`);
    }

    if (e.target.tagName !== 'BUTTON' || !e.target.closest('#popup-content')) {
        // Ignore when click is not on a button within <div id="popup-content">.
        return;
    }

    switch (e.target.id) {
        case 'loading': {
            break;
        }
        case 'download': {
            const loading = document.getElementById('loading');
            loading.style.display = 'block';

            browser.tabs
                .query({ active: true, currentWindow: true })
                .then(download)
                .catch(reportError);
            break;
        }
        case 'path_select_start': {
            const pathId = document.getElementById('path_select').value;
            const audioOnlyCheckbox = document.getElementById('audio_only');

            browser.runtime.sendMessage({
                command: 'picked_path',
                url: url,
                id: pathId,
                audioOnly: audioOnlyCheckbox.checked,
            });

            localStorage.setItem('audioOnly', audioOnlyCheckbox.checked);

            break;
        }
        case 'reconnect': {
            browser.runtime.sendMessage({
                command: 'ws-reconnect',
            });
            break;
        }
        default: {
            console.error(`Unknown button clicked: ${e.target.id}`);
        }
    }
});

// set audioOnly checkbox to checked if it was previously checked
const audioOnlyCheckbox = document.getElementById('audio_only');
if (localStorage.getItem('audioOnly') === 'true') {
    audioOnlyCheckbox.checked = true;
}

browser.runtime.onMessage.addListener((message) => {
    const downloadOptions = document.getElementById('download-options');
    const pathSelector = document.getElementById('path_select');
    const pathSelectStart = document.getElementById('path_select_start');
    const loading = document.getElementById('loading');

    const error = document.getElementById('error-content');

    switch (message.command) {
        case 'choose_path': {
            url = message.url;

            // remove all children
            while (pathSelector.firstChild) {
                pathSelector.removeChild(pathSelector.firstChild);
            }

            // add new children
            for (let i = 0; i < message.paths.length; i++) {
                const option = document.createElement('option');
                option.value = message.paths[i].id;
                option.text = message.paths[i].name;
                pathSelector.appendChild(option);
            }

            downloadOptions.style.display = 'block';
            pathSelectStart.style.display = 'block';

            loading.style.display = 'none';

            break;
        }
        case 'ws-connection': {
            console.log('WS Connection:', message.state);

            wsConnectionState = message.state;

            const reconnectButton = document.getElementById('reconnect');
            const downloadButton = document.getElementById('download');
            if (wsConnectionState !== WebSocket.OPEN) {
                downloadButton.style.display = 'none';
                reconnectButton.style.display = 'block';
            } else {
                reconnectButton.style.display = 'none';
                downloadButton.style.display = 'block';
            }
            break;
        }
        case 'error': {
            error.textContent = message.error;
            error.style.display = 'block';

            loading.style.display = 'none';

            break;
        }
        default: {
            console.error('Unknown command:', message.command);

            error.textContent = 'An error occurred. Please try again.';
            error.style.display = 'block';

            break;
        }
    }
});
