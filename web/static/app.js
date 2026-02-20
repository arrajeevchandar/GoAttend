// Attendance Engine Frontend Application
const API_BASE = '';  // Same origin

// State
let accessToken = localStorage.getItem('accessToken');
let deviceId = localStorage.getItem('deviceId');
let tokenExpiry = localStorage.getItem('tokenExpiry');

// Camera state
let cameraStream = null;
let capturedImageData = null;
let currentInputMode = 'camera';

// DOM Elements
const authSection = document.getElementById('authSection');
const dashboard = document.getElementById('dashboard');
const logoutBtn = document.getElementById('logoutBtn');
const connectionStatus = document.getElementById('connectionStatus');

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    checkHealth();
    setupEventListeners();
    updateUI();
    
    // Auto-refresh events every 10 seconds
    setInterval(() => {
        if (accessToken) refreshEvents();
    }, 10000);
});

function setupEventListeners() {
    // Register form
    document.getElementById('registerForm').addEventListener('submit', async (e) => {
        e.preventDefault();
        await registerDevice();
    });

    // Check-in form
    document.getElementById('checkinForm').addEventListener('submit', async (e) => {
        e.preventDefault();
        await submitCheckin();
    });

    // File upload
    const dropZone = document.getElementById('dropZone');
    const fileInput = document.getElementById('imageFileInput');
    
    if (dropZone) {
        dropZone.addEventListener('click', () => fileInput.click());
        dropZone.addEventListener('dragover', (e) => {
            e.preventDefault();
            dropZone.classList.add('border-purple-500', 'bg-purple-50');
        });
        dropZone.addEventListener('dragleave', () => {
            dropZone.classList.remove('border-purple-500', 'bg-purple-50');
        });
        dropZone.addEventListener('drop', (e) => {
            e.preventDefault();
            dropZone.classList.remove('border-purple-500', 'bg-purple-50');
            if (e.dataTransfer.files.length) {
                fileInput.files = e.dataTransfer.files;
                handleFileSelect(e.dataTransfer.files[0]);
            }
        });
    }
    
    if (fileInput) {
        fileInput.addEventListener('change', (e) => {
            if (e.target.files.length) {
                handleFileSelect(e.target.files[0]);
            }
        });
    }
}

// Input mode switching
function setInputMode(mode) {
    currentInputMode = mode;
    
    // Update buttons
    ['camera', 'upload', 'url'].forEach(m => {
        const btn = document.getElementById(`${m}ModeBtn`);
        const section = document.getElementById(`${m}Mode`);
        
        if (btn) {
            if (m === mode) {
                btn.classList.add('bg-white', 'shadow', 'text-purple-600');
                btn.classList.remove('text-gray-600');
            } else {
                btn.classList.remove('bg-white', 'shadow', 'text-purple-600');
                btn.classList.add('text-gray-600');
            }
        }
        
        if (section) {
            section.classList.toggle('hidden', m !== mode);
        }
    });
    
    // Stop camera if switching away
    if (mode !== 'camera' && cameraStream) {
        stopCamera();
    }
}

// Camera functions
async function startCamera() {
    const video = document.getElementById('cameraVideo');
    const startBtn = document.getElementById('startCameraBtn');
    const captureBtn = document.getElementById('captureBtn');
    const overlay = document.getElementById('faceOverlay');
    
    try {
        // Request camera access
        cameraStream = await navigator.mediaDevices.getUserMedia({
            video: {
                width: { ideal: 640 },
                height: { ideal: 480 },
                facingMode: 'user'  // Front camera
            },
            audio: false
        });
        
        video.srcObject = cameraStream;
        await video.play();
        
        startBtn.innerHTML = '<i class="fas fa-stop mr-1"></i> Stop Camera';
        startBtn.onclick = stopCamera;
        startBtn.classList.remove('bg-gray-600');
        startBtn.classList.add('bg-red-600', 'hover:bg-red-700');
        
        captureBtn.disabled = false;
        
        // Hide captured preview, show live feed
        document.getElementById('capturedPreview').classList.add('hidden');
        document.getElementById('cameraContainer').classList.remove('hidden');
        
        showToast('Camera started! Position your face in the oval.', 'success');
        
    } catch (err) {
        console.error('Camera error:', err);
        showToast('Could not access camera. Please check permissions.', 'error');
    }
}

function stopCamera() {
    if (cameraStream) {
        cameraStream.getTracks().forEach(track => track.stop());
        cameraStream = null;
    }
    
    const video = document.getElementById('cameraVideo');
    const startBtn = document.getElementById('startCameraBtn');
    const captureBtn = document.getElementById('captureBtn');
    
    if (video) video.srcObject = null;
    
    if (startBtn) {
        startBtn.innerHTML = '<i class="fas fa-video mr-1"></i> Start Camera';
        startBtn.onclick = startCamera;
        startBtn.classList.remove('bg-red-600', 'hover:bg-red-700');
        startBtn.classList.add('bg-gray-600', 'hover:bg-gray-700');
    }
    
    if (captureBtn) captureBtn.disabled = true;
}

