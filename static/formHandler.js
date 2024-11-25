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
			const formData = new FormData();
			formData.append("url", url);

			const response = await fetch("/api/v1/transcribe", {
				method: "POST",
				body: formData,
			});

			const data = await response.json();

			if (!response.ok) {
				throw new Error(data.error || "Failed to process video");
			}

			// Start checking status
			checkStatus(data.id);
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

async function checkStatus(id) {
	const statusDiv = document.getElementById("transcriptionStatus");
	const responseDiv = document.getElementById("response");

	try {
		while (true) {
			const response = await fetch(`/api/v1/transcribe/status/${id}`);
			const data = await response.json();

			if (!response.ok) {
				throw new Error(data.error || "Failed to check status");
			}

			if (data.status === "completed") {
				statusDiv.classList.add("hidden");

				// Show transcription
				document
					.getElementById("transcriptionHeader")
					.classList.remove("hidden");
				responseDiv.innerHTML = `
                    <div class="bg-gray-700 p-4 rounded-md">
                        <pre class="whitespace-pre-wrap">${data.transcription}</pre>
                    </div>
                `;

				// Enable action buttons
				const copyButton = document.getElementById("copyButton");
				copyButton.classList.remove("hidden");
				copyButton.onclick = () => {
					navigator.clipboard.writeText(data.transcription);
				};

				const downloadButton = document.getElementById("downloadButton");
				downloadButton.classList.remove("hidden");
				downloadButton.onclick = () => {
					const blob = new Blob([data.transcription], { type: "text/plain" });
					const url = URL.createObjectURL(blob);
					const a = document.createElement("a");
					a.href = url;
					a.download = "transcription.txt";
					a.click();
					URL.revokeObjectURL(url);
				};

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

			// Wait before checking again
			await new Promise((resolve) => setTimeout(resolve, 2000));
		}
	} catch (error) {
		statusDiv.classList.add("hidden");
		responseDiv.innerHTML = `
            <div class="bg-red-500 text-white p-4 rounded-md">
                <p>${error.message}</p>
            </div>
        `;
	}
}
