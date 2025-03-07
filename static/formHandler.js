import { validateURL } from "./utils.js";

// Store active WebSocket connections by job ID
const activeConnections = new Map();

// Map to track active jobs and their elements
const activeJobs = new Map();

// WebSocket reconnection settings
const WS_RECONNECT_MAX_ATTEMPTS = 5;
const WS_RECONNECT_BASE_DELAY = 1000; // 1 second
const WS_RECONNECT_MAX_DELAY = 30000; // 30 seconds

document
    .getElementById("transcriptionForm")
    .addEventListener("submit", async (event) => {
        event.preventDefault();

        const form = event.target;
        const urlInput = document.getElementById("url");
        const url = urlInput.value.trim();
        const responseDiv = document.getElementById("response");

        // Basic validation
        if (!validateURL(url)) {
            displayError(responseDiv, "Please enter a valid YouTube URL");
            return;
        }

        // Clear response div for new submissions
        resetResultDisplay();

        // Support multiple submissions - don't disable submit button
        
        // Determine which approach to use based on browser support
        if ("WebSocket" in window) {
            // Use WebSocket for real-time updates
            processWithWebSocket(url);
        } else {
            // Fall back to traditional approach
            await processThroughAPI(url);
        }
        
        // Clear the input field for the next submission
        urlInput.value = "";
    });

/**
 * Creates a WebSocket connection with reconnection capabilities
 * @param {string} url - The URL to transcribe
 * @param {HTMLElement} jobElement - The job element to update
 * @param {number} [attempt=0] - Current reconnection attempt
 * @param {boolean} [isReconnect=false] - Whether this is a reconnection
 * @param {string} [jobId=null] - The job ID if already known
 */
function createWebSocketConnection(url, jobElement, attempt = 0, isReconnect = false, jobId = null) {
    // Generate WebSocket URL
    const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${wsProtocol}//${window.location.host}/ws/transcribe`;
    
    // Create WebSocket instance
    const socket = new WebSocket(wsUrl);
    
    // Connection backoff calculation for reconnections
    const reconnectDelay = Math.min(
        WS_RECONNECT_BASE_DELAY * Math.pow(2, attempt),
        WS_RECONNECT_MAX_DELAY
    );
    
    // Clear any existing intervals
    if (socket.pingInterval) {
        clearInterval(socket.pingInterval);
    }
    
    // Track connection state
    socket.isReconnecting = isReconnect;
    socket.reconnectAttempt = attempt;
    socket.jobId = jobId;
    socket.jobElement = jobElement;
    socket.originalUrl = url;
    
    // Set up event handlers
    socket.onopen = function() {
        // Reset reconnection tracking on successful connection
        if (isReconnect) {
            const statusEl = jobElement.querySelector(".job-message");
            if (statusEl) {
                statusEl.textContent = "Reconnected. Resuming...";
            }
        }
        
        if (jobId) {
            // For reconnections, we want to query the status
            requestJobStatus(socket, jobId);
        } else {
            // For new connections, send the transcription request
            const message = {
                type: "transcribe",
                payload: { url: url }
            };
            socket.send(JSON.stringify(message));
        }
        
        // Set up ping interval for this connection
        socket.pingInterval = setInterval(() => {
            if (socket && socket.readyState === WebSocket.OPEN) {
                socket.send(JSON.stringify({ type: "ping" }));
            } else {
                clearInterval(socket.pingInterval);
            }
        }, 30000); // Send ping every 30 seconds
    };
    
    socket.onmessage = function(event) {
        const message = JSON.parse(event.data);
        
        switch (message.type) {
            case "progress":
                handleProgressUpdate(message.payload, socket, jobElement);
                break;
                
            case "error":
                // If we have a structured error with code
                const error = message.payload;
                const errorCode = error.code || "ERR_GENERAL";
                
                // If this is a not found error during reconnection, clean up
                if (isReconnect && errorCode === "ERR_JOB_NOT_FOUND") {
                    removeJobElement(jobElement);
                    closeConnection(socket);
                    return;
                }
                
                // For other errors in new connections, show error and remove job
                if (!isReconnect) {
                    removeJobElement(jobElement);
                    displayError(document.getElementById("response"), error.error || "Unknown error");
                    closeConnection(socket);
                } else {
                    // For reconnections, attempt to reconnect again if allowed
                    handleReconnection(socket, error.error);
                }
                break;
                
            case "pong":
                // Keep-alive response, no action needed
                break;
        }
    };
    
    socket.onerror = function(error) {
        // Handle connection errors
        handleReconnection(socket, error.message || "Connection error");
    };
    
    socket.onclose = function(event) {
        // Clean up the ping interval
        if (socket.pingInterval) {
            clearInterval(socket.pingInterval);
        }
        
        // Handle unexpected closures and attempt reconnection
        if (!event.wasClean && !socket.manualClose) {
            handleReconnection(socket, "Connection closed unexpectedly");
        }
    };
    
    return socket;
}

