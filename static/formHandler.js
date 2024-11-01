import { validateURL } from "./utils.js";

let controller;

document
	.getElementById("transcriptionForm")
	.addEventListener("submit", async (event) => {
		event.preventDefault();
		const submitButton = document.querySelector('button[type="submit"]');
		const spinner = document.getElementById("spinner");
		const transcriptionStatus = document.getElementById("transcriptionStatus");
		document.getElementById("response").innerText = ""; // Clear response
		document.getElementById("copyButton").classList.add("hidden"); // Hide copy button
		document.getElementById("downloadButton").classList.add("hidden"); // Hide download button
		spinner.classList.remove("hidden"); // Show spinner
		transcriptionStatus.classList.remove("hidden"); // Show transcription status

		submitButton.disabled = true; // Disable the button after sending the request

		const url = document.getElementById("url").value;

		// Validate URL format
		if (!validateURL(url)) {
			document.getElementById("response").innerText =
				"Invalid URL format. Please enter a valid URL.";
			spinner.classList.add("hidden"); // Hide spinner
			transcriptionStatus.classList.add("hidden"); // Hide transcription status
			submitButton.disabled = false; // Re-enable the button
			return;
		}

		// Create a new AbortController instance
		controller = new AbortController();
		const signal = controller.signal;

		try {
			const response = await fetch("/transcribe", {
				method: "POST",
				headers: {
					"Content-Type": "application/x-www-form-urlencoded",
				},
				body: new URLSearchParams({
					url: url,
				}),
				signal: signal, // Pass the signal to the fetch request
			});

			if (!response.ok) {
				const text = await response.text();
				if (response.status === 400) {
					document.getElementById("response").innerText =
						"Invalid URL. Please enter a valid URL.";
				} else if (response.status === 429) {
					document.getElementById("response").innerText =
						"Too many requests. Please try again later.";
				} else {
					document.getElementById("response").innerText =
						"An error occurred while processing your request. Please try again later.";
				}
				return; // Do not throw an error to prevent logging to the console
			}

			const data = await response.json();
			const responseDiv = document.getElementById("response");
			responseDiv.innerText = data.transcription;

			const copyButton = document.getElementById("copyButton");
			copyButton.classList.remove("hidden"); // Show copy button
			copyButton.onclick = () => {
				navigator.clipboard
					.writeText(data.transcription)
					.then(() => {
						alert("Text copied to clipboard");
					})
					.catch(() => {
						document.getElementById("response").innerText =
							"Failed to copy text. Please try again.";
					});
			};

			const downloadButton = document.getElementById("downloadButton");
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
		} catch (error) {
			// Handle the error gracefully without logging to the console
		} finally {
			submitButton.disabled = false; // Re-enable the button
			spinner.classList.add("hidden"); // Hide spinner
			transcriptionStatus.classList.add("hidden"); // Hide transcription status
		}
	});

window.addEventListener("beforeunload", () => {
	if (controller) {
		controller.abort(); // Abort the fetch request if the user leaves the page
	}
});