function capturePhoto() {
    const video = document.getElementById('cameraVideo');
    const flash = document.getElementById('captureFlash');
    
    if (!video || !cameraStream) {
        showToast('Please start the camera first.', 'error');
        return;
    }
    
    // Create canvas to capture frame
    const canvas = document.createElement('canvas');
    canvas.width = video.videoWidth;
    canvas.height = video.videoHeight;
    const ctx = canvas.getContext('2d');
    
    // Flip horizontally to match mirror view
    ctx.translate(canvas.width, 0);
    ctx.scale(-1, 1);
    ctx.drawImage(video, 0, 0);
    
    // Get image data
    capturedImageData = canvas.toDataURL('image/jpeg', 0.9);
    
    // Flash effect
    flash.classList.remove('opacity-0');
    flash.classList.add('capture-flash');
    setTimeout(() => {
        flash.classList.add('opacity-0');
        flash.classList.remove('capture-flash');
    }, 300);
    
    // Show preview
    const capturedImg = document.getElementById('capturedImage');
    capturedImg.src = capturedImageData;
    
    document.getElementById('cameraContainer').classList.add('hidden');
    document.getElementById('capturedPreview').classList.remove('hidden');
    
    // Stop camera to save resources
    stopCamera();
    
    showToast('Photo captured! Review and submit.', 'success');
}

function retakePhoto() {
    capturedImageData = null;
    document.getElementById('capturedPreview').classList.add('hidden');
    document.getElementById('identifyResult').classList.add('hidden');
    document.getElementById('cameraContainer').classList.remove('hidden');
    startCamera();
}

async function identifyFromCamera() {
    if (!capturedImageData) {
        showToast('Please capture a photo first', 'error');
        return;
    }
    
    const btn = document.getElementById('identifyBtn');
    const resultDiv = document.getElementById('identifyResult');
    const successDiv = document.getElementById('identifySuccess');
    const failDiv = document.getElementById('identifyFail');
    
    try {
        btn.disabled = true;
        btn.innerHTML = '<i class="fas fa-spinner fa-spin mr-2"></i> Identifying...';
        
        const response = await fetch(`${FACE_SERVICE_URL}/identify`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                image_data: capturedImageData,
                top_k: 3
            })
        });
        
        const data = await response.json();
        resultDiv.classList.remove('hidden');
        
        if (response.ok) {
            if (data.identified && data.best_match) {
                // Show success
                successDiv.classList.remove('hidden');
                failDiv.classList.add('hidden');
                resultDiv.className = 'mt-3 p-4 rounded-lg bg-green-50 border border-green-200';
                
                document.getElementById('identifiedName').textContent = data.best_match.name || data.best_match.user_id;
                document.getElementById('identifiedId').textContent = `ID: ${data.best_match.user_id}`;
                
                const confidence = (data.best_match.similarity * 100).toFixed(1);
                document.getElementById('identifyConfidence').textContent = `${confidence}%`;
                document.getElementById('identifyConfBar').style.width = `${confidence}%`;
                
                // Liveness badge
                const livenessEl = document.getElementById('identifyLiveness');
                if (data.liveness && data.liveness.is_live) {
                    livenessEl.className = 'px-2 py-1 rounded-full bg-green-100 text-green-700';
                    livenessEl.innerHTML = '<i class="fas fa-shield-alt mr-1"></i>Live';
                } else {
                    livenessEl.className = 'px-2 py-1 rounded-full bg-red-100 text-red-700';
                    livenessEl.innerHTML = '<i class="fas fa-exclamation-triangle mr-1"></i>Check Liveness';
                }
                
                // Quality badge
                if (data.quality) {
                    const qualityPct = (data.quality.score * 100).toFixed(0);
                    document.getElementById('identifyQuality').innerHTML = `<i class="fas fa-star mr-1"></i>Quality: ${qualityPct}%`;
                }
                
                // Auto-fill the employee ID for check-in
                document.getElementById('userIdInput').value = data.best_match.user_id;
                
                showToast(`Identified: ${data.best_match.name || data.best_match.user_id}`, 'success');
            } else {
                // No match found
                successDiv.classList.add('hidden');
                failDiv.classList.remove('hidden');
                resultDiv.className = 'mt-3 p-4 rounded-lg bg-red-50 border border-red-200';
                
                showToast('No match found in gallery', 'error');
            }
        } else {
            successDiv.classList.add('hidden');
            failDiv.classList.remove('hidden');
            resultDiv.className = 'mt-3 p-4 rounded-lg bg-red-50 border border-red-200';
            
            const errorMsg = data.detail || 'Identification failed';
            document.querySelector('#identifyFail p.text-sm').textContent = errorMsg;
            showToast(errorMsg, 'error');
        }
        
    } catch (err) {
        resultDiv.classList.remove('hidden');
        successDiv.classList.add('hidden');
        failDiv.classList.remove('hidden');
        resultDiv.className = 'mt-3 p-4 rounded-lg bg-red-50 border border-red-200';
        
        document.querySelector('#identifyFail p.text-sm').textContent = `Error: ${err.message}`;
        showToast(`Error: ${err.message}`, 'error');
    } finally {
        btn.disabled = false;
        btn.innerHTML = '<i class="fas fa-user-check mr-2"></i> Identify Person';
    }
}

// File handling
function handleFileSelect(file) {
    showFileName(file.name);
    
    // Show preview
    const reader = new FileReader();
    reader.onload = (e) => {
        const preview = document.getElementById('uploadPreview');
        const img = document.getElementById('uploadedImage');
        if (preview && img) {
            img.src = e.target.result;
            preview.classList.remove('hidden');
        }
    };
    reader.readAsDataURL(file);
}

function showFileName(name) {
    const fileNameEl = document.getElementById('fileName');
    fileNameEl.textContent = name;
    fileNameEl.classList.remove('hidden');
}

