import { validateURL } from './utils.js';

let controller;

document
    .getElementById("transcriptionForm")
    .addEventListener("submit", (event) => {
        event.preventDefault();
        const submitButton = document.querySelector(
            'button[type="submit"]',
        );
        const spinner = document.getElementById("spinner");
        document.getElementById("response").innerText = ""; // Clear response
        document
            .getElementById("copyButton")
            .classList.add("hidden"); // Hide copy button
        document
            .getElementById("downloadButton")
            .classList.add("hidden"); // Hide download button
        spinner.classList.remove("hidden"); // Show spinner
        const url = document.getElementById("url").value;

        // Validate URL format
        if (!validateURL(url)) {
            document.getElementById("response").innerText =
                "Invalid URL format. Please enter a valid URL.";
            spinner.classList.add("hidden"); // Hide spinner
            return;
        }

        // Create a new AbortController instance
        controller = new AbortController();
        const signal = controller.signal;

        fetch("/transcribe", {
            method: "POST",
            headers: {
                "Content-Type": "application/x-www-form-urlencoded",
            },

            body: new URLSearchParams({
                url: url,
            }),
            signal: signal, // Pass the signal to the fetch request
        })
            .then((response) => {
                if (!response.ok) {
                    return response.text().then((text) => {
                        throw new Error(text);
                    });
                }
                return response.json();
            })
            .then((data) => {
                const responseDiv =
                    document.getElementById("response");
                responseDiv.innerText = data.transcription;

                const copyButton =
                    document.getElementById("copyButton");
                copyButton.classList.remove("hidden"); // Show copy button
                copyButton.onclick = () => {
                    navigator.clipboard
                        .writeText(data.transcription)
                        .then(() => {
                            alert("Text copied to clipboard");
                        })
                        .catch((err) => {
                            alert(`Failed to copy text: ${err}`);
                        });
                };

                const downloadButton =
                    document.getElementById("downloadButton");
                downloadButton.classList.remove("hidden"); // Show download button
                downloadButton.onclick = () => {
                    const blob = new Blob([data.transcription], {
                        type: "text/plain",
                    });
                    const link = document.createElement("a");
                    link.href = URL.createObjectURL(blob);
                    link.download = `${url.replace(/[^a-z0-9]/gi, "_").toLowerCase()}.txt`;
                    link.click();
                };
            })
            .catch((error) => {
                document.getElementById("response").innerText =
                    `Error: ${error.message}`;
            })
            .finally(() => {
                submitButton.disabled = false;
                spinner.classList.add("hidden"); // Hide spinner
            });

        submitButton.disabled = true; // Disable the button after sending the request
    });

window.addEventListener("beforeunload", () => {
    if (controller) {
        controller.abort(); // Abort the fetch request if the user leaves the page
    }
});