/**
 * Handles WebSocket reconnection logic with exponential backoff
 */
function handleReconnection(socket, errorMessage) {
    const { jobElement, originalUrl, reconnectAttempt, jobId } = socket;
    
    // Don't reconnect if the connection was manually closed or job complete
    if (socket.manualClose || !jobElement || !jobElement.parentNode) {
        return;
    }
    
    // Check if we've exceeded max attempts
    if (reconnectAttempt >= WS_RECONNECT_MAX_ATTEMPTS) {
        // Failed to reconnect after max attempts
        const statusEl = jobElement.querySelector(".job-message");
        if (statusEl) {
            statusEl.textContent = "Connection failed. Falling back to polling...";
        }
        
        // If we have a job ID, fall back to polling
        if (jobId) {
            pollTranscriptionStatus(jobId, jobElement);
        } else {
            removeJobElement(jobElement);
            displayError(document.getElementById("response"), 
                "Connection failed after multiple attempts: " + errorMessage);
        }
        return;
    }
    
    // Update the UI to show reconnect status
    const statusEl = jobElement.querySelector(".job-message");
    if (statusEl) {
        const nextAttempt = reconnectAttempt + 1;
        const delay = Math.round(Math.min(
            WS_RECONNECT_BASE_DELAY * Math.pow(2, reconnectAttempt) / 1000,
            WS_RECONNECT_MAX_DELAY / 1000
        ));
        statusEl.textContent = `Connection lost. Reconnecting in ${delay}s (${nextAttempt}/${WS_RECONNECT_MAX_ATTEMPTS})...`;
    }
    
    // Schedule reconnection with exponential backoff
    const reconnectDelay = Math.min(
        WS_RECONNECT_BASE_DELAY * Math.pow(2, reconnectAttempt),
        WS_RECONNECT_MAX_DELAY
    );
    
    setTimeout(() => {
        if (jobElement && jobElement.parentNode) {  // Check if element still exists
            createWebSocketConnection(
                originalUrl, 
                jobElement, 
                reconnectAttempt + 1, 
                true, 
                jobId
            );
        }
    }, reconnectDelay);
}

/**
 * Requests the current status of a job
 */
function requestJobStatus(socket, jobId) {
    if (socket && socket.readyState === WebSocket.OPEN) {
        const message = {
            type: "status",
            payload: { id: jobId }
        };
        socket.send(JSON.stringify(message));
    }
}

/**
 * Process transcription using WebSockets for real-time updates
 */
async function processWithWebSocket(url) {
    try {
        // Create job entry with pending status (we don't have ID yet)
        const jobElement = createJobElement(url);
        
        // Create WebSocket connection with reconnection support
        const socket = createWebSocketConnection(url, jobElement);
        
    } catch (error) {
        displayError(document.getElementById("response"), error.message);
        // Fall back to traditional approach if WebSocket fails
        await processThroughAPI(url);
    }
}

/**
 * Creates a new job element in the UI
 */
function createJobElement(url) {
    const activeJobsContainer = document.getElementById("activeJobs");
    const template = document.getElementById("jobStatusTemplate");
    
    // Clone the template
    const jobElement = template.content.cloneNode(true).querySelector(".job-status");
    
    // Configure the job element
    jobElement.querySelector(".job-source-url").textContent = truncateUrl(url);
    jobElement.dataset.url = url;
    
    // Add cancel button handler
    const cancelButton = jobElement.querySelector(".cancel-button");
    cancelButton.addEventListener("click", () => {
        // Job ID will be set when we receive the first progress update
        const jobId = jobElement.dataset.jobId;
        if (jobId) {
            cancelJob(jobId, jobElement);
        }
    });
    
    // Add to the container
    activeJobsContainer.appendChild(jobElement);
    
    return jobElement;
}