async function checkHealth() {
    try {
        const res = await fetch(`${API_BASE}/healthz`);
        const data = await res.json();
        
        const statusEl = connectionStatus;
        if (data.status === 'ok' && data.db && data.redis) {
            statusEl.innerHTML = `
                <span class="w-3 h-3 bg-green-400 rounded-full mr-2 animate-pulse"></span>
                <span class="text-sm">Connected</span>
            `;
        } else {
            statusEl.innerHTML = `
                <span class="w-3 h-3 bg-yellow-400 rounded-full mr-2"></span>
                <span class="text-sm">Degraded</span>
            `;
        }
    } catch (err) {
        connectionStatus.innerHTML = `
            <span class="w-3 h-3 bg-red-400 rounded-full mr-2"></span>
            <span class="text-sm">Offline</span>
        `;
    }
}

async function registerDevice() {
    const deviceIdInput = document.getElementById('deviceIdInput');
    const errorEl = document.getElementById('registerError');
    const newDeviceId = deviceIdInput.value.trim();
    
    if (!newDeviceId) {
        showError(errorEl, 'Please enter a device ID');
        return;
    }
    
    try {
        const res = await fetch(`${API_BASE}/v1/devices/register`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ device_id: newDeviceId })
        });
        
        const data = await res.json();
        
        if (!res.ok) {
            showError(errorEl, data.error || 'Registration failed');
            return;
        }
        
        // Save credentials
        accessToken = data.access_token;
        deviceId = newDeviceId;
        tokenExpiry = data.expires_at;
        
        localStorage.setItem('accessToken', accessToken);
        localStorage.setItem('deviceId', deviceId);
        localStorage.setItem('tokenExpiry', tokenExpiry);
        localStorage.setItem('refreshToken', data.refresh_token);
        
        showToast('Device registered successfully!', 'success');
        updateUI();
        refreshEvents();
        
    } catch (err) {
        showError(errorEl, 'Network error. Please try again.');
    }
}

async function submitCheckin() {
    const userId = document.getElementById('userIdInput').value.trim();
    const imageUrl = document.getElementById('imageUrlInput').value.trim();
    const fileInput = document.getElementById('imageFileInput');
    const resultEl = document.getElementById('checkinResult');
    const submitBtn = document.getElementById('submitBtn');
    
    if (!userId) {
        showResult(resultEl, 'Please enter an employee ID', 'error');
        return;
    }
    
    let finalImageUrl = '';
    
    // Determine image source based on mode
    if (currentInputMode === 'camera' && capturedImageData) {
        finalImageUrl = capturedImageData;
    } else if (currentInputMode === 'upload' && fileInput.files.length > 0) {
        finalImageUrl = await fileToDataUrl(fileInput.files[0]);
    } else if (currentInputMode === 'url' && imageUrl) {
        finalImageUrl = imageUrl;
    }
    
    if (!finalImageUrl) {
        showResult(resultEl, 'Please capture a photo, upload an image, or enter an image URL', 'error');
        return;
    }
    
    // Show loading state
    submitBtn.disabled = true;
    submitBtn.innerHTML = '<i class="fas fa-spinner fa-spin mr-2"></i>Processing...';
    
    try {
        // Upload local image (base64 / file) to Cloudinary first; skip for plain URLs
        const isLocalImage = finalImageUrl.startsWith('data:');
        if (isLocalImage) {
            submitBtn.innerHTML = '<i class="fas fa-cloud-upload-alt fa-spin mr-2"></i>Uploading image...';
            const uploadRes = await fetch(`${API_BASE}/v1/upload`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${accessToken}`
                },
                body: JSON.stringify({ data: finalImageUrl })
            });
            const uploadData = await uploadRes.json();
            if (!uploadRes.ok) {
                // Fall back to base64 URL if Cloudinary is not configured
                if (uploadRes.status === 503) {
                    console.warn('Cloudinary not configured, falling back to base64');
                } else {
                    showResult(resultEl, uploadData.error || 'Image upload failed', 'error');
                    return;
                }
            } else {
                finalImageUrl = uploadData.url;
            }
            submitBtn.innerHTML = '<i class="fas fa-spinner fa-spin mr-2"></i>Submitting check-in...';
        }

        const res = await fetch(`${API_BASE}/v1/checkins`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${accessToken}`
            },
            body: JSON.stringify({
                user_id: userId,
                device_id: deviceId,
                image_url: finalImageUrl
            })
        });
        
        const data = await res.json();
        
        if (!res.ok) {
            if (res.status === 401) {
                logout();
                showToast('Session expired. Please login again.', 'error');
                return;
            }
            showResult(resultEl, data.error || 'Check-in failed', 'error');
            return;
        }
        
        showResult(resultEl, `✓ Check-in submitted! Event ID: ${data.event_id.slice(0, 8)}...`, 'success');
        showToast('Face check-in submitted for processing!', 'success');
        
        // Show face status panel
        const faceStatus = document.getElementById('faceStatus');
        if (faceStatus) {
            faceStatus.classList.remove('hidden');
            document.getElementById('faceStatusText').textContent = 'Processing...';
            document.getElementById('faceConfidenceBar').style.width = '0%';
        }
        
        // Clear form
        document.getElementById('userIdInput').value = '';
        document.getElementById('imageUrlInput').value = '';
        fileInput.value = '';
        document.getElementById('fileName').classList.add('hidden');
        capturedImageData = null;
        
        // Hide previews
        document.getElementById('capturedPreview')?.classList.add('hidden');
        document.getElementById('uploadPreview')?.classList.add('hidden');
        document.getElementById('cameraContainer')?.classList.remove('hidden');
        
        // Refresh events to see the new one
        setTimeout(refreshEvents, 1000);
        
        // Poll for result
        pollEventStatus(data.event_id);
        
    } catch (err) {
        showResult(resultEl, 'Network error. Please try again.', 'error');
    } finally {
        submitBtn.disabled = false;
        submitBtn.innerHTML = '<i class="fas fa-paper-plane mr-2"></i>Submit Check-in';
    }
}

