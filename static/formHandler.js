import { validateURL } from "./utils.js";

let controller;

function showError(message) {
	const responseDiv = document.getElementById("response");
	responseDiv.innerHTML = `
        <div class="bg-red-500 text-white p-4 rounded-md">
            <p class="font-bold">Error</p>
            <p>${message}</p>
        </div>
    `;
}

function showSuccess(text) {
	const responseDiv = document.getElementById("response");
	responseDiv.innerHTML = `
        <div class="bg-gray-700 p-4 rounded-md">
            <pre class="whitespace-pre-wrap">${text}</pre>
        </div>
    `;
}

function showToast(message, type = "success") {
	const toast = document.createElement("div");
	toast.className = `fixed bottom-4 right-4 ${
		type === "success" ? "bg-green-500" : "bg-red-500"
	} text-white px-6 py-3 rounded-lg shadow-lg transform transition-all duration-500 translate-y-0`;
	toast.textContent = message;
	document.body.appendChild(toast);

	setTimeout(() => {
		toast.classList.add("translate-y-full", "opacity-0");
		setTimeout(() => toast.remove(), 500);
	}, 3000);
}

function validateAndShowFeedback(url) {
	const input = document.getElementById("url");
	if (!validateURL(url)) {
		input.classList.add("border-red-500");
		input.classList.add("focus:ring-red-500");
		showError("Please enter a valid URL");
		return false;
	}
	input.classList.remove("border-red-500");
	input.classList.remove("focus:ring-red-500");
	return true;
}

document
	.getElementById("transcriptionForm")
	.addEventListener("submit", async (event) => {
		event.preventDefault();
		const submitButton = document.querySelector('button[type="submit"]');
		const spinner = document.getElementById("spinner");
		const transcriptionStatus = document.getElementById("transcriptionStatus");
		const transcriptionHeader = document.getElementById("transcriptionHeader");
		const url = document.getElementById("url").value;

		// Reset UI
		document.getElementById("response").innerText = "";
		document.getElementById("copyButton").classList.add("hidden");
		document.getElementById("downloadButton").classList.add("hidden");
		transcriptionHeader.classList.add("hidden");

		// Validate URL
		if (!validateAndShowFeedback(url)) {
			return;
		}

		// Show loading state
		spinner.classList.remove("hidden");
		transcriptionStatus.classList.remove("hidden");
		submitButton.disabled = true;

		// Create new AbortController
		controller = new AbortController();
		const signal = controller.signal;

		try {
			const response = await fetch("/transcribe", {
				method: "POST",
				headers: {
					"Content-Type": "application/x-www-form-urlencoded",
				},
				body: new URLSearchParams({ url }),
				signal,
			});

			if (!response.ok) {
				const errorMessages = {
					400: "Invalid YouTube URL. Please check and try again.",
					429: "Too many requests. Please wait a moment and try again.",
					500: "Server error. Please try again later.",
				};
				showError(
					errorMessages[response.status] || "An unexpected error occurred.",
				);
				return;
			}

			const data = await response.json();
			showSuccess(data.text);

			// Show copy button
			const copyButton = document.getElementById("copyButton");
			copyButton.classList.remove("hidden");
			copyButton.onclick = () => {
				navigator.clipboard
					.writeText(data.text)
					.then(() => {
						showToast("Text copied to clipboard");
					})
					.catch(() => {
						showToast("Failed to copy text", "error");
					});
			};

			// Show download button
			const downloadButton = document.getElementById("downloadButton");
			downloadButton.classList.remove("hidden");
			downloadButton.onclick = () => {
				const blob = new Blob([data.text], { type: "text/plain" });
				const link = document.createElement("a");
				link.href = URL.createObjectURL(blob);
				link.download = `${url.replace(/[^a-z0-9]/gi, "_").toLowerCase()}.txt`;
				link.click();
				showToast("Download started");
			};

			transcriptionHeader.classList.remove("hidden");
		} catch (error) {
			if (error.name === "AbortError") {
				showError("Request was cancelled.");
			} else {
				showError("Network error. Please check your connection and try again.");
			}
		} finally {
			submitButton.disabled = false;
			spinner.classList.add("hidden");
			transcriptionStatus.classList.add("hidden");
		}
	});

window.addEventListener("beforeunload", () => {
	if (controller) {
		controller.abort();
	}
});