/**
 * Truncates a URL for display purposes
 */
function truncateUrl(url) {
    // Maximum characters to show
    const maxLength = 30;
    
    try {
        // Extract video ID and use it as short form
        const videoId = new URL(url).searchParams.get("v");
        if (videoId) {
            return `ID: ${videoId}`;
        }
    } catch (e) {
        // URL parsing failed, use simple truncation
    }
    
    // Simple truncation fallback
    if (url.length > maxLength) {
        return url.substring(0, maxLength) + "...";
    }
    
    return url;
}

/**
 * Removes a job element from the UI
 */
function removeJobElement(jobElement) {
    if (jobElement && jobElement.parentNode) {
        jobElement.parentNode.removeChild(jobElement);
    }
}

/**
 * Updates the progress status for a specific job
 */
function handleProgressUpdate(data, socket, jobElement) {
    if (!data || !jobElement) return;
    
    // Store job ID on the element for future reference
    if (data.id) {
        const jobId = data.id;
        jobElement.dataset.jobId = jobId;
        
        // Set the job ID on the socket for reconnection purposes
        if (socket) {
            socket.jobId = jobId;
            
            // Add this connection to the active connections map
            activeConnections.set(jobId, socket);
        }
        
        // Add to active jobs map
        activeJobs.set(jobId, jobElement);
    }
    
    // Update progress bar
    const progressBar = jobElement.querySelector(".progress-bar");
    if (progressBar && data.progress !== undefined) {
        const percent = Math.round(data.progress * 100);
        progressBar.style.width = `${percent}%`;
        progressBar.textContent = `${percent}%`;
    }
    
    // Update message
    let fullMessage = data.message || "";
    
    // Add ETA if available
    if (data.eta !== undefined && data.eta > 0) {
        // Format the ETA
        let etaDisplay;
        if (data.eta > 60) {
            const minutes = Math.floor(data.eta / 60);
            const seconds = data.eta % 60;
            etaDisplay = `${minutes}m ${seconds}s`;
        } else {
            etaDisplay = `${data.eta}s`;
        }
        
        // Add ETA to message
        fullMessage += ` (ETA: ${etaDisplay})`;
    }
    
    // Update the message element
    const messageElement = jobElement.querySelector(".job-message");
    if (messageElement && fullMessage) {
        messageElement.textContent = fullMessage;
    }
    
    // Update detailed stages information
    if (jobElement.querySelector(".detailed-progress")) {
        updateDetailedStages(jobElement, data);
    } else {
        // Use simpler stage indicators if detailed view not available
        updateJobStages(jobElement, data);
    }
    
    // If completed, show transcription
    if (data.status === "completed") {
        // Fetch the complete transcription data
        fetchTranscriptionData(data.id, jobElement);
        
        // Close the WebSocket connection for this job
        if (socket) {
            socket.manualClose = true; // Mark that we're deliberately closing
            closeConnection(socket);
        }
    } 
    // If failed, show error and remove job
    else if (data.status === "failed") {
        const errorMessage = data.message || "Transcription failed";
        displayError(document.getElementById("response"), errorMessage);
        
        // Remove the job element after a short delay
        setTimeout(() => removeJobElement(jobElement), 3000);
        
        // Close the WebSocket connection for this job
        if (socket) {
            socket.manualClose = true; // Mark that we're deliberately closing
            closeConnection(socket);
        }
    }
}

/**
 * Updates detailed stages information when available
 */