// Poll event status to show face recognition result
async function pollEventStatus(eventId) {
    const faceStatus = document.getElementById('faceStatus');
    const statusText = document.getElementById('faceStatusText');
    const confidenceBar = document.getElementById('faceConfidenceBar');
    
    let attempts = 0;
    const maxAttempts = 20;
    
    const poll = async () => {
        if (attempts >= maxAttempts) {
            statusText.textContent = 'Timeout - check events table';
            return;
        }
        
        attempts++;
        
        try {
            const res = await fetch(`${API_BASE}/v1/events?limit=50`, {
                headers: { 'Authorization': `Bearer ${accessToken}` }
            });
            
            if (!res.ok) return;
            
            const data = await res.json();
            const event = data.events?.find(e => e.ID === eventId);
            
            if (event) {
                if (event.Status === 'processed') {
                    const score = event.MatchScore || 0;
                    const percent = Math.round(score * 100);
                    
                    statusText.innerHTML = `<span class="text-green-600">✓ Face Detected</span> - ${percent}% confidence`;
                    confidenceBar.style.width = `${percent}%`;
                    confidenceBar.className = `h-full transition-all duration-300 ${percent >= 80 ? 'bg-green-500' : percent >= 50 ? 'bg-yellow-500' : 'bg-red-500'}`;
                    
                    showToast(`Face verified with ${percent}% confidence!`, percent >= 50 ? 'success' : 'error');
                    refreshEvents();
                    return;
                } else if (event.Status === 'failed') {
                    statusText.innerHTML = '<span class="text-red-600">✗ Face Detection Failed</span>';
                    confidenceBar.style.width = '0%';
                    showToast('Face detection failed. Please try again.', 'error');
                    return;
                }
            }
            
            // Still pending, continue polling
            statusText.textContent = `Processing... (${attempts}/${maxAttempts})`;
            confidenceBar.style.width = `${(attempts / maxAttempts) * 30}%`;
            setTimeout(poll, 1500);
            
        } catch (err) {
            console.error('Poll error:', err);
        }
    };
    
    poll();
}

async function refreshEvents() {
    if (!accessToken) return;
    
    try {
        const res = await fetch(`${API_BASE}/v1/events?limit=20`, {
            headers: { 'Authorization': `Bearer ${accessToken}` }
        });
        
        if (res.status === 401) {
            logout();
            return;
        }
        
        const data = await res.json();
        renderEvents(data.events || []);
        updateStats(data.events || []);
        
    } catch (err) {
        console.error('Failed to fetch events:', err);
    }
}

function renderEvents(events) {
    const tbody = document.getElementById('eventsTableBody');
    
    if (!events.length) {
        tbody.innerHTML = `
            <tr>
                <td colspan="4" class="py-8 text-center text-gray-400">
                    <i class="fas fa-inbox text-4xl mb-2 block"></i>
                    <p>No events yet</p>
                </td>
            </tr>
        `;
        return;
    }
    
    tbody.innerHTML = events.map(event => {
        const time = new Date(event.When).toLocaleString();
        const statusClass = getStatusClass(event.Status);
        const score = event.MatchScore ? (event.MatchScore * 100).toFixed(0) + '%' : '--';
        
        return `
            <tr class="border-b border-gray-100 hover:bg-gray-50 slide-in">
                <td class="py-3 px-2">
                    <div class="flex items-center">
                        <div class="w-8 h-8 bg-purple-100 rounded-full flex items-center justify-center mr-2">
                            <i class="fas fa-user text-purple-600 text-xs"></i>
                        </div>
                        <span class="font-medium text-gray-800">${event.UserID}</span>
                    </div>
                </td>
                <td class="py-3 px-2 text-sm text-gray-600">${time}</td>
                <td class="py-3 px-2">
                    <span class="px-2 py-1 rounded-full text-xs font-medium ${statusClass}">
                        ${event.Status || 'pending'}
                    </span>
                </td>
                <td class="py-3 px-2">
                    <span class="text-sm font-medium ${event.MatchScore > 0.8 ? 'text-green-600' : 'text-yellow-600'}">
                        ${score}
                    </span>
                </td>
            </tr>
        `;
    }).join('');
}

function getStatusClass(status) {
    switch (status) {
        case 'processed':
            return 'bg-green-100 text-green-700';
        case 'failed':
            return 'bg-red-100 text-red-700';
        default:
            return 'bg-yellow-100 text-yellow-700';
    }
}

function updateStats(events) {
    const total = events.length;
    const processed = events.filter(e => e.Status === 'processed').length;
    const pending = events.filter(e => e.Status === 'pending' || !e.Status).length;
    
    const scores = events.filter(e => e.MatchScore > 0).map(e => e.MatchScore);
    const avgScore = scores.length > 0 
        ? (scores.reduce((a, b) => a + b, 0) / scores.length * 100).toFixed(0) + '%'
        : '--';
    
    document.getElementById('statTotal').textContent = total;
    document.getElementById('statProcessed').textContent = processed;
    document.getElementById('statPending').textContent = pending;
    document.getElementById('statScore').textContent = avgScore;
}

