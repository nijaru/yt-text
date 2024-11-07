import { validateURL } from "./utils.js";

let controller;

const RECAPTCHA_SITE_KEY = "6Lf-pHcqAAAAAAveMDTB_-TB0kT9MuIvQJ6qesoH";

function initRecaptcha() {
	const script = document.createElement("script");
	script.src = `https://www.google.com/recaptcha/api.js?onload=onRecaptchaLoad&render=${RECAPTCHA_SITE_KEY}`;
	script.async = true;
	script.defer = true;
	document.head.appendChild(script);
}

window.onRecaptchaLoad = () => {
	// Enable submit button immediately for local testing
	const submitButton = document.querySelector('button[type="submit"]');
	submitButton.disabled = false;

	grecaptcha.ready(() => {
		// The reCAPTCHA is ready
		submitButton.disabled = false;
	});
};

document.addEventListener("DOMContentLoaded", () => {
	initRecaptcha();
	// Enable submit button for local testing
	const submitButton = document.querySelector('button[type="submit"]');
	submitButton.disabled = false;
});

window.enableSubmit = () => {
	const submitButton = document.querySelector('button[type="submit"]');
	submitButton.disabled = false;
};

window.disableSubmit = () => {
	const submitButton = document.querySelector('button[type="submit"]');
	submitButton.disabled = true;
};

document
	.getElementById("transcriptionForm")
	.addEventListener("submit", async (event) => {
		event.preventDefault();

		const submitButton = document.querySelector('button[type="submit"]');
		const url = document.getElementById("url").value;

		// Validate URL first
		if (!validateURL(url)) {
			document.getElementById("response").innerText =
				"Invalid URL format. Please enter a valid URL.";
			return;
		}

		try {
			// Execute reCAPTCHA verification
			const token = await grecaptcha.execute(RECAPTCHA_SITE_KEY, {
				action: "submit",
			});
			if (!token) {
				throw new Error("reCAPTCHA verification failed");
			}

			const formData = new URLSearchParams({
				url: url,
				"g-recaptcha-response": token,
			});

			const spinner = document.getElementById("spinner");
			const transcriptionStatus = document.getElementById(
				"transcriptionStatus",
			);
			const transcriptionHeader = document.getElementById(
				"transcriptionHeader",
			);
			const responseDiv = document.getElementById("response");

			// Reset UI state
			responseDiv.innerText = "";
			document.getElementById("copyButton").classList.add("hidden");
			document.getElementById("downloadButton").classList.add("hidden");
			transcriptionHeader.classList.add("hidden");
			spinner.classList.remove("hidden");
			transcriptionStatus.classList.remove("hidden");
			submitButton.disabled = true;

			// Create a new AbortController instance
			controller = new AbortController();
			const signal = controller.signal;

			try {
				const response = await fetch("/transcribe", {
					method: "POST",
					headers: {
						"Content-Type": "application/x-www-form-urlencoded",
					},
					body: formData,
					signal: signal,
				});

				if (!response.ok) {
					const errorMessages = {
						400: "Invalid URL. Please enter a valid URL.",
						429: "Too many requests. Please try again later.",
						default:
							"An error occurred while processing your request. Please try again later.",
					};
					responseDiv.innerText =
						errorMessages[response.status] || errorMessages.default;
					return;
				}

				const data = await response.json();
				responseDiv.innerText = data.text;

				// Setup copy button
				const copyButton = document.getElementById("copyButton");
				copyButton.classList.remove("hidden");
				copyButton.onclick = async () => {
					try {
						await navigator.clipboard.writeText(data.text);
						alert("Text copied to clipboard");
					} catch (err) {
						responseDiv.innerText = "Failed to copy text. Please try again.";
					}
				};

				// Setup download button
				const downloadButton = document.getElementById("downloadButton");
				downloadButton.classList.remove("hidden");
				downloadButton.onclick = () => {
					const blob = new Blob([data.text], { type: "text/plain" });
					const link = document.createElement("a");
					link.href = URL.createObjectURL(blob);
					link.download = `${url.replace(/[^a-z0-9]/gi, "_").toLowerCase()}.txt`;
					link.click();
					URL.revokeObjectURL(link.href);
				};

				transcriptionHeader.classList.remove("hidden");
			} catch (error) {
				if (error.name !== "AbortError") {
					responseDiv.innerText =
						"An error occurred while processing your request. Please try again later.";
				}
			} finally {
				submitButton.disabled = false;
				spinner.classList.add("hidden");
				transcriptionStatus.classList.add("hidden");
			}
		} catch (error) {
			document.getElementById("response").innerText =
				"Please verify that you are human";
		}
	});

window.addEventListener("beforeunload", () => {
	if (controller) {
		controller.abort(); // Abort the fetch request if the user leaves the page
	}
});
