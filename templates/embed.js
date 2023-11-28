(function() {
    var scriptElement = document.currentScript;

    // Extract the event ID from the script's src attribute
    var scriptSrc = scriptElement.src;
    var host = new URL(scriptSrc).origin;

    // Extract the event parameter from the script's src attribute
    var eventParam = scriptSrc.substring(scriptSrc.lastIndexOf('/') + 1);

    var width = scriptElement.getAttribute('width') || '100%';
    var height = scriptElement.getAttribute('height') || 'auto';
    var iframe = document.createElement('iframe');
    iframe.src = host + '/' + eventParam + '?embed=yes';

    // Basic styles
    iframe.style.width = width;
    iframe.style.height = height;
    // iframe.style.border = '2px solid #b6b6b6';
    // iframe.style.borderRadius = '10px';

    // Add a class to easily permit overwriting the styles
    iframe.classList.add("nostr-embedded")
    
    const myCSSClass = `
    .nostr-embedded {
        border: 2px solid #C9C9C9;
        border-radius: 10px;
    }
    @media (prefers-color-scheme: dark) {
        .nostr-embedded {
            border-color: #393939; /* Adjust the color for dark mode */
        }
    }
    `;

    const styleElement = document.createElement('style');
    styleElement.textContent = myCSSClass;
    scriptElement.parentNode.insertBefore(styleElement, scriptElement.nextSibling);
    scriptElement.parentNode.insertBefore(iframe, scriptElement.nextSibling);

    // Listen for messages from the iframe
    window.addEventListener('message', function(event) {
        // Check if the 'height' attribute is explicitly set
        if (!scriptElement.hasAttribute('height')) {
            // Calculate the maximum height based on 50% of the viewport height
            var maxViewportHeight = window.innerHeight * 0.5;

            // Adjust the height of the iframe based on the received content height
            var receivedHeight = Math.min(event.data.height, maxViewportHeight);
            iframe.style.height = receivedHeight + 'px';

            if (receivedHeight < event.data.height) {
                iframe.contentWindow.postMessage({showGradient: true}, '*');
            }
        }

        const darkModeQuery = window.matchMedia('(prefers-color-scheme: dark)');
        if (darkModeQuery.matches) {
            // Dark mode is preferred
            iframe.contentWindow.postMessage({setDarkMode: true}, '*');
        }
    });
})();