function updateUI() {
    if (accessToken && deviceId) {
        authSection.classList.add('hidden');
        dashboard.classList.remove('hidden');
        logoutBtn.classList.remove('hidden');
        
        document.getElementById('currentDeviceId').textContent = deviceId;
        
        if (tokenExpiry) {
            const expiryDate = new Date(tokenExpiry * 1000);
            document.getElementById('tokenExpiry').textContent = expiryDate.toLocaleTimeString();
        }
    } else {
        authSection.classList.remove('hidden');
        dashboard.classList.add('hidden');
        logoutBtn.classList.add('hidden');
    }
}

function logout() {
    localStorage.removeItem('accessToken');
    localStorage.removeItem('deviceId');
    localStorage.removeItem('tokenExpiry');
    localStorage.removeItem('refreshToken');
    
    accessToken = null;
    deviceId = null;
    tokenExpiry = null;
    
    updateUI();
    showToast('Logged out successfully', 'info');
}

function showError(el, message) {
    el.textContent = message;
    el.classList.remove('hidden');
    setTimeout(() => el.classList.add('hidden'), 5000);
}

function showResult(el, message, type) {
    el.textContent = message;
    el.classList.remove('hidden', 'bg-green-100', 'text-green-700', 'bg-red-100', 'text-red-700');
    
    if (type === 'success') {
        el.classList.add('bg-green-100', 'text-green-700');
    } else {
        el.classList.add('bg-red-100', 'text-red-700');
    }
    
    setTimeout(() => el.classList.add('hidden'), 5000);
}

function showToast(message, type = 'success') {
    const toast = document.getElementById('toast');
    const icon = document.getElementById('toastIcon');
    const msg = document.getElementById('toastMessage');
    
    msg.textContent = message;
    
    icon.className = 'fas';
    switch (type) {
        case 'success':
            icon.classList.add('fa-check-circle', 'text-green-400');
            break;
        case 'error':
            icon.classList.add('fa-exclamation-circle', 'text-red-400');
            break;
        default:
            icon.classList.add('fa-info-circle', 'text-blue-400');
    }
    
    toast.classList.remove('hidden');
    setTimeout(() => toast.classList.add('hidden'), 3000);
}

function fileToDataUrl(file) {
    return new Promise((resolve) => {
        const reader = new FileReader();
        reader.onload = () => resolve(reader.result);
        reader.readAsDataURL(file);
    });
}

// ============ Face Recognition Testing Functions ============

const FACE_SERVICE_URL = 'http://localhost:8000';

async function checkFaceService() {
    const statusText = document.getElementById('faceServiceStatusText');
    const outputDiv = document.getElementById('faceTestOutput');
    const resultsDiv = document.getElementById('faceTestResults');
    
    // Elements may not exist if dashboard is hidden
    if (!statusText) return;
    
    try {
        statusText.textContent = 'Checking...';
        statusText.className = 'text-sm font-medium text-yellow-500';
        
        const response = await fetch(`${FACE_SERVICE_URL}/health`);
        const data = await response.json();
        
        if (data.status === 'ok') {
            statusText.textContent = `✓ ${data.model_name}`;
            statusText.className = 'text-sm font-medium text-green-600';
        } else {
            statusText.textContent = '✗ Unhealthy';
            statusText.className = 'text-sm font-medium text-red-500';
        }
        
        // Show output
        if (resultsDiv) resultsDiv.classList.remove('hidden');
        if (outputDiv) outputDiv.textContent = JSON.stringify(data, null, 2);
        
        showToast('Face service connected!', 'success');
    } catch (err) {
        statusText.textContent = '✗ Offline';
        statusText.className = 'text-sm font-medium text-red-500';
        
        if (resultsDiv) resultsDiv.classList.remove('hidden');
        if (outputDiv) outputDiv.textContent = `Error: ${err.message}\n\nMake sure face service is running:\n  docker compose -f deploy/docker-compose.yml up -d`;
        
        showToast('Face service not reachable', 'error');
    }
}

async function testFaceEmbed() {
    const imageUrl = document.getElementById('testImageUrl').value.trim();
    const outputDiv = document.getElementById('faceTestOutput');
    const resultsDiv = document.getElementById('faceTestResults');
    const qualityDiv = document.getElementById('qualityMetrics');
    const livenessDiv = document.getElementById('livenessResults');
    
    if (!imageUrl) {
        showToast('Please enter an image URL', 'error');
        return;
    }
    
    try {
        const btn = document.getElementById('testEmbedBtn');
        btn.disabled = true;
        btn.innerHTML = '<i class="fas fa-spinner fa-spin mr-1"></i> Testing...';
        
        const response = await fetch(`${FACE_SERVICE_URL}/embed`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ image_url: imageUrl })
        });
        
        const data = await response.json();
        
        resultsDiv.classList.remove('hidden');
        livenessDiv.classList.add('hidden');
        
        if (response.ok) {
            // Truncate embedding for display
            const displayData = { ...data };
            if (displayData.embedding) {
                displayData.embedding = `[${data.embedding.slice(0, 5).map(n => n.toFixed(3)).join(', ')}... (512 dims)]`;
            }
            outputDiv.textContent = JSON.stringify(displayData, null, 2);
            
            // Show quality metrics
            if (data.quality) {
                showQualityMetrics(data.quality, data.score);
            }
            
            showToast(`Face detected! Score: ${(data.score * 100).toFixed(1)}%`, 'success');
        } else {
            outputDiv.textContent = JSON.stringify(data, null, 2);
            qualityDiv.classList.add('hidden');
            showToast(data.detail || 'Face detection failed', 'error');
        }
        
        btn.disabled = false;
        btn.innerHTML = '<i class="fas fa-fingerprint mr-1"></i> Detect Face';
    } catch (err) {
        resultsDiv.classList.remove('hidden');
        outputDiv.textContent = `Error: ${err.message}`;
        showToast('Request failed', 'error');
        
        document.getElementById('testEmbedBtn').disabled = false;
        document.getElementById('testEmbedBtn').innerHTML = '<i class="fas fa-fingerprint mr-1"></i> Detect Face';
    }
}

