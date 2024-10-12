let url = '';

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
        case 'download': {
            browser.tabs
                .query({ active: true, currentWindow: true })
                .then(download)
                .catch(reportError);
            break;
        }
        case 'path_select_start': {
            const pathId = document.getElementById('path_select').value;

            browser.runtime.sendMessage({
                command: 'picked_path',
                url: url,
                id: pathId,
            });
            break;
        }
        default: {
            console.error(`Unknown button clicked: ${e.target.id}`);
        }
    }
});

browser.runtime.onMessage.addListener((message) => {
    const pathSelector = document.getElementById('path_select');
    const pathSelectStart = document.getElementById('path_select_start');

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

            pathSelector.style.display = 'block';
            pathSelectStart.style.display = 'block';

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
