import { validateURL } from "./utils.js";

document
	.getElementById("transcriptionForm")
	.addEventListener("submit", async (event) => {
		event.preventDefault();

		const form = event.target;
		const submitButton = form.querySelector('button[type="submit"]');
		const statusDiv = document.getElementById("transcriptionStatus");
		const responseDiv = document.getElementById("response");
		const urlInput = document.getElementById("url");
		const url = urlInput.value.trim();

		// Reset UI
		resetUI(responseDiv);

		// Basic validation
		if (!validateURL(url)) {
			displayError(responseDiv, "Please enter a valid YouTube URL");
			return;
		}

		// Show loading state
		submitButton.disabled = true;
		showLoadingStatus(statusDiv, "Processing video...");

		try {
			const formData = new URLSearchParams();
			formData.append("url", url);

			const response = await fetch("/api/transcribe", {
				method: "POST",
				headers: {
					"Content-Type": "application/x-www-form-urlencoded",
				},
				body: formData,
			});

			const responseData = await response.json();

			if (!response.ok) {
				throw new Error(responseData.error || "Failed to process video");
			}

			const videoData = responseData.data;
			// Debugging: Uncomment the line below if needed
			// console.log("Initial response:", videoData);

			if (videoData.status === "completed") {
				showTranscription(videoData, statusDiv, responseDiv);
			} else {
				await pollTranscriptionStatus(videoData.id, statusDiv, responseDiv);
			}
		} catch (error) {
			hideElement(statusDiv);
			displayError(responseDiv, error.message);
		} finally {
			submitButton.disabled = false;
		}
	});

/**
 * Resets the UI elements to their default state.
 * @param {HTMLElement} responseDiv - The DIV to display responses.
 */
function resetUI(responseDiv) {
	responseDiv.innerHTML = "";
	toggleVisibility("copyButton", true);
	toggleVisibility("downloadButton", true);
	toggleVisibility("transcriptionHeader", true);
}

/**
 * Displays an error message in the specified DIV.
 * @param {HTMLElement} container - The container to display the error.
 * @param {string} message - The error message.
 */
function displayError(container, message) {
	container.innerHTML = `
        <div class="bg-red-500 text-white p-4 rounded-md">
            <p>${message}</p>
        </div>
    `;
}

/**
 * Shows a loading status message.
 * @param {HTMLElement} statusDiv - The DIV to show the status.
 * @param {string} message - The status message.
 */
function showLoadingStatus(statusDiv, message) {
	statusDiv.classList.remove("hidden");
	statusDiv.innerHTML = `
        <div class="flex items-center">
            <div class="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-500 mr-2"></div>
            <span>${message}</span>
        </div>
    `;
}

/**
 * Hides the specified element.
 * @param {HTMLElement} element - The element to hide.
 */
function hideElement(element) {
	element.classList.add("hidden");
}

/**
 * Toggles visibility of an element based on the hidden flag.
 * @param {string} elementId - The ID of the element.
 * @param {boolean} hide - Whether to hide the element.
 */
function toggleVisibility(elementId, hide) {
	const element = document.getElementById(elementId);
	if (hide) {
		element.classList.add("hidden");
	} else {
		element.classList.remove("hidden");
	}
}

/**
 * Displays the transcription and sets up action buttons.
 * @param {Object} data - The transcription data.
 * @param {HTMLElement} statusDiv - The DIV showing status.
 * @param {HTMLElement} responseDiv - The DIV to display the transcription.
 */
function showTranscription(data, statusDiv, responseDiv) {
	// Debugging: Uncomment the line below if needed
	// console.log("Showing transcription:", data);

	hideElement(statusDiv);
	toggleVisibility("transcriptionHeader", false);

	responseDiv.innerHTML = `
        <div class="bg-gray-700 p-4 rounded-md">
            <pre class="whitespace-pre-wrap">${escapeHTML(data.transcription || "")}</pre>
        </div>
    `;

	// Enable action buttons
	setupActionButtons(data.transcription || "");
}

/**
 * Escapes HTML characters to prevent XSS attacks.
 * @param {string} unsafe - The unsafe string.
 * @returns {string} - The escaped string.
 */
function escapeHTML(unsafe) {
	return unsafe
		.replace(/&/g, "&amp;")
		.replace(/</g, "&lt;")
		.replace(/>/g, "&gt;")
		.replace(/"/g, "&quot;")
		.replace(/'/g, "&#039;");
}

/**
 * Polls the transcription status with increasing backoff intervals.
 * @param {string} id - The transcription ID.
 * @param {HTMLElement} statusDiv - The DIV showing status.
 * @param {HTMLElement} responseDiv - The DIV to display the transcription.
 */
async function pollTranscriptionStatus(id, statusDiv, responseDiv) {
	const baseInterval = 5000; // 5 seconds in milliseconds
	const maxInterval = 30000; // 30 seconds in milliseconds
	const maxAttempts = 30 * 6; // (30 minutes / 5 seconds) = 360 attempts
	let attempts = 0;

	while (attempts < maxAttempts) {
		attempts++;
		const pollingInterval = Math.min(attempts * baseInterval, maxInterval);

		try {
			const response = await fetch(`/api/transcribe/${id}`);
			const responseData = await response.json();

			if (!response.ok) {
				throw new Error(responseData.error || "Failed to check status");
			}

			const data = responseData.data;
			// Debugging: Uncomment the line below if needed
			// console.log("Poll response:", data);

			if (data.status === "completed") {
				showTranscription(data, statusDiv, responseDiv);
				return;
			}

			if (data.status === "failed") {
				throw new Error(data.error || "Transcription failed");
			}

			// Update status message
			statusDiv.innerHTML = `
                <div class="flex items-center">
                    <div class="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-500 mr-2"></div>
                    <span>Status: ${data.status}</span>
                </div>
            `;

			// Wait before next attempt with increasing backoff
			await delay(pollingInterval);
		} catch (error) {
			hideElement(statusDiv);
			displayError(responseDiv, error.message);
			return;
		}
	}

	// If max attempts reached without completion
	hideElement(statusDiv);
	displayError(responseDiv, "Transcription timed out. Please try again.");
}

/**
 * Returns a promise that resolves after the specified delay.
 * @param {number} ms - The delay in milliseconds.
 * @returns {Promise} - A promise that resolves after the delay.
 */
function delay(ms) {
	return new Promise((resolve) => setTimeout(resolve, ms));
}

/**
 * Sets up the copy and download buttons for the transcription.
 * @param {string} transcription - The transcription text.
 */
function setupActionButtons(transcription) {
	const copyButton = document.getElementById("copyButton");
	const downloadButton = document.getElementById("downloadButton");

	// Show and set up copy button
	toggleVisibility("copyButton", false);
	copyButton.onclick = () => {
		navigator.clipboard.writeText(transcription).catch((err) => {
			console.error("Failed to copy transcription:", err);
		});
	};

	// Show and set up download button
	toggleVisibility("downloadButton", false);
	downloadButton.onclick = () => {
		const blob = new Blob([transcription], { type: "text/plain" });
		const url = URL.createObjectURL(blob);
		const a = document.createElement("a");
		a.href = url;
		a.download = "transcription.txt";
		a.click();
		URL.revokeObjectURL(url);
	};
}