async function testLiveness() {
    const imageUrl = document.getElementById('testImageUrl').value.trim();
    const outputDiv = document.getElementById('faceTestOutput');
    const resultsDiv = document.getElementById('faceTestResults');
    const livenessDiv = document.getElementById('livenessResults');
    const qualityDiv = document.getElementById('qualityMetrics');
    
    if (!imageUrl) {
        showToast('Please enter an image URL', 'error');
        return;
    }
    
    try {
        const btn = document.getElementById('testLivenessBtn');
        btn.disabled = true;
        btn.innerHTML = '<i class="fas fa-spinner fa-spin mr-1"></i> Testing...';
        
        const response = await fetch(`${FACE_SERVICE_URL}/liveness`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ image_url: imageUrl })
        });
        
        const data = await response.json();
        
        resultsDiv.classList.remove('hidden');
        qualityDiv.classList.add('hidden');
        
        if (response.ok) {
            outputDiv.textContent = JSON.stringify(data, null, 2);
            showLivenessResults(data);
            showToast(data.is_live ? 'Live face detected!' : 'Possible spoof detected', data.is_live ? 'success' : 'error');
        } else {
            outputDiv.textContent = JSON.stringify(data, null, 2);
            livenessDiv.classList.add('hidden');
            showToast(data.detail || 'Liveness check failed', 'error');
        }
        
        btn.disabled = false;
        btn.innerHTML = '<i class="fas fa-user-shield mr-1"></i> Liveness';
    } catch (err) {
        resultsDiv.classList.remove('hidden');
        outputDiv.textContent = `Error: ${err.message}`;
        showToast('Request failed', 'error');
        
        document.getElementById('testLivenessBtn').disabled = false;
        document.getElementById('testLivenessBtn').innerHTML = '<i class="fas fa-user-shield mr-1"></i> Liveness';
    }
}

async function testAnalyze() {
    const imageUrl = document.getElementById('testImageUrl').value.trim();
    const outputDiv = document.getElementById('faceTestOutput');
    const resultsDiv = document.getElementById('faceTestResults');
    const qualityDiv = document.getElementById('qualityMetrics');
    const livenessDiv = document.getElementById('livenessResults');
    
    if (!imageUrl) {
        showToast('Please enter an image URL', 'error');
        return;
    }
    
    try {
        const btn = document.getElementById('testAnalyzeBtn');
        btn.disabled = true;
        btn.innerHTML = '<i class="fas fa-spinner fa-spin mr-1"></i> Analyzing...';
        
        const response = await fetch(`${FACE_SERVICE_URL}/analyze?image_url=${encodeURIComponent(imageUrl)}`, {
            method: 'POST'
        });
        
        const data = await response.json();
        
        resultsDiv.classList.remove('hidden');
        
        if (response.ok) {
            outputDiv.textContent = JSON.stringify(data, null, 2);
            
            if (data.faces && data.faces.length > 0) {
                const face = data.faces[0];
                if (face.quality) {
                    showQualityMetrics(face.quality, face.detection_score);
                }
                if (face.liveness) {
                    showLivenessResults(face.liveness);
                }
                
                let info = `Faces detected: ${data.faces_detected}`;
                if (face.age) info += `, Age: ~${face.age}`;
                if (face.gender) info += `, Gender: ${face.gender}`;
                showToast(info, 'success');
            } else {
                qualityDiv.classList.add('hidden');
                livenessDiv.classList.add('hidden');
                showToast('No faces detected', 'error');
            }
        } else {
            outputDiv.textContent = JSON.stringify(data, null, 2);
            qualityDiv.classList.add('hidden');
            livenessDiv.classList.add('hidden');
            showToast(data.detail || 'Analysis failed', 'error');
        }
        
        btn.disabled = false;
        btn.innerHTML = '<i class="fas fa-microscope mr-1"></i> Full Analysis';
    } catch (err) {
        resultsDiv.classList.remove('hidden');
        outputDiv.textContent = `Error: ${err.message}`;
        showToast('Request failed', 'error');
        
        document.getElementById('testAnalyzeBtn').disabled = false;
        document.getElementById('testAnalyzeBtn').innerHTML = '<i class="fas fa-microscope mr-1"></i> Full Analysis';
    }
}