function updateDetailedStages(jobElement, data) {
    const { progress, stage, substage } = data;
    
    // Find all stage indicators
    const stageElements = jobElement.querySelectorAll(".stage-item");
    if (!stageElements.length) return;
    
    // Reset all stages
    stageElements.forEach(el => {
        el.classList.remove("active", "completed", "failed");
    });
    
    // Get the current stage element
    let currentStageEl = null;
    let currentSubstageEl = null;
    
    if (data.status === "failed") {
        // Mark all as failed
        stageElements.forEach(el => el.classList.add("failed"));
        return;
    }
    
    if (data.status === "completed") {
        // Mark all as completed
        stageElements.forEach(el => el.classList.add("completed"));
        return;
    }
    
    // Map stages and substages to elements
    const stageMap = {
        "download": {
            element: stageElements[0],
            substages: {
                "preparing": 0,
                "video": 1, 
                "audio": 2
            }
        },
        "process": {
            element: stageElements[1],
            substages: {
                "analyzing": 0,
                "transcribing": 1,
                "finalizing": 2
            }
        }
    };
    
    // Mark stages up to current as completed
    let foundCurrent = false;
    for (const [stageName, stageData] of Object.entries(stageMap)) {
        if (foundCurrent) break;
        
        if (stageName === stage) {
            // This is the current stage
            stageData.element.classList.add("active");
            foundCurrent = true;
            
            // Update substage if available
            if (substage && substage in stageData.substages) {
                const substageEl = stageData.element.querySelector(`.substage-${stageData.substages[substage]}`);
                if (substageEl) {
                    substageEl.classList.add("active");
                }
            }
        } else {
            // This stage is already completed
            stageData.element.classList.add("completed");
        }
    }
}

/**
 * Updates the visual job stages based on progress
 */
function updateJobStages(jobElement, data) {
    const progress = data.progress || 0;
    const stages = jobElement.querySelectorAll(".stage");
    
    // Reset all stages
    stages.forEach(stage => {
        stage.classList.remove("bg-blue-600", "text-white");
        stage.classList.add("bg-gray-600");
    });
    
    // Determine active stage based on progress and status
    let activeStage = null;
    
    if (data.status === "completed") {
        // All stages complete
        stages.forEach(stage => {
            stage.classList.remove("bg-gray-600");
            stage.classList.add("bg-green-600", "text-white");
        });
    } else if (data.status === "failed") {
        // Mark stages as failed
        stages.forEach(stage => {
            stage.classList.remove("bg-gray-600");
            stage.classList.add("bg-red-600", "text-white");
        });
    } else {
        // In progress
        if (progress < 0.33) {
            // Downloading stage
            activeStage = stages[0];
        } else if (progress < 0.95) {
            // Processing stage
            activeStage = stages[1];
            stages[0].classList.remove("bg-gray-600");
            stages[0].classList.add("bg-blue-600", "text-white");
        } else {
            // Completing stage
            activeStage = stages[2];
            stages[0].classList.remove("bg-gray-600");
            stages[0].classList.add("bg-blue-600", "text-white");
            stages[1].classList.remove("bg-gray-600");
            stages[1].classList.add("bg-blue-600", "text-white");
        }
        
        if (activeStage) {
            activeStage.classList.remove("bg-gray-600");
            activeStage.classList.add("bg-blue-600", "text-white");
        }
    }
}

/**
 * Cancels an in-progress job
 */
function cancelJob(jobId, jobElement) {
    // Get the WebSocket for this job
    const socket = activeConnections.get(jobId);
    
    if (socket && socket.readyState === WebSocket.OPEN) {
        // Send cancellation message
        const message = {
            type: "cancel",
            payload: { id: jobId }
        };
        socket.send(JSON.stringify(message));
        
        // Update the UI to show cancelling state
        const messageElement = jobElement.querySelector(".job-message");
        if (messageElement) {
            messageElement.textContent = "Cancelling...";
        }
        
        // Disable cancel button
        const cancelButton = jobElement.querySelector(".cancel-button");
        if (cancelButton) {
            cancelButton.disabled = true;
            cancelButton.classList.add("opacity-50");
        }
        
    } else {
        // No active WebSocket for this job, try the REST API
        fetch(`/api/transcribe/${jobId}`, {
            method: "DELETE"
        })
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                // Remove the job element
                removeJobElement(jobElement);
            } else {
                displayError(document.getElementById("response"), data.message || "Failed to cancel job");
            }
        })
        .catch(error => {
            displayError(document.getElementById("response"), "Error: " + error.message);
        });
    }
}

/**
 * Closes a WebSocket connection and cleans up resources
 */
function closeConnection(socket) {
    if (!socket) return;
    
    // Mark the socket as being manually closed to prevent auto-reconnect
    socket.manualClose = true;
    
    // Clear ping interval if exists
    if (socket.pingInterval) {
        clearInterval(socket.pingInterval);
        socket.pingInterval = null;
    }
    
    // Close the connection if open or connecting
    if (socket.readyState === WebSocket.OPEN || 
        socket.readyState === WebSocket.CONNECTING) {
        socket.close();
    }
    
    // Clean up from active connections map
    if (socket.jobId) {
        activeConnections.delete(socket.jobId);
    } else {
        // Fallback for sockets without jobId
        for (const [id, conn] of activeConnections.entries()) {
            if (conn === socket) {
                activeConnections.delete(id);
                break;
            }
        }
    }
}

