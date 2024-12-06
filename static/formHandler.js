import { validateURL } from "./utils.js";

document
	.getElementById("transcriptionForm")
	.addEventListener("submit", async (event) => {
		event.preventDefault();

		const form = event.target;
		const submitButton = form.querySelector('button[type="submit"]');
		const statusDiv = document.getElementById("transcriptionStatus");
		const responseDiv = document.getElementById("response");
		const url = document.getElementById("url").value;

		// Reset UI
		responseDiv.innerHTML = "";
		document.getElementById("copyButton").classList.add("hidden");
		document.getElementById("downloadButton").classList.add("hidden");
		document.getElementById("transcriptionHeader").classList.add("hidden");

		// Basic validation
		if (!validateURL(url)) {
			responseDiv.innerHTML = `
                <div class="bg-red-500 text-white p-4 rounded-md">
                    <p>Please enter a valid YouTube URL</p>
                </div>
            `;
			return;
		}

		// Show loading state
		submitButton.disabled = true;
		statusDiv.classList.remove("hidden");
		statusDiv.innerHTML = `
            <div class="flex items-center">
                <div class="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-500 mr-2"></div>
                <span>Processing video...</span>
            </div>
        `;

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

			const videoId = responseData.data.id;
			if (!videoId) {
				throw new Error("No video ID received from server");
			}

			// Poll for status
			await pollTranscriptionStatus(videoId, statusDiv, responseDiv);
		} catch (error) {
			statusDiv.classList.add("hidden");
			responseDiv.innerHTML = `
                <div class="bg-red-500 text-white p-4 rounded-md">
                    <p>${error.message}</p>
                </div>
            `;
		} finally {
			submitButton.disabled = false;
		}
	});

async function pollTranscriptionStatus(id, statusDiv, responseDiv) {
	const maxAttempts = 30 * 60; // 30 minutes at 1-second intervals
	let attempts = 0;

	while (attempts < maxAttempts) {
		try {
			const response = await fetch(`/api/transcribe/${id}`);
			const responseData = await response.json();

			if (!response.ok) {
				throw new Error(responseData.error || "Failed to check status");
			}

			const data = responseData.data;

			if (data.status === "completed") {
				// Hide status and show result
				statusDiv.classList.add("hidden");
				document
					.getElementById("transcriptionHeader")
					.classList.remove("hidden");

				responseDiv.innerHTML = `
                    <div class="bg-gray-700 p-4 rounded-md">
                        <pre class="whitespace-pre-wrap">${data.transcription}</pre>
                    </div>
                `;

				// Enable action buttons
				setupActionButtons(data.transcription);
				break;
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

			// Wait before next attempt
			await new Promise((resolve) => setTimeout(resolve, 1000));
			attempts++;
		} catch (error) {
			statusDiv.classList.add("hidden");
			responseDiv.innerHTML = `
                <div class="bg-red-500 text-white p-4 rounded-md">
                    <p>${error.message}</p>
                </div>
            `;
			break;
		}
	}

	if (attempts >= maxAttempts) {
		statusDiv.classList.add("hidden");
		responseDiv.innerHTML = `
            <div class="bg-red-500 text-white p-4 rounded-md">
                <p>Transcription timed out. Please try again.</p>
            </div>
        `;
	}
}

function setupActionButtons(transcription) {
	const copyButton = document.getElementById("copyButton");
	const downloadButton = document.getElementById("downloadButton");

	copyButton.classList.remove("hidden");
	copyButton.onclick = () => {
		navigator.clipboard.writeText(transcription);
	};

	downloadButton.classList.remove("hidden");
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