function showQualityMetrics(quality, detectionScore) {
    const qualityDiv = document.getElementById('qualityMetrics');
    qualityDiv.classList.remove('hidden');
    
    // Overall quality
    const qScore = quality.score * 100;
    document.getElementById('qualityScore').textContent = `${qScore.toFixed(1)}%`;
    document.getElementById('qualityBar').style.width = `${qScore}%`;
    document.getElementById('qualityBar').className = `h-full rounded-full transition-all ${qScore > 70 ? 'bg-green-500' : qScore > 40 ? 'bg-yellow-500' : 'bg-red-500'}`;
    
    // Detection confidence
    const dScore = (detectionScore || 0) * 100;
    document.getElementById('detectionScore').textContent = `${dScore.toFixed(1)}%`;
    document.getElementById('detectionBar').style.width = `${dScore}%`;
    
    // Blur (inverse - lower blur = better)
    const sharpness = (1 - quality.blur) * 100;
    document.getElementById('blurScore').textContent = `${sharpness.toFixed(1)}%`;
    document.getElementById('blurBar').style.width = `${sharpness}%`;
    document.getElementById('blurBar').className = `h-full rounded-full transition-all ${sharpness > 70 ? 'bg-purple-500' : sharpness > 40 ? 'bg-yellow-500' : 'bg-red-500'}`;
    
    // Pose angles
    document.getElementById('poseYaw').textContent = `${quality.pose_yaw?.toFixed(1) || 0}°`;
    document.getElementById('posePitch').textContent = `${quality.pose_pitch?.toFixed(1) || 0}°`;
    document.getElementById('poseRoll').textContent = `${quality.pose_roll?.toFixed(1) || 0}°`;
    
    // Frontal badge
    const frontalBadge = document.getElementById('frontalBadge');
    if (quality.is_frontal) {
        frontalBadge.classList.remove('hidden');
    } else {
        frontalBadge.classList.add('hidden');
    }
}

function showLivenessResults(liveness) {
    const livenessDiv = document.getElementById('livenessResults');
    livenessDiv.classList.remove('hidden');
    
    const statusDiv = document.getElementById('livenessStatus');
    const icon = document.getElementById('livenessIcon');
    const text = document.getElementById('livenessText');
    const confidence = document.getElementById('livenessConfidence');
    const checksDiv = document.getElementById('livenessChecks');
    
    if (liveness.is_live) {
        statusDiv.className = 'p-3 rounded-lg text-center bg-green-100';
        icon.className = 'fas fa-user-check text-4xl mb-2 text-green-600';
        text.textContent = 'LIVE FACE';
        text.className = 'font-medium text-green-800';
    } else {
        statusDiv.className = 'p-3 rounded-lg text-center bg-red-100';
        icon.className = 'fas fa-user-slash text-4xl mb-2 text-red-600';
        text.textContent = 'POSSIBLE SPOOF';
        text.className = 'font-medium text-red-800';
    }
    
    confidence.textContent = `Confidence: ${(liveness.confidence * 100).toFixed(1)}%`;
    
    // Show individual checks
    if (liveness.checks) {
        checksDiv.innerHTML = Object.entries(liveness.checks)
            .map(([key, value]) => {
                const score = typeof value === 'number' ? value : 0.5;
                const color = score > 0.7 ? 'text-green-600' : score > 0.4 ? 'text-yellow-600' : 'text-red-600';
                const name = key.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase());
                return `<div class="flex justify-between"><span class="text-gray-500">${name}</span><span class="${color} font-medium">${(score * 100).toFixed(0)}%</span></div>`;
            })
            .join('');
    }
}

// ============ Face Gallery Functions ============

async function enrollFace() {
    const userId = document.getElementById('enrollUserId').value.trim();
    const name = document.getElementById('enrollName').value.trim();
    const imageUrl = document.getElementById('enrollImageUrl').value.trim();
    
    if (!userId) {
        showToast('Please enter a User ID', 'error');
        return;
    }
    if (!imageUrl) {
        showToast('Please enter an image URL', 'error');
        return;
    }
    
    const btn = document.getElementById('enrollBtn');
    try {
        btn.disabled = true;
        btn.innerHTML = '<i class="fas fa-spinner fa-spin mr-1"></i> Enrolling...';
        
        const response = await fetch(`${FACE_SERVICE_URL}/enroll`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                user_id: userId,
                image_url: imageUrl,
                name: name || null
            })
        });
        
        const data = await response.json();
        
        if (response.ok && data.success) {
            showToast(`Enrolled ${userId} successfully!`, 'success');
            document.getElementById('enrollUserId').value = '';
            document.getElementById('enrollName').value = '';
            document.getElementById('enrollImageUrl').value = '';
            refreshGallery();
        } else {
            showToast(data.message || data.detail || 'Enrollment failed', 'error');
        }
    } catch (err) {
        showToast(`Error: ${err.message}`, 'error');
    } finally {
        btn.disabled = false;
        btn.innerHTML = '<i class="fas fa-plus-circle mr-1"></i> Enroll Face';
    }
}