/**
 * Fetch complete transcription data when finished
 */
async function fetchTranscriptionData(id, jobElement) {
    try {
        const response = await fetch(`/api/transcribe/${id}`);
        const responseData = await response.json();
        
        if (!response.ok) {
            throw new Error(responseData.error || "Failed to fetch transcription");
        }
        
        showTranscription(responseData.data);
        
        // Remove the job element after a short delay to show completion
        setTimeout(() => removeJobElement(jobElement), 2000);
        
        // Remove from active jobs map
        activeJobs.delete(id);
        
        // Close WebSocket connection if still open
        const socket = activeConnections.get(id);
        closeConnection(socket);
        activeConnections.delete(id);
        
    } catch (error) {
        displayError(document.getElementById("response"), error.message);
    }
}

/**
 * Traditional API approach (fallback)
 */
async function processThroughAPI(url) {
    try {
        // Create job element for the pending job
        const jobElement = createJobElement(url);
        
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
            removeJobElement(jobElement);
            throw new Error(responseData.error || "Failed to process video");
        }

        const videoData = responseData.data;
        
        // Store job ID on element
        if (videoData.id) {
            jobElement.dataset.jobId = videoData.id;
        }

        if (videoData.status === "completed") {
            showTranscription(videoData);
            removeJobElement(jobElement);
        } else {
            await pollTranscriptionStatus(videoData.id, jobElement);
        }
    } catch (error) {
        displayError(document.getElementById("response"), error.message);
    }
}

/**
 * Resets the result display area
 */
function resetResultDisplay() {
    const responseDiv = document.getElementById("response");
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
    if (element) {
        if (hide) {
            element.classList.add("hidden");
        } else {
            element.classList.remove("hidden");
        }
    }
}

/**
 * Displays the transcription and sets up action buttons.
 * @param {Object} data - The transcription data.
 */
function showTranscription(data) {
    const responseDiv = document.getElementById("response");
    toggleVisibility("transcriptionHeader", false);

    // Determine source label text and color
    let sourceLabel = "AI-Generated";
    let sourceColor = "bg-purple-600";
    
    if (data.source === "youtube_api") {
        sourceLabel = "Official Captions";
        sourceColor = "bg-green-600";
    }

    responseDiv.innerHTML = `
        <div class="bg-gray-700 p-4 rounded-md">
            <div class="flex justify-between items-center mb-3">
                <div class="text-lg font-semibold">${escapeHTML(data.title || "")}</div>
                <div class="${sourceColor} text-white text-xs px-2 py-1 rounded">${sourceLabel}</div>
            </div>
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
 * @param {HTMLElement} jobElement - The job element to update.
 */
async function pollTranscriptionStatus(id, jobElement) {
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
            
            // Calculate progress (this is an estimate)
            let progress = 0;
            
            if (data.status === "processing") {
                // Estimate progress based on time elapsed (very rough)
                const createdAt = new Date(data.created_at).getTime();
                const elapsed = Date.now() - createdAt;
                progress = Math.min(elapsed / (10 * 60 * 1000), 0.95); // Cap at 95% until complete
            } else if (data.status === "completed" || data.status === "failed") {
                progress = 1.0;
            }
            
            // Create synthetic progress data for the job element update
            const progressData = {
                id: id,
                status: data.status,
                progress: progress,
                message: data.status === "failed" ? data.error : `Status: ${data.status}`
            };
            
            // Update the job element
            handleProgressUpdate(progressData, null, jobElement);

            if (data.status === "completed") {
                showTranscription(data);
                removeJobElement(jobElement);
                return;
            }

            if (data.status === "failed") {
                displayError(document.getElementById("response"), data.error || "Transcription failed");
                removeJobElement(jobElement);
                return;
            }

            // Wait before next attempt with increasing backoff
            await delay(pollingInterval);
        } catch (error) {
            displayError(document.getElementById("response"), error.message);
            removeJobElement(jobElement);
            return;
        }
    }

    // If max attempts reached without completion
    displayError(document.getElementById("response"), "Transcription timed out. Please try again.");
    removeJobElement(jobElement);
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