async function searchFace() {
    const imageUrl = document.getElementById('searchImageUrl').value.trim();
    const resultsDiv = document.getElementById('searchResults');
    const matchList = document.getElementById('searchMatchList');
    
    if (!imageUrl) {
        showToast('Please enter an image URL to search', 'error');
        return;
    }
    
    const btn = document.getElementById('searchBtn');
    try {
        btn.disabled = true;
        btn.innerHTML = '<i class="fas fa-spinner fa-spin mr-1"></i> Searching...';
        
        const response = await fetch(`${FACE_SERVICE_URL}/search`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                image_url: imageUrl,
                top_k: 5
            })
        });
        
        const data = await response.json();
        resultsDiv.classList.remove('hidden');
        
        if (response.ok) {
            if (data.matches && data.matches.length > 0) {
                matchList.innerHTML = data.matches.map((match, i) => {
                    const similarity = (match.similarity * 100).toFixed(1);
                    const barColor = match.similarity > 0.7 ? 'bg-green-500' : match.similarity > 0.5 ? 'bg-yellow-500' : 'bg-red-500';
                    const matchIcon = i === 0 && match.similarity > 0.6 ? '<i class="fas fa-check-circle text-green-500 ml-2"></i>' : '';
                    return `
                        <div class="p-2 bg-gray-50 rounded-lg">
                            <div class="flex justify-between items-center mb-1">
                                <span class="font-medium text-sm">${match.name || match.user_id}${matchIcon}</span>
                                <span class="text-xs text-gray-500">${similarity}%</span>
                            </div>
                            <div class="h-1.5 bg-gray-200 rounded-full">
                                <div class="h-full ${barColor} rounded-full" style="width: ${similarity}%"></div>
                            </div>
                            <p class="text-xs text-gray-400 mt-1">ID: ${match.user_id}</p>
                        </div>
                    `;
                }).join('');
                showToast(`Found ${data.matches.length} match(es)`, 'success');
            } else {
                matchList.innerHTML = '<p class="text-sm text-gray-500 text-center py-2">No matches found</p>';
                showToast('No matches found in gallery', 'info');
            }
        } else {
            matchList.innerHTML = `<p class="text-sm text-red-500">${data.detail || 'Search failed'}</p>`;
            showToast(data.detail || 'Search failed', 'error');
        }
    } catch (err) {
        resultsDiv.classList.remove('hidden');
        matchList.innerHTML = `<p class="text-sm text-red-500">Error: ${err.message}</p>`;
        showToast(`Error: ${err.message}`, 'error');
    } finally {
        btn.disabled = false;
        btn.innerHTML = '<i class="fas fa-search mr-1"></i> Find Matches';
    }
}

async function refreshGallery() {
    const listDiv = document.getElementById('galleryList');
    const countDiv = document.getElementById('galleryCount');
    
    // Get auth token
    const token = localStorage.getItem('accessToken');
    if (!token) {
        listDiv.innerHTML = '<p class="text-sm text-gray-400 text-center py-4">Login to view employees</p>';
        return;
    }
    
    try {
        const response = await fetch(`${API_BASE}/v1/employees`, {
            headers: { 'Authorization': `Bearer ${token}` }
        });
        const data = await response.json();
        
        if (response.ok) {
            const employees = data.employees || [];
            countDiv.textContent = `${employees.length} employee(s) registered`;
            
            if (employees.length === 0) {
                listDiv.innerHTML = '<p class="text-sm text-gray-400 text-center py-4">No employees registered yet</p>';
            } else {
                listDiv.innerHTML = employees.map(emp => `
                    <div class="flex items-center p-2 bg-gray-50 rounded hover:bg-gray-100 transition">
                        <div class="w-8 h-8 ${emp.face_enrolled ? 'bg-green-100' : 'bg-gray-100'} rounded-full flex items-center justify-center mr-3">
                            <i class="fas fa-user ${emp.face_enrolled ? 'text-green-600' : 'text-gray-400'} text-sm"></i>
                        </div>
                        <div class="flex-1">
                            <p class="text-sm font-medium text-gray-800">${emp.name || emp.employee_id}</p>
                            <p class="text-xs text-gray-400">ID: ${emp.employee_id}</p>
                        </div>
                        ${emp.face_enrolled ? '<span class="text-xs text-green-600"><i class="fas fa-check-circle"></i></span>' : '<span class="text-xs text-gray-400" title="Face not enrolled"><i class="fas fa-exclamation-circle"></i></span>'}
                    </div>
                `).join('');
            }
        } else {
            listDiv.innerHTML = '<p class="text-sm text-red-500 text-center py-2">Failed to load employees</p>';
        }
    } catch (err) {
        listDiv.innerHTML = '<p class="text-sm text-red-500 text-center py-2">API unavailable</p>';
    }
}

async function deleteFromGallery(userId) {
    if (!confirm(`Remove ${userId} from gallery?`)) return;
    
    try {
        const response = await fetch(`${FACE_SERVICE_URL}/enroll/${encodeURIComponent(userId)}`, {
            method: 'DELETE'
        });
        
        if (response.ok) {
            showToast(`Removed ${userId}`, 'success');
            refreshGallery();
        } else {
            const data = await response.json();
            showToast(data.detail || 'Delete failed', 'error');
        }
    } catch (err) {
        showToast(`Error: ${err.message}`, 'error');
    }
}

// Check face service on load
document.addEventListener('DOMContentLoaded', () => {
    setTimeout(() => {
        checkFaceService();
        refreshGallery();
    }, 1000);
});

// Expose functions globally
window.logout = logout;
window.refreshEvents = refreshEvents;
window.setInputMode = setInputMode;
window.startCamera = startCamera;
window.stopCamera = stopCamera;
window.capturePhoto = capturePhoto;
window.retakePhoto = retakePhoto;
window.identifyFromCamera = identifyFromCamera;
window.checkFaceService = checkFaceService;
window.testFaceEmbed = testFaceEmbed;
window.testLiveness = testLiveness;
window.testAnalyze = testAnalyze;
window.enrollFace = enrollFace;
window.searchFace = searchFace;
window.refreshGallery = refreshGallery;
window.deleteFromGallery = deleteFromGallery;

